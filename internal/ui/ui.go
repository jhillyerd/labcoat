package ui

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhillyerd/labcoat/internal/config"
	"github.com/jhillyerd/labcoat/internal/nix"
	"github.com/jhillyerd/labcoat/internal/npool"
	"github.com/jhillyerd/labcoat/internal/runner"
	"github.com/jhillyerd/labcoat/internal/store"
)

const (
	viewModeHosts = iota
	viewModeText
	viewModeError
	viewModeConfirm
)

const (
	hostTabStatus = iota
	hostTabDeploy
	hostTabRunCmd
	hostTabLog
)

var hostTabNames = []string{"Host Status", "Deploy", "Run Command", "Op Log"}

type Model struct {
	ctx            context.Context
	program        *tea.Program
	db             *store.BoltDB
	config         config.Config
	ready          bool // true once screen size is known.
	viewMode       int  // Current UI mode.
	flakePath      string
	hostList       hostListModel
	hosts          map[string]*hostModel
	selectedHost   *hostModel
	hoverTimerID   uint64 // Unique ID for each host hover timer.
	nixPool        *npool.Pool
	contentPanel   *viewport.Model
	sizes          layoutSizes
	keys           config.KeyMap
	help           help.Model
	spinner        spinner.Model
	jumpToLetter   bool
	confirmation   *confirmationMsg
	commandPalette *CommandPalette
	inputOverlay   *InputOverlay
	text           string
	error          string
	flashText      string
	flashTimer     *time.Timer
	cmdHistory     []string
}

type sshCheckState int

const (
	sshCheckNone    sshCheckState = iota // Not yet attempted.
	sshCheckPending                      // In-flight.
	sshCheckPassed                       // Reachable.
	sshCheckFailed                       // Unreachable; user must retry.
)

type hostModel struct {
	name    string
	target  *nix.TargetInfo // Cached info about target host.
	hostTab int             // Currently visible host tab.
	sshState sshCheckState // Current state of SSH pre-flight connectivity check.
	deploy  struct {
		intro        string // Rendered intro text: command, host, etc.
		contentPanel viewport.Model
		runner       *runner.Model
		cancel       func()
	}
	log struct {
		contentPanel viewport.Model
	}
	runCmd struct {
		intro        string // Rendered intro text: command, host, etc.
		contentPanel viewport.Model
		runner       *runner.Model
	}
	status struct {
		collected    bool   // Whether status has been collected for this host.
		intro        string // Rendered intro text: command, host, etc.
		contentPanel viewport.Model
		runner       *runner.Model
	}
}

type layoutSizes struct {
	screen        dim
	hostList      dim
	contentHeader dim
	contentPanel  dim
	hintBar       dim
}

type dim struct {
	width  int
	height int
}

func New(
	conf config.Config, keys config.KeyMap, flakePath string, hostNames []string, db *store.BoltDB,
) Model {
	hostList := newHostList(hostNames)
	hostList.list.KeyMap.CursorUp = keys.Up
	hostList.list.KeyMap.CursorDown = keys.Down
	hostList.list.KeyMap.Filter = keys.Filter
	hostList.list.KeyMap.NextPage = keys.Right
	hostList.list.KeyMap.PrevPage = keys.Left

	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#80c080"))

	hosts := make(map[string]*hostModel, len(hostNames))
	for _, v := range hostNames {
		hm := &hostModel{name: v}
		hm.log.contentPanel = newContentPanel(keys)
		hm.runCmd.contentPanel = newContentPanel(keys)
		hm.status.contentPanel = newContentPanel(keys)
		hosts[v] = hm
	}

	m := Model{
		ctx:       context.Background(),
		config:    conf,
		db:        db,
		viewMode:  viewModeHosts,
		flakePath: flakePath,
		hostList:  hostList,
		hosts:     hosts,
		nixPool:   npool.New("nix", 2),
		keys:      keys,
		help:      help.New(),
		spinner:   spin,
	}

	m.commandPalette = NewCommandPalette(m.commands())
	return m
}

func newContentPanel(keys config.KeyMap) viewport.Model {
	cp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(25))

	cp.KeyMap.PageUp = keys.ScrollUp
	cp.KeyMap.PageDown = keys.ScrollDown

	blank := key.NewBinding(key.WithDisabled())
	cp.KeyMap.Up = blank
	cp.KeyMap.Down = blank
	cp.KeyMap.HalfPageUp = blank
	cp.KeyMap.HalfPageDown = blank

	return cp
}

type hostTargetInfoMsg struct {
	hostName string
	target   nix.TargetInfo
}

type hostChangedMsg struct {
	hostName string
}

type hostHoverMsg struct {
	hostName string
	timerID  uint64
}

type openPagerMsg struct{}

type confirmationMsg struct {
	text   string
	yesCmd tea.Cmd
	noCmd  tea.Cmd
}

type textInputPromptMsg struct {
	prompt   string
	submitFn func(string) tea.Cmd
}

type hostSSHCheckMsg struct {
	hostName string
	err      error // nil on success, *runner.SSHCheckError on failure.
}

type criticalErrorMsg struct {
	detail string
}

type errorFlashMsg struct {
	text string
}

type flakeMetadataMsg struct {
	meta nix.FlakeMetadata
}

type flakeMetadataTickMsg struct{}

type textDisplayMsg struct {
	text string
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.hostList.Init(),
		m.spinner.Tick,
		m.fetchFlakeMetadataCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// slog.Debug("tea.KeyMsg", "key", msg)

		if msg.String() == "ctrl+\\" {
			// Ctrl-\ overrides all view states to exit.
			return m, tea.Quit
		}

		if m.viewMode == viewModeText {
			// Any key to continue.
			m.viewMode = viewModeHosts
			m.text = ""

			return m, nil
		}

		if m.viewMode == viewModeError {
			// Error display is modal, swallow all key press messages.
			if msg.String() == "esc" {
				// Exit error display.
				m.viewMode = viewModeHosts
			}

			return m, nil
		}

		if m.hostList.FilterState() == list.Filtering {
			// User is entering filter text, disable keymaps.
			break
		}

		if m.jumpToLetter {
			m.jumpToLetter = false
			letter := msg.String()

			if len(letter) != 1 {
				slog.Debug("Invalid jump letter keypress", "key", msg)
				return m, errorFlashCmd("Invalid jump letter key pressed")
			}

			m.hostList, cmd = m.hostList.Update(jumpToLetterMsg(letter))
			return m, cmd
		}

		if m.confirmation != nil {
			// Awaiting confirmation, `y` or `n` will trigger the corresponding cmd.
			if msg.String() == "y" {
				cmd = m.confirmation.yesCmd
				m.confirmation = nil
				m.viewMode = viewModeHosts
				return m, cmd
			}

			if msg.String() == "n" {
				cmd = m.confirmation.noCmd
				m.confirmation = nil
				m.viewMode = viewModeHosts
				return m, cmd
			}

			slog.Debug("Invalid confirmation keypress", "key", msg)
			return m, nil
		}

		if m.inputOverlay != nil && m.inputOverlay.IsVisible() {
			visible := m.inputOverlay.IsVisible()
			cmd = m.inputOverlay.Update(msg)
			if !m.inputOverlay.IsVisible() && visible {
				if msg.String() == "enter" {
					value := m.inputOverlay.Value()
					submitFn := m.inputOverlay.submitFn
					m.inputOverlay = nil
					if value != "" {
						m.addToCmdHistory(value)
					}
					return m, submitFn(value)
				}
				m.inputOverlay = nil
			}
			return m, cmd
		}

		if m.commandPalette != nil && m.commandPalette.IsVisible() {
			selected, cmd := m.commandPalette.Update(msg)
			if selected != nil {
				return m, selected.Execute(&m)
			}
			return m, cmd
		}

		if msg.String() == "ctrl+c" {
			m.withVisibleRunner(func(r *runner.Model) {
				r.Cancel()
			})

			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Jump):
			m.jumpToLetter = true
			return m, func() tea.Msg {
				// Clear any flash message to ensure prompt visible.
				return errorFlashMsg{}
			}

		case key.Matches(msg, m.keys.NextTab):
			return m, m.handleNextTabKey()

		case key.Matches(msg, m.keys.CommandPalette):
			return m, m.commandPalette.Show()

		case key.Matches(msg, m.keys.Deploy):
			return m, m.hostDeployCmd(m.selectedHost)

		case key.Matches(msg, m.keys.Pager):
			return m, func() tea.Msg { return openPagerMsg{} }

		case key.Matches(msg, m.keys.Reboot):
			if ok, cmd := requireHostTarget("Reboot", m.selectedHost); !ok {
				return m, cmd
			}
			return m, func() tea.Msg {
				return confirmationMsg{
					text:   fmt.Sprintf("Confirm reboot of %q?", m.selectedHost.target.DeployHost),
					yesCmd: m.hostRunCommandCmd(m.selectedHost, "/run/current-system/sw/bin/reboot"),
				}
			}

		case key.Matches(msg, m.keys.RunCommandPrompt):
			if ok, cmd := requireHostTarget("RunCommand", m.selectedHost); !ok {
				return m, cmd
			}
			return m, func() tea.Msg {
				return textInputPromptMsg{
					prompt: fmt.Sprintf("Run on %q: ", m.selectedHost.target.DeployHost),
					submitFn: func(cmd string) tea.Cmd {
						return m.hostRunCommandCmd(m.selectedHost, cmd)
					},
				}
			}

		case key.Matches(msg, m.keys.Status):
			// Reset SSH check to allow re-checking on explicit status request.
			if m.selectedHost != nil && m.selectedHost.sshState == sshCheckFailed {
				m.selectedHost.sshState = sshCheckNone
			}
			return m, m.hostStatusCmd(m.selectedHost)

		case key.Matches(msg, m.keys.SSHInto):
			return m, m.startHostInteractiveSSH()

		case key.Matches(msg, m.keys.Help):
			m.viewMode = viewModeText
			m.text = labelStyle.Render("Help") +
				"\n\n" +
				m.help.FullHelpView(m.keys.FullHelp())
			return m, nil

		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}

	case hostChangedMsg:
		return m, m.handleHostChangedMsg(msg)

	case hostDeployMsg:
		return m, m.handleHostDeployMsg(msg)

	case hostDeployOutputMsg:
		return m, m.handleHostDeployOutputMsg(msg)

	case hostHoverMsg:
		return m, m.handleHostHoverMsg(msg)

	case hostLogUpdatedMsg:
		return m, m.handleHostLogUpdatedMsg(msg)

	case hostRunCommandMsg:
		return m, m.handleHostRunCommandMsg(msg)

	case hostRunCommandOutputMsg:
		return m, m.handleHostRunCommandOutputMsg(msg)

	case hostSSHCheckMsg:
		return m, m.handleHostSSHCheckMsg(msg)

	case hostStatusMsg:
		return m, m.handleHostStatusMsg(msg)

	case hostTargetInfoMsg:
		return m, m.handleHostTargetInfoMsg(msg)

	case openPagerMsg:
		return m, m.handleOpenPagerMsg(msg)

	case tea.WindowSizeMsg:
		m.sizes = calculateSizes(msg)
		m.hostList.SetSize(m.sizes.hostList.width, m.sizes.hostList.height)
		m.updateContentPanel()

		return m, nil

	case confirmationMsg:
		m.confirmation = &msg
		m.viewMode = viewModeConfirm
		return m, nil

	case textInputPromptMsg:
		m.inputOverlay = NewInputOverlay(msg.prompt, msg.submitFn, &m.cmdHistory)

	case criticalErrorMsg:
		m.viewMode = viewModeError
		m.error = msg.detail

	case errorFlashMsg:
		return m, m.handleErrorFlashMsg(msg)

	case flakeMetadataMsg:
		return m, m.handleFlakeMetadataMsg(msg)

	case flakeMetadataTickMsg:
		return m, m.fetchFlakeMetadataCmd()

	case textDisplayMsg:
		m.viewMode = viewModeText
		m.text = msg.text
		return m, nil

	case *tea.Program:
		m.program = msg

	case tea.Cmd:
		slog.Error("Got tea.Cmd instead of tea.Msg")
		return m, nil
	}

	m.hostList, cmd = m.hostList.Update(msg)
	cmds = append(cmds, cmd)

	if m.contentPanel != nil {
		cp, cmd := m.contentPanel.Update(msg)
		*m.contentPanel = cp
		cmds = append(cmds, cmd)
	}

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) handleNextTabKey() tea.Cmd {
	if m.selectedHost != nil {
		return m.setVisibleHostTab(m.selectedHost.hostTab + 1)
	}

	return nil
}

func (m *Model) setVisibleHostTab(hostTab int) tea.Cmd {
	if m.selectedHost != nil {
		m.selectedHost.hostTab = hostTab % len(hostTabNames)
		return m.updateContentPanel()
	}

	return nil
}

// Updates the main contentPanel viewport for current host & tab.
// Multiple viewports are used to maintain scroll position when switching.
func (m *Model) updateContentPanel() tea.Cmd {
	var cmd tea.Cmd

	if m.selectedHost != nil {
		switch m.selectedHost.hostTab {
		case hostTabLog:
			m.contentPanel = &m.selectedHost.log.contentPanel
			// Force log content update.
			cmd = func() tea.Msg { return hostLogUpdatedMsg{m.selectedHost} }
		case hostTabDeploy:
			m.contentPanel = &m.selectedHost.deploy.contentPanel
		case hostTabRunCmd:
			m.contentPanel = &m.selectedHost.runCmd.contentPanel
		case hostTabStatus:
			m.contentPanel = &m.selectedHost.status.contentPanel
		default:
			slog.Error("Unknown host tab index (bug)", "index", m.selectedHost.hostTab)
			return nil
		}

		m.contentPanel.SetWidth(m.sizes.contentPanel.width)
		m.contentPanel.SetHeight(m.sizes.contentPanel.height)
		m.ready = true
	}

	return cmd
}

func (m *Model) handleHostChangedMsg(msg hostChangedMsg) tea.Cmd {
	if m.selectedHost != nil && m.selectedHost.name == msg.hostName {
		return nil
	}

	m.selectedHost = m.hosts[msg.hostName]
	m.updateContentPanel()

	// Invalidate previous hover timer.
	m.hoverTimerID++

	// Trigger fetch status after timeout.
	myTimerID := m.hoverTimerID
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return hostHoverMsg{hostName: msg.hostName, timerID: myTimerID}
	})
}

func (m *Model) handleHostHoverMsg(msg hostHoverMsg) tea.Cmd {
	if msg.timerID != m.hoverTimerID {
		// Timer has been invalidated.
		return nil
	}

	hostName := msg.hostName
	host := m.hosts[hostName]

	if host.target == nil {
		// Must collect target info before querying host status.
		return m.hostTargetInfoCmd(host)
	}

	if host.status.collected {
		// Only collect status on hover once.
		return nil
	}

	if host.sshState == sshCheckFailed {
		// SSH check previously failed; user must press `s` to retry.
		return nil
	}

	return m.hostStatusCmd(host)
}

func (m *Model) hostTargetInfoCmd(host *hostModel) tea.Cmd {
	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render("Querying nix for information on "+host.name) + "\n"
	host.status.contentPanel.SetContent(intro)
	m.setVisibleHostTab(hostTabStatus)
	m.updateContentPanel()

	return func() tea.Msg {
		const getNixWorkerTimeout = 30 * time.Second

		ctx, done := context.WithTimeout(context.Background(), getNixWorkerTimeout)
		defer done()

		worker, err := m.nixPool.Get(ctx)
		if err != nil {
			slog.Error("failed to get nix worker", "err", err, "timeout", getNixWorkerTimeout)
			return nil
		}
		defer worker.Done()

		slog.Info("Fetching target info from nix", "host", host.name, "worker", worker)
		targetInfo, nerr := nix.GetTargetInfo(nix.TargetInfoRequest{
			FlakePath: m.flakePath,
			HostName:  host.name,
			Config:    m.config,
		})
		if nerr != nil {
			slog.Error("Failed to fetch target info from nix",
				"host", host.name, "worker", worker, "err", nerr)
			return criticalErrorMsg{detail: nerr.Error()}
		}
		slog.Debug("Got target info", "host", host.name, "worker", worker, "info", targetInfo)

		return hostTargetInfoMsg{hostName: host.name, target: *targetInfo}
	}
}

func (m *Model) handleHostTargetInfoMsg(msg hostTargetInfoMsg) tea.Cmd {
	// Store target info in hostModel.
	host := m.hosts[msg.hostName]
	host.target = &msg.target

	// Apply defaults.
	if m.config.Hosts.DefaultSSHDomain != "" &&
		!strings.ContainsRune(host.target.DeployHost, '.') {
		// Append default domain.
		host.target.DeployHost += "." + m.config.Hosts.DefaultSSHDomain

	}
	if host.target.DeployUser == "" {
		host.target.DeployUser = m.config.Hosts.DefaultSSHUser
	}

	// Verify SSH connectivity before attempting status.
	return m.hostSSHCheckCmd(host)
}

func (m *Model) hostSSHCheckCmd(host *hostModel) tea.Cmd {
	if host.target == nil {
		return nil
	}

	// Prevent duplicate in-flight checks.
	if host.sshState == sshCheckPending {
		return nil
	}
	host.sshState = sshCheckPending

	// Show the user we're checking connectivity.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render("Checking SSH connectivity to " + host.target.DeployHost + "...") + "\n"
	host.status.contentPanel.SetContent(intro)

	user := host.target.DeployUser
	hostParam := host.target.DeployHost

	return func() tea.Msg {
		err := runner.CheckSSH(m.ctx, hostParam, user)
		return hostSSHCheckMsg{hostName: host.name, err: err}
	}
}

func (m *Model) handleHostSSHCheckMsg(msg hostSSHCheckMsg) tea.Cmd {
	host := m.hosts[msg.hostName]
	if msg.err == nil {
		slog.Debug("SSH check passed", "host", msg.hostName)
		host.sshState = sshCheckPassed

		// Proceed with status collection.
		return m.hostStatusCmd(host)
	}

	host.sshState = sshCheckFailed

	checkErr, ok := msg.err.(*runner.SSHCheckError)
	if !ok {
		slog.Error("SSH check returned unexpected error type", "host", msg.hostName, "err", msg.err)
		return func() tea.Msg {
			return criticalErrorMsg{detail: "SSH check failed: " + msg.err.Error()}
		}
	}

	slog.Warn("SSH check failed", "host", msg.hostName, "message", checkErr.Message)

	// Show error in status panel.
	detail := lipgloss.NewStyle().Foreground(errorColor).Render(checkErr.Message) +
		"\n\n" + checkErr.Suggestion +
		"\n\n" + subtleStyle.Render("Press `i` to connect interactively or `s` to retry after fixing.")
	host.status.contentPanel.SetContent(detail)

	return nil
}

func (m *Model) fetchFlakeMetadataCmd() tea.Cmd {
	return func() tea.Msg {
		const timeout = 30 * time.Second

		ctx, done := context.WithTimeout(context.Background(), timeout)
		defer done()

		worker, err := m.nixPool.Get(ctx)
		if err != nil {
			slog.Error("failed to get nix worker for flake metadata", "err", err, "timeout", timeout)
			return nil
		}
		defer worker.Done()

		slog.Info("Fetching flake metadata", "worker", worker)
		meta, err := nix.GetFlakeMetadata(m.flakePath)
		if err != nil {
			slog.Error("Failed to fetch flake metadata", "worker", worker, "err", err)
			return nil
		}
		slog.Info("Fetched flake metadata",
			"worker", worker, "revision", meta.Revision, "fingerprint", meta.Fingerprint)

		return flakeMetadataMsg{meta: *meta}
	}
}

func (m *Model) handleFlakeMetadataMsg(msg flakeMetadataMsg) tea.Cmd {
	if err := m.db.StoreFlakeVersion(msg.meta); err != nil {
		slog.Error("Failed to store flake version", "err", err)
	}
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return flakeMetadataTickMsg{}
	})
}

func (m *Model) handleOpenPagerMsg(_ openPagerMsg) tea.Cmd {
	// Write visible runner buffer to temp file.
	f, err := os.CreateTemp("", "*.txt")
	if err != nil {
		slog.Error("Failed to create temp file", "err", err)
		return errorFlashCmd("Pager: %s", err)
	}

	var withErr error
	m.withVisibleRunner(func(r *runner.Model) {
		if _, err := r.CopyTo(f); err != nil {
			_ = f.Close()
			slog.Error("Failed to write temp file", "err", err)
			withErr = err
			return
		}

	})
	if withErr != nil {
		return errorFlashCmd("Pager: %s", withErr)
	}

	fname := f.Name()
	if err := f.Close(); err != nil {
		slog.Error("Failed to close temp file", "err", err)
		return errorFlashCmd("Pager: %s", err)
	}

	// TODO handle pager arguments.
	cmd := exec.Command(m.config.General.Pager, fname)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(fname)

		if err != nil {
			slog.Error("Pager failed", "err", err)
			return errorFlashMsg{text: "Pager: " + err.Error()}
		}

		return nil
	})
}

func (m *Model) handleErrorFlashMsg(msg errorFlashMsg) tea.Cmd {
	if m.flashTimer != nil {
		m.flashTimer.Stop()
	}

	if msg.text == "" {
		m.flashText = ""
		m.flashTimer = nil
		return nil
	}

	m.flashText = msg.text
	m.flashTimer = time.NewTimer(5 * time.Second)
	return func() tea.Msg {
		<-m.flashTimer.C
		return errorFlashMsg{text: ""}
	}
}

func (m *Model) startHostInteractiveSSH() tea.Cmd {
	host := m.selectedHost
	if ok, cmd := requireHostTarget("startHostInteractiveSSH", host); !ok {
		return cmd
	}

	slog.Info("starting interactive SSH", "host", host.name)

	// TODO look into tea.ExecCommand interface to display destination host to user, handle errors.
	dest := host.target.SSHDestination()
	cmd := exec.Command("ssh", dest)
	prog := m.program

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			prog.ReleaseTerminal()
			defer prog.RestoreTerminal()

			slog.Error("Interactive SSH failed", "cmd", cmd, "error", err)

			fmt.Fprintf(os.Stderr, "\n%v\n\n[Press enter to continue]", err)
			fmt.Scanln()
		}

		return nil
	})
}

var (
	errorColor     = lipgloss.Color("172")
	subtleColor    = lipgloss.Color("241")
	confirmColor   = lipgloss.Color("220")
	highlightColor = lipgloss.Color("170")
	labelFgColor   = lipgloss.Color("230")
	labelBgColor   = lipgloss.Color("62")
	successColor   = lipgloss.Color("119")
	failureColor   = lipgloss.Color("196")

	subtleStyle = lipgloss.NewStyle().Foreground(subtleColor)
	labelStyle  = lipgloss.NewStyle().MarginTop(1).Padding(0, 1).
			Foreground(labelFgColor).Background(labelBgColor)
	contentFooterStyle = lipgloss.NewStyle().Reverse(true).Padding(0, 1)
	contentPanelStyle  = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, true, true).Padding(0, 1)
	hintBarStyle       = lipgloss.NewStyle().Padding(0, 1)
	errorFlashStyle    = hintBarStyle.Foreground(errorColor)
	confirmDialogStyle = hintBarStyle.Foreground(confirmColor)

	hostListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#80c080"))

	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	activeTabStyle    = lipgloss.NewStyle().Border(activeTabBorder, true).Padding(0, 1)
	inactiveTabStyle  = activeTabStyle.Border(inactiveTabBorder, true).Foreground(subtleColor)
	tabSuffixStyle    = lipgloss.NewStyle().Border(tabSuffixBorder(), true).Padding(0, 1)
)

// View implements tea.Model.
func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("\n")
	}

	hosts := hostListStyle.Width(m.sizes.hostList.width + 4).Render(m.hostList.View())

	scroll := "(END)"
	if !m.contentPanel.AtBottom() {
		scroll = fmt.Sprintf("%.0f%%", m.contentPanel.ScrollPercent()*100)
	}

	hostName := "None"
	selectedTab := 0
	if m.selectedHost != nil {
		selectedTab = m.selectedHost.hostTab
		hostName = m.selectedHost.name

		m.withVisibleRunner(func(r *runner.Model) {
			if r.Running() {
				scroll += " - Running"
			}
		})
	}

	var renderedTabs []string
	var tabBarWidth int
	for i, t := range hostTabNames {
		var style lipgloss.Style
		isFirst, isActive := i == 0, i == selectedTab
		if isActive {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		}
		style = style.Border(border)
		rendered := style.Render(t)
		tabBarWidth += lipgloss.Width(rendered)
		renderedTabs = append(renderedTabs, rendered)
	}

	barSuffix := lipgloss.PlaceHorizontal(
		m.sizes.contentHeader.width-tabBarWidth, lipgloss.Center, hostName)
	renderedTabs = append(renderedTabs, tabSuffixStyle.Render(barSuffix))
	contentHeader := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	contentFooter := contentFooterStyle.Render(scroll)
	content := contentHeader + "\n" +
		contentPanelStyle.Render(m.contentPanel.View()+"\n"+contentFooter)

	hintBar := ""
	switch {
	case m.flashText != "":
		hintBar = errorFlashStyle.Render(m.flashText)

	case m.jumpToLetter:
		hintBar = confirmDialogStyle.Render("Jump to letter: ")

	default:
		hintBar = hintBarStyle.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	}

	content = lipgloss.JoinHorizontal(lipgloss.Top, hosts, content) + "\n" + hintBar

	switch m.viewMode {
	case viewModeText:
		dialogContent := m.text + "\n\n" + subtleStyle.Render("[Press any key to continue]")
		dialog := NewDialog(dialogContent, lipgloss.Color("#7dcfff"))
		content = dialog.Overlay(content, m.sizes.screen.width, m.sizes.screen.height)

	case viewModeError:
		dialogContent := labelStyle.Render("Critical Error") +
			"\n\n" + m.error + "\n\n" + subtleStyle.Render("[Press Esc to continue]")
		dialog := NewDialog(dialogContent, lipgloss.Color("#ff6b6b"))
		content = dialog.Overlay(content, m.sizes.screen.width, m.sizes.screen.height)

	case viewModeConfirm:
		dialogContent := m.confirmation.text + "\n\n" + subtleStyle.Render("[y/n]")
		dialog := NewDialog(dialogContent, lipgloss.Color("#e0af68"))
		content = dialog.Overlay(content, m.sizes.screen.width, m.sizes.screen.height)
	}

	if m.inputOverlay != nil && m.inputOverlay.IsVisible() {
		content = m.inputOverlay.Overlay(content, m.sizes.screen.width, m.sizes.screen.height)
	}

	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		content = m.commandPalette.Overlay(content, m.sizes.screen.width, m.sizes.screen.height)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// Calcuate size of panels based on window dimensions.
func calculateSizes(win tea.WindowSizeMsg) layoutSizes {
	const minHostListWidth = 20

	var (
		s           layoutSizes
		frameWidth  int
		frameHeight int
	)

	s.screen.width = win.Width
	s.screen.height = win.Height

	// Host list and hint bar.
	s.hintBar.height = 1
	hintBarHeight := s.hintBar.height + hintBarStyle.GetVerticalFrameSize()

	frameWidth, frameHeight = hostListStyle.GetFrameSize()
	hostListWidth := int(math.Max(minHostListWidth, float64(win.Width)*0.2))
	s.hostList.width = hostListWidth - frameWidth
	s.hostList.height = win.Height - hintBarHeight - frameHeight

	s.hintBar.width = win.Width - hostListWidth - hintBarStyle.GetHorizontalFrameSize()

	// Content.
	frameWidth, frameHeight = activeTabStyle.GetFrameSize()
	s.contentHeader.width = win.Width - hostListWidth - frameWidth
	s.contentHeader.height = 1
	contentHeaderHeight := s.contentHeader.height + frameHeight
	contentFooterHeight := 1

	frameWidth, frameHeight = contentPanelStyle.GetFrameSize()
	s.contentPanel.width = win.Width - hostListWidth - frameWidth
	s.contentPanel.height = win.Height -
		contentHeaderHeight - contentFooterHeight - hintBarHeight - frameHeight

	return s
}

// Confirms we have nix info for the specified host.
func requireHostTarget(logName string, host *hostModel) (bool, tea.Cmd) {
	if host == nil {
		slog.Error(logName + " called with nil host (bug)")
		return false, nil
	}
	if host.target == nil {
		return false, errorFlashCmd("Target info for host %q not yet available", host.name)
	}

	return true, nil
}

func (m *Model) withVisibleRunner(fn func(*runner.Model)) {
	var runner *runner.Model

	switch m.selectedHost.hostTab {
	case hostTabDeploy:
		runner = m.selectedHost.deploy.runner
	case hostTabRunCmd:
		runner = m.selectedHost.runCmd.runner
	case hostTabStatus:
		runner = m.selectedHost.status.runner
	}

	if runner != nil {
		fn(runner)
	}
}

func errorFlashCmd(format string, a ...any) tea.Cmd {
	text := fmt.Sprintf(format, a...)

	return func() tea.Msg {
		return errorFlashMsg{
			text: text,
		}
	}
}

func (m *Model) commands() []Command {
	return []Command{
		{
			Name:        "fingerprints",
			Description: "List stored flake fingerprints",
			Execute:     (*Model).cmdListFingerprints,
		},
		{
			Name:        "hosts",
			Description: "List configured hosts",
			Execute:     (*Model).cmdListHosts,
		},
	}
}

func (m *Model) cmdListFingerprints() tea.Cmd {
	return func() tea.Msg {
		versions, err := m.db.ListFlakeVersions()
		if err != nil {
			slog.Error("Failed to list flake versions", "err", err)
			return criticalErrorMsg{detail: "Failed to list flake versions: " + err.Error()}
		}

		if len(versions) == 0 {
			return textDisplayMsg{text: "No flake fingerprints stored."}
		}

		var b strings.Builder
		b.WriteString("Stored Flake Fingerprints\n\n")

		for _, v := range versions {
			dirty := ""
			if v.Dirty {
				dirty = " (dirty)"
			}
			fmt.Fprintf(&b, "  %s%s\n", v.Fingerprint, dirty)
			fmt.Fprintf(&b, "    Revision: %s\n", v.Revision)
			fmt.Fprintf(&b, "    Last Modified: %s\n", v.LastModified.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(&b, "    Stored At: %s\n\n", v.StoredAt.Format("2006-01-02 15:04:05"))
		}

		return textDisplayMsg{text: b.String()}
	}
}

func (m *Model) cmdListHosts() tea.Cmd {
	return func() tea.Msg {
		var b strings.Builder
		b.WriteString("Configured Hosts\n\n")

		names := make([]string, 0, len(m.hosts))
		for name := range m.hosts {
			names = append(names, name)
		}
		slices.Sort(names)

		for _, name := range names {
			host := m.hosts[name]
			line := "  " + name
			if host.target != nil {
				line += subtleStyle.Render(" → " + host.target.DeployHost)
			}
			b.WriteString(line + "\n")
		}

		return textDisplayMsg{text: b.String()}
	}
}

const maxCmdHistory = 10

func (m *Model) addToCmdHistory(cmd string) {
	for i, c := range m.cmdHistory {
		if c == cmd {
			m.cmdHistory = append(m.cmdHistory[:i], m.cmdHistory[i+1:]...)
			break
		}
	}
	m.cmdHistory = append(m.cmdHistory, cmd)
	if len(m.cmdHistory) > maxCmdHistory {
		m.cmdHistory = m.cmdHistory[len(m.cmdHistory)-maxCmdHistory:]
	}
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

func tabSuffixBorder() lipgloss.Border {
	border := lipgloss.NormalBorder()
	border.BottomRight = border.TopRight
	border.BottomLeft = border.Bottom

	border.Top = " "
	border.TopRight = " "
	border.Right = " "
	border.TopLeft = " "
	border.Left = " "
	return border
}
