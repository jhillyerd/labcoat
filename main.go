package main

import (
	"fmt"
	"log/slog"
	"math"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	hostList hostListModel
	status   map[string]string
	width    int
	height   int
}

func initialModel() model {
	return model{
		hostList: newHostList(flakeHosts()),
		status:   make(map[string]string),
		width:    80,
		height:   25,
	}
}

type statusMsg struct {
	host   string
	status string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case statusMsg:
		m.status[msg.host] = msg.status
		return m, nil

	case statusTimeoutMsg:
		slog.Info("status timeout", "host", msg.host)
		return m, statusCmd(msg.host)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		setHostListSize(&m.hostList, msg)

		return m, nil
	}

	var cmd tea.Cmd
	m.hostList, cmd = m.hostList.Update(msg)

	return m, cmd
}

// View implements tea.Model.
func (m model) View() string {
	hosts := m.hostList.View()

	status := "\n"
	selected := m.hostList.Selected()
	if selected != nil {
		if text, ok := m.status[string(*selected)]; ok {
			status = "Status: " + text + "\n"
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, hosts, status)
}

func statusCmd(host hostItem) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{
			host:   string(host),
			status: getHostStatus(string(host)),
		}
	}
}

// Calcuate size of hostList based on screen dimensions.
func setHostListSize(hl *hostListModel, msg tea.WindowSizeMsg) {
	width := int(math.Max(10, float64(msg.Width)*0.2))

	slog.Info("host list", "width", width)

	hl.SetSize(width, msg.Height)
}

func main() {
	// Init logging.
	lf, err := tea.LogToFile("debug.log", "")
	if err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
	defer lf.Close()

	slog.Info("### STARTUP ###")

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("oops: %v", err)
		os.Exit(1)
	}
}

func flakeHosts() []string {
	return []string{"fastd", "metrics", "longlonglonglonglonglonglonglonglonglong", "web"}
}

func getHostStatus(host string) string {
	return fmt.Sprintf("meh, %q seems fine", host)
}
