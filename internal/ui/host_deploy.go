package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhillyerd/labcoat/internal/runner"
	"github.com/jhillyerd/labcoat/internal/store"
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
		return hostDeployOutputMsg{host: host, final: r.Closed()}
	}

	// Construct nixos-rebuild command line.
	targetHost := host.target.DeployUser + "@" + host.target.DeployHost
	args := []string{"--flake", ".#" + host.name, "--target-host", targetHost}
	if m.config.Nix.DefaultBuildHost != "" {
		args = append(args, "--build-host", m.config.Nix.DefaultBuildHost)
	}
	args = append(args, "switch")

	ctx, cancel := context.WithCancel(m.ctx)
	srunner := runner.NewLocal(ctx, onUpdate, m.flakePath, "nixos-rebuild", args...)
	srunner.PassEnv("HOME", "PATH", "SSH_AUTH_SOCK", "SSH_TTY")

	// Prevent nixos-rebuild's internal SSH from prompting for host keys or
	// passwords, which would freeze the TUI. Failures are shown inline.
	srunner.SetEnv("NIX_SSHOPTS", "-oBatchMode=yes")

	srunner.Styles.StatusSuffix = subtleStyle
	host.deploy.runner = srunner
	host.deploy.cancel = cancel
	host.deploy.fingerprint = m.flakeFingerprint // Capture current fingerprint.

	// Init status display.
	intro := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(srunner.String()) + "\n"
	host.deploy.intro = intro
	host.deploy.contentPanel.SetContent(intro)

	logText := fmt.Sprintf("NixOS deployment started, target: %s", targetHost)
	if host.deploy.fingerprint != "" {
		logText += fmt.Sprintf(", fingerprint: %s", shortFingerprint(host.deploy.fingerprint))
	}
	logCmd := m.hostLogCmd(host, logText)
	busyCmd := hostListIncrBusyCmd(host.name)
	return tea.Batch(srunner.Init(m.program), logCmd, busyCmd)
}

func (m *Model) handleHostDeployOutputMsg(msg hostDeployOutputMsg) tea.Cmd {
	var cmds []tea.Cmd

	host := msg.host
	srunner := host.deploy.runner
	if srunner == nil {
		slog.Error("Received hostDeployOutputMsg for host with no runner (bug)", "host", host.name)
		return nil
	}

	if msg.final {
		success := srunner.Successful()

		fp := host.deploy.fingerprint

		// Persist deployment record.
		if fp != "" {
			record := store.DeploymentRecord{
				Timestamp:   time.Now(),
				Fingerprint: fp,
				Success:     success,
			}
			if err := m.db.RecordDeployment(host.name, record); err != nil {
				slog.Error("Failed to record deployment", "host", host.name, "err", err)
			}
		} else {
			slog.Warn("No flake fingerprint available, skipping deployment record", "host", host.name)
		}

		// Log completion.
		logText := fmt.Sprintf("NixOS deployment finished: %s", srunner.StateString())
		if fp != "" {
			logText += fmt.Sprintf(", fingerprint: %s", shortFingerprint(fp))
		}
		logCmd := m.hostLogCmd(
			msg.host,
			logText)

		status := hostItemStatusSuccess
		if !success {
			status = hostItemStatusFailed
		}
		busyCmd := hostListDecrBusyCmd(host.name, status)

		cmds = append(cmds, logCmd, busyCmd, m.fetchAllHostsBehindCmd())
	} else {
		// Schedule next update.
		_, updateCmd := srunner.Update(nil)
		cmds = append(cmds, updateCmd)
	}

	// Render and cache output content.
	panel := &host.deploy.contentPanel
	follow := panel.AtBottom()
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

	panel.SetContent(output)
	if follow {
		panel.GotoBottom()
	}

	return tea.Batch(cmds...)
}
