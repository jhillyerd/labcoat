package ui

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/nix"
	"github.com/jhillyerd/labui/internal/npool"
	"github.com/jhillyerd/labui/internal/runner"
)

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	Filter     key.Binding
	Status     key.Binding
	Quit       key.Binding
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Filter, k.Status, k.Quit}}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Filter, k.Status, k.Quit}
}

type Model struct {
	ready        bool // true once screen size is known.
	flakePath    string
	hostList     hostListModel
	hosts        map[string]*hostModel
	selectedHost *hostModel
	hoverTimer   *time.Timer // Triggers host status collection when user hovers.
	nixPool      *npool.Pool
	contentPanel viewport.Model
	sizes        layoutSizes
	keys         KeyMap
	help         help.Model
	spinner      spinner.Model
}

type hostModel struct {
	name   string
	target *nix.TargetInfo // Cached info about target host.
	status struct {
		intro    string // Rendered intro text: command, host, etc.
		rendered string // Rendered status content cache.
		runner   *runner.Model
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

func New(keys KeyMap, flakePath string, hostNames []string) Model {
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
		hosts[v] = &hostModel{name: v}
	}

	return Model{
		flakePath: flakePath,
		hostList:  hostList,
		hosts:     hosts,
		nixPool:   npool.New("nix", 2),
		keys:      keys,
		help:      help.New(),
		spinner:   spin,
	}
}

type hostTargetInfoMsg struct {
	hostName string
	target   nix.TargetInfo
}

type hostStatusMsg struct {
	hostName string
}

type hostChangedMsg struct {
	hostName string
}

type hostHoverMsg struct {
	hostName string
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
		slog.Debug("tea.KeyMsg", "key", msg)

		if m.hostList.FilterState() == list.Filtering {
			// User is entering filter text, disable keymaps.
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
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

	case tea.WindowSizeMsg:
		m.sizes = calculateSizes(msg)
		m.hostList.SetSize(m.sizes.hostList.width, m.sizes.hostList.height)

		if m.ready {
			m.contentPanel.Width = m.sizes.contentPanel.width
			m.contentPanel.Height = m.sizes.contentPanel.height
		} else {
			// First size message, init content viewport.
			m.initContentPanel()
			m.ready = true
		}
		return m, nil

	case tea.Cmd:
		slog.Error("Got tea.Cmd instead of tea.Msg")
		return m, nil
	}

	m.hostList, cmd = m.hostList.Update(msg)
	cmds = append(cmds, cmd)

	m.contentPanel, cmd = m.contentPanel.Update(msg)
	cmds = append(cmds, cmd)

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) initContentPanel() {
	cp := viewport.New(m.sizes.contentPanel.width, m.sizes.contentPanel.height)
	cp.KeyMap.PageUp = m.keys.ScrollUp
	cp.KeyMap.PageDown = m.keys.ScrollDown

	cp.KeyMap.Up = key.NewBinding(key.WithDisabled())
	cp.KeyMap.Down = key.NewBinding(key.WithDisabled())
	cp.KeyMap.HalfPageUp = key.NewBinding(key.WithDisabled())
	cp.KeyMap.HalfPageDown = key.NewBinding(key.WithDisabled())

	m.contentPanel = cp
}

func (m *Model) handleHostChangedMsg(msg hostChangedMsg) tea.Cmd {
	slog.Debug("hostChanged", "host", msg.hostName)

	m.selectedHost = m.hosts[msg.hostName]
	m.contentPanel.SetContent(m.hosts[msg.hostName].status.rendered)

	if m.hoverTimer != nil {
		// Discard timer for previous host.
		m.hoverTimer.Stop()
		m.hoverTimer = nil
	}

	// Do nothing if status is already running.
	statusRunner := m.selectedHost.status.runner
	if statusRunner != nil && statusRunner.Running() {
		return nil
	}

	// Trigger fetch status after timeout.
	m.hoverTimer = time.NewTimer(500 * time.Millisecond)
	return func() tea.Msg {
		<-m.hoverTimer.C
		return hostHoverMsg{hostName: msg.hostName}
	}
}

func (m *Model) handleHostHoverMsg(msg hostHoverMsg) tea.Cmd {
	const getNixWorkerTimeout = 30 * time.Second

	hostName := msg.hostName

	host := m.hosts[hostName]

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render("Querying nix for information on "+hostName) + "\n"
	host.status.intro = intro
	host.status.rendered = intro
	m.contentPanel.SetContent(intro)

	return func() tea.Msg {
		ctx, done := context.WithTimeout(context.Background(), getNixWorkerTimeout)
		defer done()

		worker, err := m.nixPool.Get(ctx)
		if err != nil {
			slog.Error("failed to get nix worker", "err", err, "timeout", getNixWorkerTimeout)
			return nil
		}
		defer worker.Done()

		slog.Info("Fetching target info from nix", "host", hostName, "worker", worker)
		targetInfo, nerr := nix.GetTargetInfo(nix.TargetInfoRequest{
			FlakePath: m.flakePath,
			HostName:  hostName,
		})
		if nerr != nil {
			slog.Error("Failed to fetch target info",
				"host", hostName, "worker", worker, "err", nerr, "detail", nerr.Detail())
			return nil
		}
		slog.Info("targetInfo", "info", targetInfo)

		return hostTargetInfoMsg{hostName: hostName, target: *targetInfo}
	}
}

func (m *Model) handleHostTargetInfoMsg(msg hostTargetInfoMsg) tea.Cmd {
	hostName := msg.hostName
	target := &msg.target
	m.hosts[hostName].target = target

	// Fetch host status now that we know target deployHost.
	onUpdate := func(r *runner.Model) tea.Msg {
		// Sent when the runner has new output to display.
		return hostStatusMsg{hostName: hostName}
	}

	script := runner.NewScript([]string{"systemctl --failed", "uname -a", "df -h"})
	srunner := runner.NewRemoteScript(
		onUpdate, target.DeployHost, "root", "host status (script)", script)

	host := m.hosts[hostName]
	host.status.runner = srunner

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()+" @ "+srunner.Destination()) + "\n"
	host.status.intro = intro
	host.status.rendered = intro
	m.contentPanel.SetContent(intro)

	return srunner.Init()
}

func (m *Model) handleHostStatusMsg(msg hostStatusMsg) tea.Cmd {
	host := m.hosts[msg.hostName]
	srunner := host.status.runner
	_, cmd := srunner.Update(nil)

	// Render and cache status content.
	status := host.status.intro
	status += runner.FormatOutput(
		srunner.View(),
		func(s string) string { return scriptLabelStyle.Render(s) })
	host.status.rendered = status

	if m.selectedHost == host {
		// User is currently viewing the host receiving this status, update the panel content.
		m.contentPanel.SetContent(status)
	}

	return cmd
}

var subtleColor = lipgloss.Color("241")
var labelFgColor = lipgloss.Color("230")
var labelBgColor = lipgloss.Color("62")
var scriptLabelStyle = lipgloss.NewStyle().MarginTop(1).Padding(0, 1).
	Foreground(labelFgColor).Background(labelBgColor)

var hostListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
var contentHeaderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
var contentPanelStyle = lipgloss.NewStyle().Border(
	lipgloss.NormalBorder(), false, true, true, true).Padding(0, 1)
var hintBarStyle = lipgloss.NewStyle().Padding(0, 1)

// View implements tea.Model.
func (m Model) View() string {
	// Postpone rendering until screen dimensions are known.
	if !m.ready {
		return "\n"
	}

	hosts := hostListStyle.Render(m.hostList.View())

	hostName := "None"
	state := "Unknown"
	scroll := "---%"

	if m.selectedHost != nil {
		hostName = m.selectedHost.name
		if sr := m.selectedHost.status.runner; sr != nil {
			state = sr.StateString()
		}
	}

	scroll = fmt.Sprintf("%3.0f%%", m.contentPanel.ScrollPercent()*100)

	contentHeader := contentHeaderStyle.Render(
		lipgloss.PlaceHorizontal(m.sizes.contentHeader.width, lipgloss.Center,
			"Status | "+hostName+" | "+state+" | Scroll: "+scroll))
	content := contentHeader + "\n" +
		contentPanelStyle.Render(m.contentPanel.View())

	hintBar := hintBarStyle.Render(m.help.View(m.keys))

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content) + "\n" + hintBar
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
