package main

import (
	"fmt"
	"log/slog"
	"math"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
)

type model struct {
	hostList hostListModel
	status   map[string]string
	sizes    layoutSizes
}

type layoutSizes struct {
	hostList     dim
	contentPanel dim
}

type dim struct {
	width  int
	height int
}

func initialModel() model {
	return model{
		hostList: newHostList(flakeHosts()),
		status:   make(map[string]string),
	}
}

type statusMsg struct {
	host   string
	status string
	runner *runner.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m model) View() string {
	hosts := hostListStyle.Render(m.hostList.View())

	status := "\n"
	selected := m.hostList.Selected()
	if selected != nil {
		if text, ok := m.status[string(*selected)]; ok {
			status = text
		}
	}
	content := contentPanelStyle.Render(status)

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, content)
}

func statusCmd(m model, host hostItem) (model, tea.Cmd) {
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
	s.hostList.width = int(math.Max(10, float64(msg.Width)*0.2)) - frameX
	s.hostList.height = msg.Height - frameY

	frameX, frameY = contentPanelStyle.GetFrameSize()
	s.contentPanel.width = int(math.Max(10, float64(msg.Width)*0.2)) - frameX
	s.contentPanel.height = msg.Height - frameY

	return s
}

func main() {
	// Init logging.
	lf, err := tea.LogToFile("debug.log", "")
	if err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
	defer lf.Close()

	slog.Info("### STARTUP ###################################################################")

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("oops: %v", err)
		os.Exit(1)
	}
}

func flakeHosts() []string {
	return []string{"fastd", "metrics", "longlonglonglonglonglonglonglonglonglong", "web"}
}
