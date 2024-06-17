package runner

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	Styles struct {
		StatusSuffix lipgloss.Style
	}

	prog   string
	args   []string
	dest   string
	cmd    *exec.Cmd
	state  int
	err    error
	closed bool // No more writes accepted when true.

	output   *buffer              // Permanent output buffer.
	onUpdate func(*Model) tea.Msg // Construct msg when there is new output.
	notify   chan struct{}        // Pinged when data is written to buffer.
	cancel   func()
}

// NewLocal constructs a runner for a local command.
func NewLocal(ctx context.Context, onUpdate func(*Model) tea.Msg, dir string, prog string, args ...string) *Model {
	ctx, cancel := context.WithCancel(ctx)

	r := newRunner(onUpdate, prog, args...)
	r.cmd = exec.CommandContext(ctx, prog, args...)
	r.cmd.Dir = dir
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output
	r.cancel = cancel
	r.dest = "local"

	slog.Debug("Local runner created", "prog", prog, "args", args)

	return r
}

func NewRemote(
	ctx context.Context, onUpdate func(*Model) tea.Msg, host string, user string,
	prog string, args ...string,
) *Model {
	ctx, cancel := context.WithCancel(ctx)

	dest := "ssh://"
	if user != "" {
		dest += user + "@"
	}
	dest += host

	sshArgs := []string{"-T", "-oBatchMode=yes", dest, prog}
	sshArgs = append(sshArgs, args...)

	r := newRunner(onUpdate, prog, args...)
	r.cmd = exec.CommandContext(ctx, "ssh", sshArgs...)
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output
	r.cancel = cancel
	r.dest = dest

	slog.Debug("Remote runner created", "prog", prog, "args", args, "dest", dest)

	return r
}

func NewRemoteScript(
	ctx context.Context, onUpdate func(*Model) tea.Msg, host string, user string,
	name string, script string,
) *Model {
	ctx, cancel := context.WithCancel(ctx)

	dest := "ssh://"
	if user != "" {
		dest += user + "@"
	}
	dest += host

	r := newRunner(onUpdate, name)
	r.cmd = exec.CommandContext(ctx, "ssh", "-T", "-oBatchMode=yes", dest, "bash", "-s")
	r.cmd.Stdin = strings.NewReader(script)
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output
	r.cancel = cancel
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

	r.Styles.StatusSuffix = lipgloss.NewStyle()

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
		if r.Complete() {
			r.Lock()
			defer r.Unlock()
			if r.closed {
				return nil
			}

			// Render status text and stop waiting for output.
			r.closed = true
			s := r.Styles.StatusSuffix.Render("\n[" + stateToString(r.state) + "]")
			_, _ = r.output.Write([]byte(s))

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

// PassEnv copies a parent environment variable for use by the child process.
func (r *Model) PassEnv(name string) {
	value := os.Getenv(name)
	r.SetEnv(name, value)
}

// SetEnv appends an environment variable definition.  Due to the way `exec.Cmd` works, the first
// call to this effectively stops the parent environment from being passed to the child process.
func (r *Model) SetEnv(name string, value string) {
	r.Lock()
	defer r.Unlock()
	r.cmd.Env = append(r.cmd.Env, name+"="+value)
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

// WriteTo writes buffer contents to the provided writer.
func (r *Model) CopyTo(w io.Writer) (int64, error) {
	r.output.RLock()
	defer r.output.RUnlock()

	br := bytes.NewReader(r.output.buf)
	return io.Copy(w, br)
}

// Cancel running process.
func (r *Model) Cancel() {
	if r.cancel != nil {
		r.cancel()

		r.Lock()
		defer r.Unlock()
		_, _ = r.output.Write([]byte("\n[Interrupt]\n"))
	}
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
