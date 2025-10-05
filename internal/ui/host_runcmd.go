package ui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labcoat/internal/runner"
)

type hostRunCommandMsg struct {
	host *hostModel
	prog string
	args []string
}

// Sent when the runner has new output/status to display.
type hostRunCommandOutputMsg struct {
	host  *hostModel
	final bool
}

func (m *Model) hostRunCommandCmd(host *hostModel, prog string, args ...string) tea.Cmd {
	return func() tea.Msg {
		return hostRunCommandMsg{
			host: host,
			prog: prog,
			args: args,
		}
	}
}

func (m *Model) handleHostRunCommandMsg(msg hostRunCommandMsg) tea.Cmd {
	host := msg.host
	if ok, cmd := requireHostTarget("hostRunCommand", host); !ok {
		return cmd
	}

	m.setVisibleHostTab(hostTabRunCmd)

	if host.runCmd.runner != nil && host.runCmd.runner.Running() {
		slog.Info("host cmd already running", "host", host.name)
		return nil
	}

	onUpdate := func(r *runner.Model) tea.Msg {
		return hostRunCommandOutputMsg{host: host, final: r.Closed()}
	}

	srunner := runner.NewRemote(m.ctx, onUpdate,
		host.target.DeployHost, host.target.DeployUser,
		msg.prog, msg.args...)
	srunner.Styles.StatusSuffix = subtleStyle
	host.runCmd.runner = srunner

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()+" @ "+srunner.Destination()) + "\n"
	host.runCmd.intro = intro
	host.runCmd.contentPanel.SetContent(intro)

	busyCmd := hostListIncrBusyCmd(host.name)
	return tea.Batch(srunner.Init(), busyCmd)
}

func (m *Model) handleHostRunCommandOutputMsg(msg hostRunCommandOutputMsg) tea.Cmd {
	var cmds []tea.Cmd

	host := msg.host
	srunner := host.runCmd.runner
	if srunner == nil {
		slog.Error("Received hostCmdOutput for host with no runner (bug)", "host", host.name)
		return nil
	}

	if msg.final {
		// Log completion.
		logCmd := m.hostLogCmd(msg.host, fmt.Sprintf("Ran `%s`: %s", srunner, srunner.StateString()))

		status := hostItemStatusSuccess
		if !srunner.Successful() {
			status = hostItemStatusFailed
		}
		busyCmd := hostListDecrBusyCmd(host.name, status)

		cmds = append(cmds, logCmd, busyCmd)
	} else {
		// Schedule next update.
		_, cmd := srunner.Update(nil)
		cmds = append(cmds, cmd)
	}

	// Render and cache output content.
	output := host.runCmd.intro
	output += runner.FormatOutput(
		srunner.View(),
		func(s string) string { return labelStyle.Render(s) })

	// Carriage returns cause formatting issues.
	output = strings.ReplaceAll(output, "\r", "")

	// Truncate content width to preserve correct viewport line counts & scrolling.
	// Viewport bug: https://github.com/charmbracelet/bubbles/issues/479
	// TODO configurable line wrapping?
	output = lipgloss.NewStyle().MaxWidth(m.sizes.contentPanel.width).Render(output)

	host.runCmd.contentPanel.SetContent(output)
	return tea.Batch(cmds...)
}
