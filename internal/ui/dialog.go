package ui

import (
	"image/color"

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
