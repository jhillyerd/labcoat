package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jhillyerd/labui/internal/nix"
	"github.com/jhillyerd/labui/internal/ui"
)

var DefaultKeyMap = ui.KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("→/l", "right"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("PgUp", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("PgDn", "scroll down"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter hosts"),
	),
	Status: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "get status"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
}

func main() {
	// Init logging.
	lf, err := tea.LogToFile("debug.log", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "logging error: ", err)
		os.Exit(1)
	}
	defer lf.Close()

	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.Info("### STARTUP ###################################################################")

	// Load host list.
	flakePath := "/home/james/devel/homelab/nixos"
	hosts, nerr := nix.GetNames(nix.NamesRequest{FlakePath: flakePath})
	if nerr != nil {
		fmt.Fprintln(os.Stderr, nerr.Error())
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(DefaultKeyMap, flakePath, hosts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
