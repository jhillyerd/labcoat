package ui

import (
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
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
	if host == nil {
		slog.Error("hostRunCommand given nil host arg (bug)")
		return nil
	}

	m.setVisibleHostTab(hostTabRunCmd)

	if host.runCmd.runner != nil && host.runCmd.runner.Running() {
		slog.Info("host cmd already running", "host", host.name)
		return nil
	}

	onUpdate := func(r *runner.Model) tea.Msg {
		return hostRunCommandOutputMsg{host: host, final: r.Complete()}
	}

	srunner := runner.NewRemote(onUpdate, host.target.DeployHost, host.target.DeployUser,
		msg.prog, msg.args...)
	host.runCmd.runner = srunner

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()+" @ "+srunner.Destination()) + "\n"
	host.runCmd.intro = intro
	host.runCmd.contentPanel.SetContent(intro)

	return srunner.Init()
}

func (m *Model) handleHostRunCommandOutputMsg(msg hostRunCommandOutputMsg) tea.Cmd {
	host := msg.host
	if host.runCmd.runner == nil {
		slog.Error("Received hostCmdOutput for host with no runner (bug)", "host", host.name)
		return nil
	}

	srunner := host.runCmd.runner
	_, cmd := srunner.Update(nil)

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
	return cmd
}
