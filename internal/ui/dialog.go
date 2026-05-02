package ui

import (
	"fmt"
	"image/color"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhillyerd/labcoat/internal/config"
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

// ScrollableDialog is a modal dialog with a scrollable viewport for displaying
// long text content. It supports PgUp/PgDn for scrolling and Esc to dismiss.
type ScrollableDialog struct {
	viewport    viewport.Model
	borderColor color.Color
	keys        config.KeyMap
}

const scrollableDialogPadding = 0
const scrollableDialogBorder = 2 // 1 per side

// NewScrollableDialog creates a scrollable dialog sized to fit within the
// given screen dimensions, displaying the provided content.
func NewScrollableDialog(content string, borderColor color.Color, screenW, screenH int, keys config.KeyMap) *ScrollableDialog {
	// Dialog occupies 80% of screen, max 120 wide, centered.
	w := min(120, screenW*80/100)
	h := min(screenH-4, screenH*80/100)

	d := &ScrollableDialog{
		borderColor: borderColor,
		keys:        keys,
	}
	d.initViewport(content, w, h)
	return d
}

func (d *ScrollableDialog) initViewport(content string, w, h int) {
	// Clamp to minimum usable dimensions.
	w = max(w, 20)
	h = max(h, 6)

	// Inner viewport dimensions minus border and footer.
	vpW := w - 2*scrollableDialogPadding - scrollableDialogBorder
	vpH := h - scrollableDialogBorder - 2 // blank line + footer hint
	vpW = max(vpW, 10)
	vpH = max(vpH, 3)

	vp := viewport.New(viewport.WithWidth(vpW), viewport.WithHeight(vpH))
	vp.KeyMap.PageUp = d.keys.ScrollUp
	vp.KeyMap.PageDown = d.keys.ScrollDown
	vp.SetContent(content)
	vp.GotoTop()
	d.viewport = vp
}

// Resize updates the dialog dimensions to fit new screen size.
func (d *ScrollableDialog) Resize(screenW, screenH int) {
	w := min(120, screenW*80/100)
	h := min(screenH-4, screenH*80/100)

	w = max(w, 20)
	h = max(h, 6)

	vpW := w - 2*scrollableDialogPadding - scrollableDialogBorder
	vpH := h - scrollableDialogBorder - 2
	vpW = max(vpW, 10)
	vpH = max(vpH, 3)

	d.viewport.SetWidth(vpW)
	d.viewport.SetHeight(vpH)
}

// Update handles key events for scrolling. Returns true if the dialog should
// be dismissed.
func (d *ScrollableDialog) Update(msg tea.Msg) (dismiss bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter":
			return true, nil
		}
	}

	d.viewport, cmd = d.viewport.Update(msg)
	return false, cmd
}

// View renders the dialog content with border and hint.
func (d *ScrollableDialog) View() string {
	scroll := "(END)"
	if !d.viewport.AtBottom() {
		scroll = fmt.Sprintf("%.0f%%", d.viewport.ScrollPercent()*100)
	}

	hint := subtleStyle.Render(fmt.Sprintf("PgUp/PgDn: scroll • Esc/Enter: close • %s", scroll))
	body := d.viewport.View() + "\n\n" + hint

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.borderColor).
		Padding(0, 1)

	return style.Render(body)
}

// Overlay renders the dialog centered on top of the background content.
func (d *ScrollableDialog) Overlay(background string, width, height int) string {
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
