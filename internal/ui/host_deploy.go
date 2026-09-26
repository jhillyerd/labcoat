package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhillyerd/labcoat/internal/nix"
	"github.com/jhillyerd/labcoat/internal/runner"
	"github.com/jhillyerd/labcoat/internal/store"
)

type hostDeployMsg struct {
	host   *hostModel
	action string // nixos-rebuild action: "switch" or "boot".
}

// hostOutPathMsg delivers the system outPath resolved for one specific
// deploy, keyed by that deploy's generation so resolutions arriving after a
// newer deploy started are discarded.
type hostOutPathMsg struct {
	host    *hostModel
	gen     int
	outPath string
}

// Sent when the runner has new output/status to display.  runner and gen
// identify the deployment that emitted the update, so completion handling
// can discard finals superseded by a newer deploy.
type hostDeployOutputMsg struct {
	host   *hostModel
	runner *runner.Model
	gen    int
	final  bool
}

func (m *Model) hostDeployCmd(host *hostModel, action string) tea.Cmd {
	return func() tea.Msg {
		return hostDeployMsg{
			host:   host,
			action: action,
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

	host.deploy.deployGen++ // Invalidate in-flight resolutions and completions.
	deployGen := host.deploy.deployGen

	onUpdate := func(r *runner.Model) tea.Msg {
		return hostDeployOutputMsg{host: host, runner: r, gen: deployGen, final: r.Closed()}
	}

	// Construct nixos-rebuild command line.
	targetHost := host.target.DeployUser + "@" + host.target.DeployHost
	args := []string{"--flake", ".#" + host.name, "--target-host", targetHost}
	if m.config.Nix.DefaultBuildHost != "" {
		args = append(args, "--build-host", m.config.Nix.DefaultBuildHost)
	}
	args = append(args, msg.action)

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
	host.deploy.outPath = ""                     // Resolved per deploy, binds the post-deploy GC root.

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
	outPathCmd := m.hostOutPathCmd(host, host.deploy.deployGen)
	return tea.Batch(srunner.Init(m.program), logCmd, busyCmd, outPathCmd)
}

// hostOutPathCmd resolves the system outPath for host at deploy start,
// concurrently with nixos-rebuild, so the GC root registered after a
// successful deploy pins exactly the closure that deploy built instead of a
// fresh evaluation of the (mutable) flake path, which could have drifted if
// the flake changed during the build.  Non-fatal on failure: GC root
// registration then falls back to evaluating at registration time.
func (m *Model) hostOutPathCmd(host *hostModel, gen int) tea.Cmd {
	return func() tea.Msg {
		const outPathTimeout = 60 * time.Second

		ctx, done := context.WithTimeout(context.Background(), outPathTimeout)
		defer done()

		worker, err := m.nixPool.Get(ctx)
		if err != nil {
			slog.Warn("Failed to get nix worker for outPath resolution", "host", host.name, "err", err)
			return nil
		}
		defer worker.Done()

		outPath, err := nix.SystemOutPath(ctx, nix.OutPathRequest{
			FlakePath: m.flakePath,
			HostName:  host.name,
		})
		if err != nil {
			slog.Warn("Failed to resolve system outPath for deploy",
				"host", host.name, "worker", worker, "err", err)
			return nil
		}

		return hostOutPathMsg{host: host, gen: gen, outPath: outPath}
	}
}

func (m *Model) handleHostOutPathMsg(msg hostOutPathMsg) tea.Cmd {
	// A newer deploy may have started since this resolution began; only the
	// current generation may populate the field its GC root will read.
	if msg.gen != msg.host.deploy.deployGen {
		slog.Debug("Discarding outPath from superseded deploy", "host", msg.host.name)
		return nil
	}
	msg.host.deploy.outPath = msg.outPath
	return nil
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

		// Register the deployed closure as a GC root so local garbage
		// collection cannot delete it.  Non-fatal.  Passing the outPath
		// captured at deploy start (if resolved by now) pins the root to the
		// closure this deploy built, not to a fresh evaluation.
		if success {
			// A newer deploy may have started before this completion was
			// processed, replacing the runner and resetting outPath; only the
			// current deployment may register a root (it does so on its own
			// completion).  Still-current completions keep the empty-outPath
			// fallback inside hostGCRootCmd.
			if msg.runner != host.deploy.runner || msg.gen != host.deploy.deployGen {
				slog.Debug("Skipping GC root for superseded deploy completion", "host", host.name)
			} else {
				cmds = append(cmds, m.hostGCRootCmd(host, host.deploy.outPath))
			}
		}
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

// gcRootResultMsg signals the outcome of GC root registration.  Carried as
// text (not an error) because rooting failure must never fail a deploy.
type gcRootResultMsg struct {
	host *hostModel
	text string
}

// hostGCRootCmd registers host's deployed system closure as a GC root on the
// control machine (issue #17).  nixos-rebuild does not pass --add-root, so
// without this a local `nix-store --gc` can delete the running closure and
// force a rebuild.  One root per host; the previous root is replaced.
//
// outPath is the system outPath captured when the deploy started, binding
// the root to the closure that deploy built even if the flake has since
// changed; when empty (capture unfinished or failed) the flake is evaluated
// here instead, accepting the drift window that implies.
func (m *Model) hostGCRootCmd(host *hostModel, outPath string) tea.Cmd {
	return func() tea.Msg {
		const gcRootTimeout = 60 * time.Second

		ctx, done := context.WithTimeout(context.Background(), gcRootTimeout)
		defer done()

		worker, err := m.nixPool.Get(ctx)
		if err != nil {
			slog.Warn("Failed to get nix worker for GC root", "host", host.name, "err", err)
			return nil
		}
		defer worker.Done()

		if outPath == "" {
			// No captured outPath (very fast build, or resolution
			// failed); evaluate the current flake state as a fallback.
			outPath, err = nix.SystemOutPath(ctx, nix.OutPathRequest{
				FlakePath: m.flakePath,
				HostName:  host.name,
			})
			if err != nil {
				slog.Warn("Failed to resolve system outPath for GC root", "host", host.name, "err", err)
				return gcRootResultMsg{
					host: host,
					text: fmt.Sprintf("GC root not registered: %s", err),
				}
			}
		}

		if err := nix.RegisterRoot(ctx, nix.RootsDir(m.config), host.name, outPath); err != nil {
			slog.Warn("Failed to register GC root", "host", host.name,
				"path", outPath, "worker", worker, "err", err)
			return gcRootResultMsg{
				host: host,
				text: fmt.Sprintf("GC root not registered: %s", err),
			}
		}

		slog.Info("Registered GC root", "host", host.name, "path", outPath, "worker", worker)
		return gcRootResultMsg{
			host: host,
			text: fmt.Sprintf("GC root registered: %s", outPath),
		}
	}
}

func (m *Model) handleGCRootResultMsg(msg gcRootResultMsg) tea.Cmd {
	return m.hostLogCmd(msg.host, msg.text)
}
