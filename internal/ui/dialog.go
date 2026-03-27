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
	input      textinput.Model
	title      string
	visible    bool
	submitFn   func(string) tea.Cmd
	history    *[]string
	historyIdx int
	tempValue  string
}

func NewInputOverlay(title string, submitFn func(string) tea.Cmd, history *[]string) *InputOverlay {
	ti := textinput.New()
	ti.Focus()
	return &InputOverlay{
		input:      ti,
		title:      title,
		visible:    true,
		submitFn:   submitFn,
		history:    history,
		historyIdx: -1,
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
		case "up":
			if o.history != nil && len(*o.history) > 0 {
				if o.historyIdx == -1 {
					o.tempValue = o.input.Value()
					o.historyIdx = len(*o.history) - 1
				} else if o.historyIdx > 0 {
					o.historyIdx--
				}
				o.input.SetValue((*o.history)[o.historyIdx])
				o.input.CursorEnd()
			}
			return nil
		case "down":
			if o.history != nil && o.historyIdx != -1 {
				if o.historyIdx < len(*o.history)-1 {
					o.historyIdx++
					o.input.SetValue((*o.history)[o.historyIdx])
				} else {
					o.historyIdx = -1
					o.input.SetValue(o.tempValue)
				}
				o.input.CursorEnd()
			}
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

	hint := "enter: run • esc: cancel"
	if o.history != nil && len(*o.history) > 0 {
		hint = "enter: run • ↑↓: history • esc: cancel"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		o.title,
		"",
		o.input.View(),
		"",
		subtleStyle.Render(hint),
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
