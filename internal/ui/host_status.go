package ui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
)

type hostStatusMsg struct {
	hostName string
}

func (m *Model) hostStatusCmd(host *hostModel) tea.Cmd {
	if host.target == nil {
		slog.Error("hostStatusCmd called with nil target", "host", host.name)
		return nil
	}

	// Do nothing if status job is already running.
	srunner := m.selectedHost.status.runner
	if srunner != nil && srunner.Running() {
		return nil
	}

	onUpdate := func(r *runner.Model) tea.Msg {
		// Sent when the runner has new output to display.
		return hostStatusMsg{hostName: host.name}
	}

	script := runner.NewScript(m.config.Commands.StatusCmds)
	srunner = runner.NewRemoteScript(
		onUpdate, host.target.DeployHost, host.target.DeployUser, "host status (script)", script)

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
		func(s string) string { return labelStyle.Render(s) })
	host.status.rendered = status

	if m.selectedHost == host {
		// User is currently viewing the host receiving this status, update the panel content.
		m.contentPanel.SetContent(status)
	}

	return cmd
}
