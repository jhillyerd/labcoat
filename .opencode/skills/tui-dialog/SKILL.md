---
name: tui-dialog
description: >
  Specialized guidance for implementing dialog and overlay systems in Bubbletea v2
  applications, including modal patterns, command palettes, and compositor-based
  overlay rendering.
---

# TUI Dialog System Expert

Specialized guidance for implementing dialog and overlay systems in Bubbletea v2 applications.

## When to Use This Skill

Activate when:
- Implementing modal dialogs
- Creating overlay systems
- Building confirmation dialogs
- Implementing command palettes
- Managing dialog state
- Handling modal input routing

## Core Modal Pattern (Bubble Tea v2)

Bubble Tea v2 uses **lipgloss compositor** for proper modal overlay rendering.

### Three Essential Methods

Every modal MUST implement:

```go
type Modal struct {
    visible bool
    // ... modal state
}

// 1. View() - Renders modal content (no positioning)
func (m *Modal) View() string {
    if !m.visible {
        return ""
    }

    // Build modal content with styles
    style := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#7dcfff")).
        Padding(1, 2)

    return style.Render(content)
}

// 2. Update() - Handles input, returns result
func (m *Modal) Update(msg tea.Msg) (*Result, tea.Cmd) {
    if !m.visible {
        return nil, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            m.visible = false
            return &Result{Cancelled: true}, nil
        case "enter":
            m.visible = false
            return &Result{Confirmed: true, Value: m.value}, nil
        }
    }

    return nil, nil
}

// 3. Overlay() - Layers modal on top using compositor
func (m *Modal) Overlay(background string, width, height int) string {
    if !m.visible {
        return background
    }

    modal := m.View()

    // Create layers
    bgLayer := lipgloss.NewLayer(background)
    modalLayer := lipgloss.NewLayer(modal)

    // Center the modal
    modalW := lipgloss.Width(modal)
    modalH := lipgloss.Height(modal)
    centerX := (width - modalW) / 2
    centerY := (height - modalH) / 4  // Upper quarter

    // Set position and z-index (CRITICAL)
    modalLayer.X(centerX).Y(centerY).Z(1)

    // Compose layers
    compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
    return compositor.Render()
}
```

### Critical Input Routing Pattern

**MUST BE FIRST** in parent Update() to prevent input bleed-through:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Update modal too
        m.modal.Update(msg)
        return m, nil

    case tea.KeyMsg:
        // CRITICAL: Modal check MUST be first
        if m.modal.IsVisible() {
            result, cmd := m.modal.Update(msg)
            if result != nil {
                // Handle modal result
                m.modal = nil
                return m, m.handleModalResult(result)
            }
            // Return early - prevents input reaching main view
            return m, cmd
        }

        // Normal view handling only if modal not active
        switch msg.String() {
        case "ctrl+p":
            m.modal.Show()
            return m, m.modal.Init()
        // ... other handlers
        }
    }

    return m, nil
}
```

### View Integration

```go
func (m Model) View() tea.View {
    // Render main content
    content := m.renderMainView()

    // Overlay modal if visible
    if m.modal != nil && m.modal.IsVisible() {
        content = m.modal.Overlay(content, m.width, m.height)
    }

    v := tea.NewView(content)
    v.AltScreen = true
    return v
}
```

## Modal Types

### 1. Command Palette

```go
type CommandPalette struct {
    commands        []Action
    filteredResults []Action
    input           textinput.Model
    cursor          int
    visible         bool
}

func NewCommandPalette(commands []Action) *CommandPalette {
    input := textinput.New()
    input.Placeholder = "Search commands..."
    input.Prompt = "> "

    return &CommandPalette{
        commands:        commands,
        filteredResults: commands,
        input:           input,
        visible:         false,
    }
}

func (cp *CommandPalette) Show() tea.Cmd {
    cp.visible = true
    cp.input.SetValue("")
    cp.filteredResults = cp.commands
    cp.cursor = 0
    cp.input.Focus()
    return textinput.Blink
}

func (cp *CommandPalette) IsVisible() bool {
    return cp.visible
}

func (cp *CommandPalette) Update(msg tea.Msg) (*Action, tea.Cmd) {
    if !cp.visible {
        return nil, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            cp.visible = false
            return nil, nil

        case "enter":
            if len(cp.filteredResults) > 0 {
                selected := &cp.filteredResults[cp.cursor]
                cp.visible = false
                return selected, nil
            }

        case "up", "ctrl+k":
            if cp.cursor > 0 {
                cp.cursor--
            }

        case "down", "ctrl+j":
            if cp.cursor < len(cp.filteredResults)-1 {
                cp.cursor++
            }

        default:
            // Update input and filter
            var cmd tea.Cmd
            cp.input, cmd = cp.input.Update(msg)
            cp.filterCommands()
            return nil, cmd
        }
    }

    return nil, nil
}

func (cp *CommandPalette) View() string {
    if !cp.visible {
        return ""
    }

    style := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#7dcfff")).
        Padding(1, 2).
        Width(80)

    var b strings.Builder
    b.WriteString("Command Palette\n\n")
    b.WriteString(cp.input.View() + "\n\n")

    // Render filtered results
    for i, cmd := range cp.filteredResults {
        if i >= 10 {
            break
        }

        cursor := "  "
        if i == cp.cursor {
            cursor = "> "
        }

        b.WriteString(cursor + cmd.Name + "\n")
    }

    b.WriteString("\n↑↓: navigate • enter: execute • esc: close")

    return style.Render(b.String())
}

func (cp *CommandPalette) Overlay(background string, width, height int) string {
    if !cp.visible {
        return background
    }

    modal := cp.View()

    bgLayer := lipgloss.NewLayer(background)
    modalLayer := lipgloss.NewLayer(modal)

    modalW := lipgloss.Width(modal)
    modalH := lipgloss.Height(modal)
    centerX := (width - modalW) / 2
    centerY := (height - modalH) / 4

    modalLayer.X(centerX).Y(centerY).Z(1)

    compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
    return compositor.Render()
}
```

### 2. Confirm Dialog

```go
type ConfirmDialog struct {
    title    string
    message  string
    visible  bool
    focused  int  // 0=cancel, 1=confirm
}

func (d *ConfirmDialog) Update(msg tea.Msg) (*bool, tea.Cmd) {
    if !d.visible {
        return nil, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            d.visible = false
            cancelled := false
            return &cancelled, nil

        case "left", "right", "tab":
            d.focused = (d.focused + 1) % 2

        case "enter":
            d.visible = false
            confirmed := d.focused == 1
            return &confirmed, nil
        }
    }

    return nil, nil
}

func (d *ConfirmDialog) View() string {
    if !d.visible {
        return ""
    }

    cancelBtn := "Cancel"
    confirmBtn := "Confirm"

    if d.focused == 0 {
        cancelBtn = "[" + cancelBtn + "]"
    } else {
        confirmBtn = "[" + confirmBtn + "]"
    }

    buttons := lipgloss.JoinHorizontal(
        lipgloss.Center,
        cancelBtn, "  ", confirmBtn,
    )

    content := lipgloss.JoinVertical(
        lipgloss.Left,
        d.title,
        "",
        d.message,
        "",
        buttons,
    )

    style := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1, 2)

    return style.Render(content)
}

func (d *ConfirmDialog) Overlay(background string, width, height int) string {
    if !d.visible {
        return background
    }

    modal := d.View()

    bgLayer := lipgloss.NewLayer(background)
    modalLayer := lipgloss.NewLayer(modal)

    modalW := lipgloss.Width(modal)
    modalH := lipgloss.Height(modal)
    centerX := (width - modalW) / 2
    centerY := (height - modalH) / 2

    modalLayer.X(centerX).Y(centerY).Z(1)

    compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
    return compositor.Render()
}
```

### 3. Input Dialog

```go
type InputDialog struct {
    title   string
    input   textinput.Model
    visible bool
}

func (d *InputDialog) Update(msg tea.Msg) (*string, tea.Cmd) {
    if !d.visible {
        return nil, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            d.visible = false
            return nil, nil

        case "enter":
            value := d.input.Value()
            d.visible = false
            return &value, nil
        }
    }

    var cmd tea.Cmd
    d.input, cmd = d.input.Update(msg)
    return nil, cmd
}

func (d *InputDialog) View() string {
    if !d.visible {
        return ""
    }

    content := lipgloss.JoinVertical(
        lipgloss.Left,
        d.title,
        "",
        d.input.View(),
        "",
        "enter: confirm • esc: cancel",
    )

    style := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1, 2)

    return style.Render(content)
}

func (d *InputDialog) Overlay(background string, width, height int) string {
    if !d.visible {
        return background
    }

    modal := d.View()

    bgLayer := lipgloss.NewLayer(background)
    modalLayer := lipgloss.NewLayer(modal)

    modalW := lipgloss.Width(modal)
    modalH := lipgloss.Height(modal)
    centerX := (width - modalW) / 2
    centerY := (height - modalH) / 3

    modalLayer.X(centerX).Y(centerY).Z(1)

    compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
    return compositor.Render()
}
```

## Critical Rules

### 1. Input Routing

**ALWAYS** check modal visibility FIRST in Update():

```go
case tea.KeyMsg:
    // MUST BE FIRST
    if m.modal.IsVisible() {
        result, cmd := m.modal.Update(msg)
        if result != nil {
            m.modal = nil
            // handle result
        }
        return m, cmd  // EARLY RETURN
    }

    // main view handling...
```

### 2. Z-Index Required

Always set Z-index on modal layer:

```go
modalLayer.X(x).Y(y).Z(1)  // Z(1) puts modal on top
```

### 3. Compositor Layers

Use compositor for true overlay (background remains visible):

```go
bgLayer := lipgloss.NewLayer(background)
modalLayer := lipgloss.NewLayer(modal)
compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
return compositor.Render()
```

### 4. Clear Modal State

Set modal to nil when done:

```go
if result != nil {
    m.modal = nil  // Clear pointer
    return m, m.handleResult(result)
}
```

### 5. Window Size Updates

Pass window size to modal:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.modal.Update(msg)  // Modal needs dimensions
```

## Best Practices

1. **Separate View() and Overlay()** - View renders content, Overlay handles positioning
2. **Use early returns** - Prevent input bleed-through to main view
3. **Center by default** - Most modals should be centered
4. **Show help text** - Display available keybindings
5. **Focus management** - Focus input when modal opens, blur when closes
6. **Validate before closing** - Check input validity on enter
7. **ESC always cancels** - Universal close binding
8. **Return results, not booleans** - More flexible for caller

## Common Mistakes

### ❌ Wrong: Manual positioning in View()

```go
func (m *Modal) View() string {
    // DON'T DO THIS
    topPadding := strings.Repeat("\n", y)
    leftPadding := strings.Repeat(" ", x)
    return topPadding + leftPadding + content
}
```

### ✅ Right: Use Overlay() with compositor

```go
func (m *Modal) Overlay(background string, width, height int) string {
    modal := m.View()
    bgLayer := lipgloss.NewLayer(background)
    modalLayer := lipgloss.NewLayer(modal)
    modalLayer.X(centerX).Y(centerY).Z(1)
    compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
    return compositor.Render()
}
```

### ❌ Wrong: Modal check after other handlers

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        }

        // TOO LATE - 'q' already handled above
        if m.modal.IsVisible() {
            // ...
        }
    }
}
```

### ✅ Right: Modal check first

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // FIRST - prevents 'q' from reaching main view
        if m.modal.IsVisible() {
            result, cmd := m.modal.Update(msg)
            return m, cmd  // Early return
        }

        switch msg.String() {
        case "q":
            return m, tea.Quit
        }
    }
}
```

## Testing Modals

```go
func TestModalInputRouting(t *testing.T) {
    m := Model{modal: NewModal()}
    m.modal.Show()

    // Modal should intercept keys
    m, _ = m.Update(tea.KeyMsg{String: "q"})

    // Modal should still be visible (not quit)
    if !m.modal.IsVisible() {
        t.Error("modal should intercept 'q' key")
    }
}
```

## Reference Implementation

See `internal/tui/dialog.go` and `internal/tui/flow.go` in the FDT CLI project for complete working examples.
