package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hostListModel is the list of hosts to manage.
type hostListModel struct {
	list       list.Model
	hoverTimer *time.Timer // Triggers host status collection when user hovers.
	prevHost   hostItem    // Used to invalidate timer when the list position changes.
}

type statusHoverMsg struct {
	host hostItem
}

func newHostList(hosts []string) hostListModel {
	items := make([]list.Item, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, hostItem(host))
	}

	hl := list.New(items, newItemDelegate(10), 10, 10)
	hl.Title = "Hosts"

	return hostListModel{list: hl}
}

// Init implements tea.Model.
func (m hostListModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m hostListModel) Update(msg tea.Msg) (hostListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	host := m.Selected()
	if host == nil {
		return m, cmd
	}

	if *host != m.prevHost {
		// Host has changed, reset timer.
		m.prevHost = *host
		if m.hoverTimer != nil {
			m.hoverTimer.Stop()
		}

		m.hoverTimer = time.NewTimer(time.Second)

		// Trigger status update after timeout.
		cmd = tea.Batch(cmd, func() tea.Msg {
			<-m.hoverTimer.C
			return statusHoverMsg{*host}
		})
	}

	return m, cmd
}

// View implements tea.Model.
func (m hostListModel) View() string {
	return m.list.View()
}

// Selected returns the currently selected host, or nil.
func (m hostListModel) Selected() *hostItem {
	selected := m.list.SelectedItem()
	if selected == nil {
		return nil
	}

	host := selected.(hostItem)
	return &host
}

// SetSize controls the size of list rendering.
func (m *hostListModel) SetSize(width, height int) {
	m.list.SetSize(width, height)
	m.list.SetDelegate(newItemDelegate(width))
}

// hostItem represents an entry in the host list.
type hostItem string

func (item hostItem) FilterValue() string { return string(item) }
func (item hostItem) String() string      { return string(item) }

type itemDelegate struct {
	itemStyle         lipgloss.Style
	selectedItemStyle lipgloss.Style
	maxWidth          int
}

func newItemDelegate(maxWidth int) itemDelegate {
	itemStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle := itemStyle.Copy().PaddingLeft(0).Foreground(lipgloss.Color("170"))

	return itemDelegate{
		itemStyle:         itemStyle,
		selectedItemStyle: selectedItemStyle,
		maxWidth:          maxWidth,
	}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render a particular hostList entry.
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(hostItem)
	if !ok {
		return
	}

	fn := d.itemStyle.MaxWidth(d.maxWidth).Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.selectedItemStyle.MaxWidth(d.maxWidth).Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(string(item)))
}
