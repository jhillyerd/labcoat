package ui

import (
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
	"github.com/jhillyerd/labui/internal/runner"
)

type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Filter key.Binding
	Status key.Binding
	Quit   key.Binding
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
	hostList     hostListModel
	hoverTimer   *time.Timer // Triggers host status collection when user hovers.
	selectedHost string
	contentPanel viewport.Model
	status       map[string]*runner.Model
	sizes        layoutSizes
	keys         KeyMap
	help         help.Model
	spinner      spinner.Model
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

func New(keys KeyMap, hosts []string) Model {
	hostList := newHostList(hosts)
	hostList.list.KeyMap.CursorUp = keys.Up
	hostList.list.KeyMap.CursorDown = keys.Down
	hostList.list.KeyMap.Filter = keys.Filter

	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#80c080"))

	return Model{
		hostList: hostList,
		status:   make(map[string]*runner.Model),
		keys:     keys,
		help:     help.New(),
		spinner:  spin,
	}
}

type statusMsg struct {
	host   string
	status string
	runner *runner.Model
}

type hostChangedMsg struct {
	host string
}

type hostHoverMsg struct {
	host string
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
		if m.hostList.FilterState() == list.Filtering {
			// User is entering filter text, disable keymaps.
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case hostChangedMsg:
		return m, m.handleHostChange(msg)

	case hostHoverMsg:
		return m, m.statusCmd(msg.host)

	case statusMsg:
		// slog.Debug("statusMsg", "host", msg.host)
		statusRunner, cmd := msg.runner.Update(nil)
		m.status[msg.host] = statusRunner
		return m, cmd

	case tea.WindowSizeMsg:
		m.sizes = calculateSizes(msg)
		m.hostList.SetSize(m.sizes.hostList.width, m.sizes.hostList.height)

		if m.ready {
			m.contentPanel.Width = m.sizes.contentPanel.width
			m.contentPanel.Height = m.sizes.contentPanel.height
		} else {
			// First size message, init content viewport.
			m.contentPanel = viewport.New(m.sizes.contentPanel.width, m.sizes.contentPanel.height)
			m.ready = true
		}
		return m, nil

	case tea.Cmd:
		slog.Error("Got tea.Cmd instead of tea.Msg")
		return m, nil
	}

	m.hostList, cmd = m.hostList.Update(msg)
	cmds = append(cmds, cmd)

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) handleHostChange(msg hostChangedMsg) tea.Cmd {
	slog.Debug("hostChanged", "host", msg.host)

	m.selectedHost = msg.host

	if m.hoverTimer != nil {
		// Discard timer for previous host.
		m.hoverTimer.Stop()
		m.hoverTimer = nil
	}

	statusRunner := m.status[m.selectedHost]
	if statusRunner != nil && statusRunner.Running() {
		slog.Debug("status already running", "host", m.selectedHost)
		return nil
	}

	// Trigger status update after timeout.
	m.hoverTimer = time.NewTimer(500 * time.Millisecond)
	return func() tea.Msg {
		<-m.hoverTimer.C
		return hostHoverMsg{host: msg.host}
	}
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

	host := "None"
	status := "\n"
	state := "Unknown"

	if m.selectedHost != "" {
		host = m.selectedHost
		if statusRunner := m.status[m.selectedHost]; statusRunner != nil {
			state = statusRunner.StateString()

			status = lipgloss.NewStyle().Foreground(subtleColor).Render(
				statusRunner.String() + " @ " + statusRunner.Destination())
			status += "\n"

			// TODO inefficient, cache this.
			status += runner.FormatOutput(statusRunner.View(),
				func(s string) string { return scriptLabelStyle.Render(s) })

			if statusRunner.Running() {
				status += m.spinner.View()
			}
		}
	}

	contentHeader := contentHeaderStyle.Render(
		lipgloss.PlaceHorizontal(m.sizes.contentHeader.width, lipgloss.Center,
			"Status | "+host+" | "+state))
	m.contentPanel.SetContent(status)
	content := contentHeader + "\n" +
		contentPanelStyle.Render(m.contentPanel.View())

	hintBar := hintBarStyle.Render(m.help.View(m.keys))

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content) + "\n" + hintBar
}

func (m *Model) statusCmd(host string) tea.Cmd {
	onUpdate := func(r *runner.Model) tea.Msg {
		// slog.Debug("statusCmd onUpdate called", "host", host)

		return statusMsg{
			host:   host,
			runner: r,
		}
	}

	// r := runner.NewLocal(onUpdate, "/home/james/slow-script.sh")
	// r := runner.NewRemote(onUpdate, host, "root", "df", "-h")

	cmds := []string{"systemctl --failed", "uname -a", "df -h"}
	script := runner.NewScript(cmds)
	r := runner.NewRemoteScript(onUpdate, host, "root", "host status (script)", script)
	m.status[host] = r

	return r.Init()
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
