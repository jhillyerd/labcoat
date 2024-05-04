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
	cmd   *exec.Cmd
	state int
	err   error

	output   *buffer              // Permanent output buffer.
	onUpdate func(*Model) tea.Msg // Construct msg when there is new output.
	notify   chan struct{}
}

// NewLocal constructs a runner for a local command.
func NewLocal(onUpdate func(*Model) tea.Msg, prog string, args ...string) *Model {
	r := &Model{
		prog:     prog,
		args:     args,
		cmd:      exec.Command(prog, args...),
		onUpdate: onUpdate,
		notify:   make(chan struct{}, 10),
	}

	r.output = newBuffer(func() {
		r.notify <- struct{}{}
	})
	r.cmd.Stdout = r.output
	r.cmd.Stderr = r.output

	return r
}

func (r *Model) waitForOutput() tea.Cmd {
	running := func() bool {
		r.RLock()
		defer r.RUnlock()
		return r.state == stateNotStarted || r.state == stateRunning
	}

	return func() tea.Msg {
		if !running() {
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

		slog.Info("running", "cmd", r)

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
func (r *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return r, r.waitForOutput()
}

// View implements tea.Model.
func (r *Model) View() string {
	r.RLock()
	defer r.RUnlock()

	s := "[" + stateToString(r.state) + "] " + r.String() + "\n\n"

	return s + r.output.String()
}

// String returns the requested command line.
func (r *Model) String() string {
	parts := append([]string{}, r.prog)
	parts = append(parts, r.args...)
	return strings.Join(parts, " ")
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
