package runner

import (
	"log/slog"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	stateNotStarted = iota
	stateRunning
	stateDone
	stateFailed
)

// Model runner runs commands and collects their output.
type Model struct {
	sync.RWMutex
	prog  string
	args  []string
	dest  string
	cmd   *exec.Cmd
	state int
	err   error

	output   *buffer              // Permanent output buffer.
	onUpdate func(*Model) tea.Msg // Construct msg when there is new output.
	notify   chan struct{}        // Pinged when data is written to buffer.
}

// NewLocal constructs a runner for a local command.
func NewLocal(onUpdate func(*Model) tea.Msg, prog string, args ...string) *Model {
	r := newRunner(onUpdate, prog, args...)
	r.dest = "local"
	r.cmd = exec.Command(prog, args...)

	slog.Debug("Local runner created", "prog", prog, "args", args)

	return r
}

func NewRemote(
	onUpdate func(*Model) tea.Msg, host string, user string,
	prog string, args ...string,
) *Model {
	dest := "ssh://"
	if user != "" {
		dest += user + "@"
	}
	dest += host

	sshArgs := []string{"-T", "-oBatchMode=yes", dest, prog}
	sshArgs = append(sshArgs, args...)

	r := newRunner(onUpdate, prog, args...)
	r.cmd = exec.Command("ssh", sshArgs...)
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output
	r.dest = dest

	slog.Debug("Remote runner created", "prog", prog, "args", args, "dest", dest)

	return r
}

func NewRemoteScript(
	onUpdate func(*Model) tea.Msg, host string, user string,
	name string, script string,
) *Model {
	dest := "ssh://"
	if user != "" {
		dest += user + "@"
	}
	dest += host

	r := newRunner(onUpdate, name)
	r.cmd = exec.Command("ssh", "-T", "-oBatchMode=yes", dest, "bash", "-s")
	r.cmd.Stdin = strings.NewReader(script)
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output
	r.dest = dest

	slog.Debug("Remote runner created", "script", name, "dest", dest)

	return r
}

// newRunner creates a basic Model, which further requires `cmd` and `dest` to be populated.
func newRunner(onUpdate func(*Model) tea.Msg, prog string, args ...string) *Model {
	r := &Model{
		prog:     prog,
		args:     args,
		onUpdate: onUpdate,
		notify:   make(chan struct{}, 1),
	}

	r.output = newBuffer(func() {
		select {
		case r.notify <- struct{}{}:
		default:
		}
	})

	return r
}

// Running is true if in not-started or running state.
func (r *Model) Running() bool {
	r.RLock()
	defer r.RUnlock()
	return r.state == stateNotStarted || r.state == stateRunning
}

// Complete is true if in done or failed state.
func (r *Model) Complete() bool {
	r.RLock()
	defer r.RUnlock()
	return r.state == stateDone || r.state == stateFailed
}

// Successful is true if done, not failed.
func (r *Model) Successful() bool {
	r.RLock()
	defer r.RUnlock()
	return r.state == stateDone
}

func (r *Model) waitForOutput() tea.Cmd {
	return func() tea.Msg {
		if !r.Running() {
			// Don't wait on completed/failed process output.
			return nil
		}

		<-r.notify
		return r.onUpdate(r)
	}
}

// Init implements tea.Model.
func (r *Model) Init() tea.Cmd {
	cmd := func() tea.Msg {
		r.Lock()
		r.state = stateRunning
		r.Unlock()

		slog.Info("running", "cmd", r, "dest", r.dest)

		r.err = r.cmd.Run()

		r.Lock()
		if r.err == nil {
			r.state = stateDone
		} else {
			r.state = stateFailed
		}
		r.Unlock()

		return r.onUpdate(r)
	}

	return tea.Batch(cmd, r.waitForOutput())
}

// Update implements tea.Model.
func (r *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	return r, r.waitForOutput()
}

// View implements tea.Model.
func (r *Model) View() string {
	r.RLock()
	defer r.RUnlock()

	return r.output.String()
}

// Destination returns the SSH URL or `local`.
func (r *Model) Destination() string {
	return r.dest
}

// String returns the requested command line.
func (r *Model) String() string {
	parts := append([]string{}, r.prog)
	parts = append(parts, r.args...)
	return strings.Join(parts, " ")
}

// StateString returns the current state as a human readable string.
func (r *Model) StateString() string {
	r.RLock()
	defer r.RUnlock()

	return stateToString(r.state)
}

func stateToString(state int) string {
	switch state {
	case stateNotStarted:
		return "Not Started"
	case stateRunning:
		return "Running"
	case stateDone:
		return "Done"
	case stateFailed:
		return "Failed"
	}

	return "Unknown"
}
