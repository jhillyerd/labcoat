package ui

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/config"
	"github.com/jhillyerd/labui/internal/nix"
	"github.com/jhillyerd/labui/internal/npool"
	"github.com/jhillyerd/labui/internal/runner"
)

const (
	viewModeHosts = iota
	viewModeError
)

const (
	hostTabStatus = iota
	hostTabDeploy
	hostTabRunCmd
)

var hostTabNames = []string{"Host Status", "Deploy", "Run Command"}

type Model struct {
	ctx          context.Context
	program      *tea.Program
	config       config.Config
	ready        bool // true once screen size is known.
	viewMode     int  // Current UI mode.
	flakePath    string
	hostList     hostListModel
	hosts        map[string]*hostModel
	selectedHost *hostModel
	hoverTimer   *time.Timer // Triggers host status collection when user hovers.
	nixPool      *npool.Pool
	contentPanel *viewport.Model
	sizes        layoutSizes
	keys         config.KeyMap
	help         help.Model
	spinner      spinner.Model
	confirmation *confirmationMsg
	error        string
	flashText    string
	flashTimer   *time.Timer
}

type hostModel struct {
	name    string
	target  *nix.TargetInfo // Cached info about target host.
	hostTab int             // Currently visible host tab.
	deploy  struct {
		intro        string // Rendered intro text: command, host, etc.
		contentPanel viewport.Model
		runner       *runner.Model
		cancel       func()
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
	hostList      dim
	contentHeader dim
	contentPanel  dim
	hintBar       dim
}

type dim struct {
	width  int
	height int
}

func New(conf config.Config, keys config.KeyMap, flakePath string, hostNames []string) Model {
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
		hm.status.contentPanel = newContentPanel(keys)
		hm.runCmd.contentPanel = newContentPanel(keys)
		hosts[v] = hm
	}

	return Model{
		ctx:       context.Background(),
		config:    conf,
		viewMode:  viewModeHosts,
		flakePath: flakePath,
		hostList:  hostList,
		hosts:     hosts,
		nixPool:   npool.New("nix", 2),
		keys:      keys,
		help:      help.New(),
		spinner:   spin,
	}
}

func newContentPanel(keys config.KeyMap) viewport.Model {
	cp := viewport.New(80, 25)

	// TODO consider handling keymaps/mouse in ui.Update().
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
}

type confirmationMsg struct {
	text   string
	yesCmd tea.Cmd
	noCmd  tea.Cmd
}

type criticalErrorMsg struct {
	detail string
}

type errorFlashMsg struct {
	text string
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.hostList.Init(),
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// slog.Debug("tea.KeyMsg", "key", msg)

		if msg.String() == "ctrl+\\" {
			// Ctrl-\ overrides all view states to exit.
			return m, tea.Quit
		}

		if m.viewMode == viewModeError {
			// Error display is modal, swallow all key press messages.
			if msg.String() == "esc" {
				// Exit error display.
				m.viewMode = viewModeHosts
			}

			return m, nil
		}

		if msg.String() == "ctrl+c" {
			m.withVisibleRunner(func(r *runner.Model) {
				r.Cancel()
			})

			return m, nil
		}

		if m.hostList.FilterState() == list.Filtering {
			// User is entering filter text, disable keymaps.
			break
		}

		if m.confirmation != nil {
			// Awaiting confirmation, `y` or `n` will trigger the corresponding cmd.
			if msg.String() == "y" {
				cmd = m.confirmation.yesCmd
				m.confirmation = nil
				return m, cmd
			}

			if msg.String() == "n" {
				cmd = m.confirmation.noCmd
				m.confirmation = nil
				return m, cmd
			}

			slog.Debug("Invalid confirmation keypress", "key", msg)
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.NextTab):
			return m, m.handleNextTabKey()

		case key.Matches(msg, m.keys.Deploy):
			return m, m.hostDeployCmd(m.selectedHost)

		case key.Matches(msg, m.keys.Reboot):
			return m, func() tea.Msg {
				return confirmationMsg{
					text:   fmt.Sprintf("Confirm reboot of %q? y/n:", m.selectedHost.target.DeployHost),
					yesCmd: m.hostRunCommandCmd(m.selectedHost, "/run/current-system/sw/bin/reboot"),
				}
			}

		case key.Matches(msg, m.keys.Status):
			return m, m.hostStatusCmd(m.selectedHost)

		case key.Matches(msg, m.keys.SSHInto):
			return m, m.startHostInteractiveSSH()

		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}

	case hostChangedMsg:
		return m, m.handleHostChangedMsg(msg)

	case hostHoverMsg:
		return m, m.handleHostHoverMsg(msg)

	case hostTargetInfoMsg:
		return m, m.handleHostTargetInfoMsg(msg)

	case hostStatusMsg:
		return m, m.handleHostStatusMsg(msg)

	case hostDeployMsg:
		return m, m.handleHostDeployMsg(msg)

	case hostDeployOutputMsg:
		return m, m.handleHostDeployOutputMsg(msg)

	case hostRunCommandMsg:
		return m, m.handleHostRunCommandMsg(msg)

	case hostRunCommandOutputMsg:
		return m, m.handleHostRunCommandOutputMsg(msg)

	case tea.WindowSizeMsg:
		m.sizes = calculateSizes(msg)
		m.hostList.SetSize(m.sizes.hostList.width, m.sizes.hostList.height)
		m.updateContentPanel()

		return m, nil

	case confirmationMsg:
		m.confirmation = &msg
		return m, nil

	case criticalErrorMsg:
		m.viewMode = viewModeError
		m.error = msg.detail

	case errorFlashMsg:
		return m, m.handleErrorFlashMsg(msg)

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
		m.setVisibleHostTab(m.selectedHost.hostTab + 1)
	}

	return nil
}

func (m *Model) setVisibleHostTab(hostTab int) {
	if m.selectedHost != nil {
		m.selectedHost.hostTab = hostTab % len(hostTabNames)
		m.updateContentPanel()
	}
}

// Updates the main contentPanel viewport for current host & tab.
// Multiple viewports are used to maintain scroll position when switching.
func (m *Model) updateContentPanel() {
	if m.selectedHost != nil {
		switch m.selectedHost.hostTab {
		case hostTabStatus:
			m.contentPanel = &m.selectedHost.status.contentPanel
		case hostTabDeploy:
			m.contentPanel = &m.selectedHost.deploy.contentPanel
		case hostTabRunCmd:
			m.contentPanel = &m.selectedHost.runCmd.contentPanel
		default:
			slog.Error("Unknown host tab index (bug)", "index", m.selectedHost.hostTab)
			return
		}

		m.contentPanel.Width = m.sizes.contentPanel.width
		m.contentPanel.Height = m.sizes.contentPanel.height
		m.ready = true
	}
}

func (m *Model) handleHostChangedMsg(msg hostChangedMsg) tea.Cmd {
	// slog.Debug("hostChanged", "host", msg.hostName)

	m.selectedHost = m.hosts[msg.hostName]
	m.updateContentPanel()

	if m.hoverTimer != nil {
		// Discard timer for previous host.
		m.hoverTimer.Stop()
		m.hoverTimer = nil
	}

	// Trigger fetch status after timeout.
	m.hoverTimer = time.NewTimer(500 * time.Millisecond)
	return func() tea.Msg {
		<-m.hoverTimer.C
		return hostHoverMsg{hostName: msg.hostName}
	}
}

func (m *Model) handleHostHoverMsg(msg hostHoverMsg) tea.Cmd {
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
		strings.IndexRune(host.target.DeployHost, '.') == -1 {
		// Append default domain.
		host.target.DeployHost += "." + m.config.Hosts.DefaultSSHDomain

	}
	if host.target.DeployUser == "" {
		host.target.DeployUser = m.config.Hosts.DefaultSSHUser
	}

	// Fetch host status now that we know target info.
	return m.hostStatusCmd(host)
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
	errorColor   = lipgloss.Color("172")
	subtleColor  = lipgloss.Color("241")
	confirmColor = lipgloss.Color("220")
	labelFgColor = lipgloss.Color("230")
	labelBgColor = lipgloss.Color("62")

	subtleStyle = lipgloss.NewStyle().Foreground(subtleColor)
	labelStyle  = lipgloss.NewStyle().MarginTop(1).Padding(0, 1).
			Foreground(labelFgColor).Background(labelBgColor)
	hostListStyle      = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
	contentHeaderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
	contentPanelStyle  = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, true, true).Padding(0, 1)
	hintBarStyle       = lipgloss.NewStyle().Padding(0, 1)
	errorFlashStyle    = hintBarStyle.Copy().Foreground(errorColor)
	confirmDialogStyle = hintBarStyle.Copy().Foreground(confirmColor)
)

// View implements tea.Model.
func (m Model) View() string {
	// Postpone rendering until screen dimensions are known.
	if !m.ready {
		return "\n"
	}

	switch m.viewMode {
	case viewModeHosts:
		hosts := hostListStyle.Render(m.hostList.View())

		tabName := "Unknown"
		hostName := "None"
		state := "Unknown"
		scroll := "---%"

		if m.selectedHost != nil {
			tabName = hostTabNames[m.selectedHost.hostTab]
			hostName = m.selectedHost.name

			m.withVisibleRunner(func(r *runner.Model) {
				state = r.StateString()
			})
		}

		scroll = fmt.Sprintf("%3.0f%%", m.contentPanel.ScrollPercent()*100)

		contentHeader := contentHeaderStyle.Render(
			lipgloss.PlaceHorizontal(m.sizes.contentHeader.width, lipgloss.Center,
				tabName+" | "+hostName+" | "+state+" | Scroll: "+scroll))
		content := contentHeader + "\n" +
			contentPanelStyle.Render(m.contentPanel.View())

		// Display help or error flash if present.
		hintBar := ""
		if m.flashText != "" {
			hintBar = errorFlashStyle.Render(m.flashText)
		} else if m.confirmation != nil {
			hintBar = confirmDialogStyle.Render(m.confirmation.text)
		} else {
			hintBar = hintBarStyle.Render(m.help.View(m.keys))
		}

		return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content) + "\n" + hintBar

	case viewModeError:
		return labelStyle.Render("Critical Error") +
			"\n\n" +
			m.error +
			"\n\n" +
			subtleStyle.Render("[Press Esc to continue]")
	}

	return fmt.Sprintf("Unknown view mode: %v", m.viewMode)
}

// Calcuate size of panels based on window dimensions.
func calculateSizes(win tea.WindowSizeMsg) layoutSizes {
	const minHostListWidth = 20

	var (
		s           layoutSizes
		frameWidth  int
		frameHeight int
	)

	// Host list and hint bar.
	s.hintBar.height = 1
	hintBarHeight := s.hintBar.height + hintBarStyle.GetVerticalFrameSize()

	frameWidth, frameHeight = hostListStyle.GetFrameSize()
	hostListWidth := int(math.Max(minHostListWidth, float64(win.Width)*0.2))
	s.hostList.width = hostListWidth - frameWidth
	s.hostList.height = win.Height - hintBarHeight - frameHeight

	s.hintBar.width = win.Width - hostListWidth - hintBarStyle.GetHorizontalFrameSize()

	// Content.
	frameWidth, frameHeight = contentHeaderStyle.GetFrameSize()
	s.contentHeader.width = win.Width - hostListWidth - frameWidth
	s.contentHeader.height = 1
	contentHeaderHeight := s.contentHeader.height + frameHeight

	frameWidth, frameHeight = contentPanelStyle.GetFrameSize()
	s.contentPanel.width = win.Width - hostListWidth - frameWidth
	s.contentPanel.height = win.Height - contentHeaderHeight - hintBarHeight - frameHeight

	return s
}

func requireHostTarget(logName string, host *hostModel) (bool, tea.Cmd) {
	if host == nil {
		slog.Error(logName + " called with nil host (bug)")
		return false, nil
	}
	if host.target == nil {
		return false, func() tea.Msg {
			return errorFlashMsg{
				text: fmt.Sprintf("Target info for host %q not yet available", host.name),
			}
		}
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
