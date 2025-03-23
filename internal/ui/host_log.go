package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type hostLogUpdatedMsg struct {
	host *hostModel
}

// Logs host+text to DB, returns a hostLogUpdatedMsg to update UI.
func (m *Model) hostLogCmd(host *hostModel, text string) tea.Cmd {
	db := m.db

	return func() tea.Msg {
		hostName := host.name
		slog.Debug("Writing op log", "host", hostName, "text", text)

		if err := db.WriteHostLog(hostName, text); err != nil {
			slog.Error("Failed to write to operation log", "err", err, "host", hostName)
			return errorFlashMsg{
				text: fmt.Sprintf("Failed to write to `%s` op log: %s", hostName, err),
			}
		}

		return hostLogUpdatedMsg{host}
	}
}

func (m *Model) handleHostLogUpdatedMsg(msg hostLogUpdatedMsg) tea.Cmd {
	if m.selectedHost != msg.host || m.selectedHost.hostTab != hostTabLog {
		return nil
	}

	// Load log entries.
	hostName := msg.host.name
	slog.Debug("Reading op log", "host", hostName)
	entries, err := m.db.ReadHostLogs(hostName)
	if err != nil {
		slog.Error("Failed to read operation log", "err", err, "host", hostName)
		return nil
	}

	if len(entries) == 0 {
		m.contentPanel.SetContent("No operation log entries for " + hostName)
		return nil
	}

	// Render log entries.
	// TODO set size for wrapping
	table := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		Wrap(true)
	for _, e := range entries {
		table.Row(
			subtleStyle.Render(e.Time.Format(time.DateTime)),
			e.Entry,
		)
	}
	m.contentPanel.SetContent(table.String())

	// TODO cache rendered output if no new log entries

	return nil
}
