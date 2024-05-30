package config

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	// Navigation.
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	NextTab    key.Binding
	Filter     key.Binding

	// Commands.
	Deploy  key.Binding
	Reboot  key.Binding
	SSHInto key.Binding
	Status  key.Binding
	Quit    key.Binding
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Filter, k.Status, k.Quit}}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Filter, k.Status, k.Deploy, k.SSHInto, k.Reboot, k.Quit}
}

var DefaultKeyMap = KeyMap{
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
	NextTab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("Tab", "next tab"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter hosts"),
	),

	Deploy: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "deploy"),
	),
	Reboot: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reboot"),
	),
	SSHInto: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "ssh into"),
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
