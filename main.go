package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	hosts  []string
	cursor int
	status map[string]string
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

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.hosts)-1 {
				m.cursor++
			}

		case "enter", " ":
			return m, statusCmd(m.hosts[m.cursor])

		}
	case statusMsg:
		m.status[msg.host] = msg.status
	}

	return m, nil
}

// View implements tea.Model.
func (m model) View() string {
	s := "Pick a host:\n\n"

	for i, choice := range m.hosts {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	if status, ok := m.status[m.hosts[m.cursor]]; ok {
		s += "\nStatus: " + status + "\n"
	}

	s += "\nPress q to quit.\n"

	return s
}

func initialModel() model {
	return model{
		hosts:  flakeHosts(),
		status: make(map[string]string),
	}
}

func statusCmd(host string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{host: host,
			status: hostStatus(host),
		}
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("oops: %v", err)
		os.Exit(1)
	}
}

func flakeHosts() []string {
	return []string{"fastd", "metrics", "web"}
}

func hostStatus(host string) string {
	return fmt.Sprintf("meh, %q seems fine", host)
}
