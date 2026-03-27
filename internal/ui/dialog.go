package ui

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Dialog is a reusable modal dialog that overlays content on top of a background.
type Dialog struct {
	content     string
	borderColor color.Color
}

// NewDialog creates a new Dialog with the given content and border color.
func NewDialog(content string, borderColor color.Color) *Dialog {
	return &Dialog{
		content:     content,
		borderColor: borderColor,
	}
}

// View renders the Dialog content with styling.
func (d *Dialog) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.borderColor).
		Padding(1, 2)

	return style.Render(d.content)
}

// Overlay renders the Dialog centered on top of the background content using
// the lipgloss compositor for proper layering.
func (d *Dialog) Overlay(background string, width, height int) string {
	dialog := d.View()

	bgLayer := lipgloss.NewLayer(background)
	dialogLayer := lipgloss.NewLayer(dialog)

	dialogW := lipgloss.Width(dialog)
	dialogH := lipgloss.Height(dialog)
	centerX := (width - dialogW) / 2
	centerY := (height - dialogH) / 2

	dialogLayer.X(centerX).Y(centerY).Z(1)

	compositor := lipgloss.NewCompositor(bgLayer, dialogLayer)
	return compositor.Render()
}

type InputOverlay struct {
	input    textinput.Model
	title    string
	visible  bool
	submitFn func(string) tea.Cmd
}

func NewInputOverlay(title string, submitFn func(string) tea.Cmd) *InputOverlay {
	ti := textinput.New()
	ti.Focus()
	return &InputOverlay{
		input:    ti,
		title:    title,
		visible:  true,
		submitFn: submitFn,
	}
}

func (o *InputOverlay) Value() string {
	return o.input.Value()
}

func (o *InputOverlay) IsVisible() bool {
	return o.visible
}

func (o *InputOverlay) Update(msg tea.Msg) tea.Cmd {
	if !o.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			o.visible = false
			return nil
		case "enter":
			o.visible = false
			return nil
		}
	}

	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return cmd
}

func (o *InputOverlay) View() string {
	if !o.visible {
		return ""
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		o.title,
		"",
		o.input.View(),
		"",
		subtleStyle.Render("enter: run • esc: cancel"),
	)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7dcfff")).
		Padding(1, 2)

	return style.Render(content)
}

func (o *InputOverlay) Overlay(background string, width, height int) string {
	if !o.visible {
		return background
	}

	overlay := o.View()

	bgLayer := lipgloss.NewLayer(background)
	overlayLayer := lipgloss.NewLayer(overlay)

	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)
	centerX := (width - overlayW) / 2
	centerY := (height - overlayH) / 2

	overlayLayer.X(centerX).Y(centerY).Z(1)

	compositor := lipgloss.NewCompositor(bgLayer, overlayLayer)
	return compositor.Render()
}
