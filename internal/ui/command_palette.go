package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Command struct {
	Name        string
	Description string
	Execute     func(m *Model) tea.Cmd
}

type CommandPalette struct {
	commands []Command
	filtered []Command
	input    textinput.Model
	cursor   int
	visible  bool
}

func NewCommandPalette(commands []Command) *CommandPalette {
	input := textinput.New()
	input.Prompt = ": "

	return &CommandPalette{
		commands: commands,
		filtered: commands,
		input:    input,
	}
}

func (cp *CommandPalette) Show() tea.Cmd {
	cp.visible = true
	cp.input.SetValue("")
	cp.filtered = cp.commands
	cp.cursor = 0
	cp.input.Focus()
	return textinput.Blink
}

func (cp *CommandPalette) IsVisible() bool {
	return cp.visible
}

func (cp *CommandPalette) Update(msg tea.Msg) (*Command, tea.Cmd) {
	if !cp.visible {
		return nil, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			cp.visible = false
			return nil, nil

		case "enter":
			if len(cp.filtered) > 0 {
				selected := &cp.filtered[cp.cursor]
				cp.visible = false
				return selected, nil
			}
			return nil, nil

		case "up", "ctrl+k":
			if cp.cursor > 0 {
				cp.cursor--
			}
			return nil, nil

		case "down", "ctrl+j":
			if cp.cursor < len(cp.filtered)-1 {
				cp.cursor++
			}
			return nil, nil
		}
	}

	var cmd tea.Cmd
	cp.input, cmd = cp.input.Update(msg)
	cp.filterCommands()
	return nil, cmd
}

func (cp *CommandPalette) filterCommands() {
	query := strings.ToLower(cp.input.Value())
	if query == "" {
		cp.filtered = cp.commands
		return
	}

	var filtered []Command
	for _, cmd := range cp.commands {
		if strings.Contains(strings.ToLower(cmd.Name), query) {
			filtered = append(filtered, cmd)
		}
	}
	cp.filtered = filtered
	if cp.cursor >= len(cp.filtered) {
		cp.cursor = max(0, len(cp.filtered)-1)
	}
}

func (cp *CommandPalette) View() string {
	if !cp.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(labelStyle.Render("Command Palette"))
	b.WriteString("\n")
	b.WriteString(cp.input.View())
	b.WriteString("\n\n")

	for i, cmd := range cp.filtered {
		prefix := "  "
		if i == cp.cursor {
			prefix = "▸ "
		}

		name := prefix + cmd.Name
		if cmd.Description != "" {
			name += subtleStyle.Render(" — " + cmd.Description)
		}
		b.WriteString(name + "\n")
	}

	if len(cp.filtered) == 0 {
		b.WriteString(subtleStyle.Render("No matching commands"))
	}

	b.WriteString("\n" + subtleStyle.Render("↑↓: navigate • enter: execute • esc: cancel"))

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7dcfff")).
		Padding(1, 2).
		Width(60)

	return style.Render(b.String())
}

func (cp *CommandPalette) Overlay(background string, width, height int) string {
	if !cp.visible {
		return background
	}

	palette := cp.View()

	bgLayer := lipgloss.NewLayer(background)
	paletteLayer := lipgloss.NewLayer(palette)

	paletteW := lipgloss.Width(palette)
	paletteH := lipgloss.Height(palette)
	centerX := (width - paletteW) / 2
	centerY := (height - paletteH) / 4

	paletteLayer.X(centerX).Y(centerY).Z(1)

	compositor := lipgloss.NewCompositor(bgLayer, paletteLayer)
	return compositor.Render()
}
