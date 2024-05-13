package main

import (
	"encoding/json"
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
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
	defer lf.Close()
	slog.SetLogLoggerLevel(slog.LevelDebug)

	slog.Info("### STARTUP ###################################################################")

	// Load host list.
	output, nerr := nix.RunNames(nix.NamesData{FlakePath: "/home/james/devel/homelab/nixos"})
	if nerr != nil {
		fmt.Println(nerr.Detail())
		os.Exit(1)
	}

	var hosts []string
	if err := json.Unmarshal(output, &hosts); err != nil {
		fmt.Println("Failed to parse host list:", err)
		fmt.Println("\nJSON input:")
		fmt.Println(string(output))
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(DefaultKeyMap, hosts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
