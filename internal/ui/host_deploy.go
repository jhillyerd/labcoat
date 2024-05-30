package ui

import (
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhillyerd/labui/internal/runner"
)

type hostDeployMsg struct {
	host *hostModel
}

// Sent when the runner has new output/status to display.
type hostDeployOutputMsg struct {
	host  *hostModel
	final bool
}

func (m *Model) hostDeployCmd(host *hostModel) tea.Cmd {
	return func() tea.Msg {
		return hostDeployMsg{
			host: host,
		}
	}
}

func (m *Model) handleHostDeployMsg(msg hostDeployMsg) tea.Cmd {
	host := msg.host
	if ok, cmd := requireHostTarget("hostDeployMsg", host); !ok {
		return cmd
	}

	m.setVisibleHostTab(hostTabDeploy)

	if host.deploy.runner != nil && host.deploy.runner.Running() {
		slog.Info("host deploy already running", "host", host.name)
		return nil
	}

	onUpdate := func(r *runner.Model) tea.Msg {
		return hostDeployOutputMsg{host: host, final: r.Complete()}
	}

	// Construct nixos-rebuild command line.
	args := []string{
		"--flake",
		".#" + host.name,
		"--target-host",
		host.target.DeployUser + "@" + host.target.DeployHost,
	}
	if m.config.Nix.DefaultBuildHost != "" {
		args = append(args, "--build-host", m.config.Nix.DefaultBuildHost)
	}
	args = append(args, "switch")

	srunner := runner.NewLocal(onUpdate, m.flakePath, "nixos-rebuild", args...)
	host.deploy.runner = srunner

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()) + "\n"
	host.deploy.intro = intro
	host.deploy.contentPanel.SetContent(intro)

	return srunner.Init()
}

func (m *Model) handleHostDeployOutputMsg(msg hostDeployOutputMsg) tea.Cmd {
	host := msg.host
	if host.deploy.runner == nil {
		slog.Error("Received hostDeployOutputMsg for host with no runner (bug)", "host", host.name)
		return nil
	}

	srunner := host.deploy.runner
	_, cmd := srunner.Update(nil)

	// Render and cache output content.
	output := host.deploy.intro
	output += runner.FormatOutput(
		srunner.View(),
		func(s string) string { return labelStyle.Render(s) })

	// Carriage returns cause formatting issues.
	output = strings.ReplaceAll(output, "\r", "")

	// Truncate content width to preserve correct viewport line counts & scrolling.
	// Viewport bug: https://github.com/charmbracelet/bubbles/issues/479
	// TODO configurable line wrapping?
	output = lipgloss.NewStyle().MaxWidth(m.sizes.contentPanel.width).Render(output)

	host.deploy.contentPanel.SetContent(output)
	return cmd
}
