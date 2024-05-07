package ui

import (
	"log/slog"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
)

type Model struct {
	ready        bool // true once screen size is known.
	hostList     hostListModel
	hoverTimer   *time.Timer // Triggers host status collection when user hovers.
	selectedHost string
	contentPanel viewport.Model
	status       map[string]*runner.Model
	sizes        layoutSizes
}

type layoutSizes struct {
	hostList     dim
	contentPanel dim
}

type dim struct {
	width  int
	height int
}

func New(hosts []string) Model {
	return Model{
		hostList: newHostList(hosts),
		status:   make(map[string]*runner.Model),
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
	return m.hostList.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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

	return m, cmd
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

var hostListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
var contentPanelStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)

// View implements tea.Model.
func (m Model) View() string {
	// Postpone rendering until screen dimensions are known.
	if !m.ready {
		return "\n"
	}

	hosts := hostListStyle.Render(m.hostList.View())

	status := "\n"
	if m.selectedHost != "" {
		if statusRunner := m.status[m.selectedHost]; statusRunner != nil {
			status = statusRunner.View()
		}
	}
	m.contentPanel.SetContent(status)
	content := contentPanelStyle.Render(m.contentPanel.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content)
}

func (m *Model) statusCmd(host string) tea.Cmd {
	onUpdate := func(r *runner.Model) tea.Msg {
		// slog.Debug("statusCmd onUpdate called", "host", host)

		return statusMsg{
			host:   host,
			runner: r,
		}
	}

	r := runner.NewLocal(onUpdate, "/home/james/slow-script.sh")
	return r.Init()
}

// Calcuate size of hostList based on screen dimensions.
func calculateSizes(msg tea.WindowSizeMsg) layoutSizes {
	var s layoutSizes
	var frameX, frameY int

	frameX, frameY = hostListStyle.GetFrameSize()
	hostListWidth := int(math.Max(10, float64(msg.Width)*0.2))
	s.hostList.width = hostListWidth - frameX
	s.hostList.height = msg.Height - frameY

	frameX, frameY = contentPanelStyle.GetFrameSize()
	s.contentPanel.width = msg.Width - hostListWidth - frameX
	s.contentPanel.height = msg.Height - frameY

	return s
}
