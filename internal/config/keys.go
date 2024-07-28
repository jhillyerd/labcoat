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
	Jump       key.Binding
	Pager      key.Binding

	// Commands.
	Deploy           key.Binding
	Help             key.Binding
	Reboot           key.Binding
	RunCommandPrompt key.Binding
	SSHInto          key.Binding
	Status           key.Binding
	Quit             key.Binding
}

// FullHelp displays a full-screen list of all key bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.ScrollUp, k.ScrollDown, k.Jump, k.Filter},
		{k.Status, k.Deploy, k.SSHInto, k.RunCommandPrompt, k.Reboot},
		{k.Pager, k.Quit, k.Help},
	}
}

// ShortHelp displays one line of key bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.NextTab,
		k.Status, k.Deploy, k.SSHInto, k.RunCommandPrompt, k.Reboot,
		k.Help,
	}
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
	Jump: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "jump to letter"),
	),
	Pager: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "open pager"),
	),

	Deploy: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "deploy"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Reboot: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reboot"),
	),
	RunCommandPrompt: key.NewBinding(
		key.WithKeys("!"),
		key.WithHelp("!", "run cmd"),
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
