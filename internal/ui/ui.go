package ui

import (
	"log/slog"
	"math"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
)

type Model struct {
	ready        bool // true once screen size is known.
	hostList     hostListModel
	contentPanel viewport.Model
	status       map[string]string
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
		status:   make(map[string]string),
	}
}

type statusMsg struct {
	host   string
	status string
	runner *runner.Model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case statusMsg:
		slog.Info("statusMsg", "host", msg.host)
		m.status[msg.host] = msg.status
		_, cmd = msg.runner.Update(nil)
		return m, cmd

	case statusHoverMsg:
		slog.Info("statusHoverMsg", "host", msg.host)
		return statusCmd(m, msg.host)

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

var hostListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
var contentPanelStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)

// View implements tea.Model.
func (m Model) View() string {
	// Delay rendering until screen dimensions are known.
	if !m.ready {
		return "\n"
	}

	hosts := hostListStyle.Render(m.hostList.View())

	status := "\n"
	selected := m.hostList.Selected()
	if selected != nil {
		if text, ok := m.status[string(*selected)]; ok {
			status = text
		}
	}
	m.contentPanel.SetContent(status)
	content := contentPanelStyle.Render(m.contentPanel.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content)
}

func statusCmd(m Model, host hostItem) (Model, tea.Cmd) {
	onUpdate := func(r *runner.Model) tea.Msg {
		slog.Info("statusCmd onUpdate called")
		return statusMsg{
			host:   string(host),
			status: string(r.View()),
			runner: r,
		}
	}

	r := runner.NewLocal(onUpdate, "/home/james/slow-script.sh")
	return m, r.Init()
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
