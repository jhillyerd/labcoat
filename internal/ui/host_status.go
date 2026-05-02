package ui

import (
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhillyerd/labcoat/internal/runner"
)

type hostStatusMsg struct {
	hostName string
	final    bool // Indicates last status update message.
}

func (m *Model) hostStatusCmd(host *hostModel) tea.Cmd {
	if ok, cmd := requireHostTarget("hostStatusCmd", host); !ok {
		return cmd
	}

	m.setVisibleHostTab(hostTabStatus)

	// Run SSH pre-flight check if not yet verified.
	if !host.sshChecked {
		return m.hostSSHCheckCmd(host)
	}

	// If SSH check failed, don't attempt status (user must press `s` to retry).
	if !host.sshReachable {
		slog.Debug("hostStatusCmd: SSH not reachable, skipping", "host", host.name)
		return nil
	}

	// Do nothing if status job is already running.
	srunner := m.selectedHost.status.runner
	if srunner != nil && srunner.Running() {
		slog.Debug("hostStatusCmd already running", "host", host.name)
		return nil
	}

	onUpdate := func(r *runner.Model) tea.Msg {
		// Sent when the runner has new output to display.
		return hostStatusMsg{hostName: host.name, final: r.Closed()}
	}

	script := runner.NewScript(m.config.Commands.StatusCmds)
	srunner = runner.NewRemoteScript(m.ctx, onUpdate,
		host.target.DeployHost, host.target.DeployUser, "host status (script)", script)
	srunner.Styles.StatusSuffix = subtleStyle

	host.status.runner = srunner

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()+" @ "+srunner.Destination()) + "\n"
	host.status.intro = intro
	host.status.contentPanel.SetContent(intro)

	return srunner.Init(m.program)
}

func (m *Model) handleHostStatusMsg(msg hostStatusMsg) tea.Cmd {
	host := m.hosts[msg.hostName]

	if host.status.runner == nil {
		slog.Error("Received hostStatusMsg for host with no runner (bug)", "host", msg.hostName)
		return nil
	}

	srunner := host.status.runner
	_, cmd := srunner.Update(nil)

	if msg.final {
		host.status.collected = srunner.Successful()
	}

	// Render and cache status content.
	status := host.status.intro
	status += runner.FormatOutput(
		srunner.View(),
		func(s string) string { return labelStyle.Render(s) })

	// Carriage returns cause formatting issues.
	status = strings.ReplaceAll(status, "\r", "")

	// Truncate content width to preserve correct viewport line counts & scrolling.
	// Viewport bug: https://github.com/charmbracelet/bubbles/issues/479
	// TODO configurable line wrapping?
	status = lipgloss.NewStyle().MaxWidth(m.sizes.contentPanel.width).Render(status)

	host.status.contentPanel.SetContent(status)

	return cmd
}
