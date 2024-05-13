package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hostListModel is the list of hosts to manage.
type hostListModel struct {
	list     list.Model
	prevItem list.Item // Used to detect when selected host changes for hover.
}

func newHostList(hosts []string) hostListModel {
	items := make([]list.Item, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, hostItem(host))
	}

	hl := list.New(items, newItemDelegate(10), 10, 10)
	hl.Title = "Hosts"
	hl.DisableQuitKeybindings()
	hl.SetShowHelp(false)

	hl.Styles.TitleBar.Padding(0)
	hl.Styles.StatusBar.Padding(0, 0, 1, 0)

	return hostListModel{list: hl}
}

// Init implements tea.Model.
func (m hostListModel) Init() tea.Cmd {
	return m.handleHostChange()
}

// Update implements tea.Model.
func (m hostListModel) Update(msg tea.Msg) (hostListModel, tea.Cmd) {
	var cmd tea.Cmd

	m.list, cmd = m.list.Update(msg)

	return m, tea.Batch(cmd, m.handleHostChange())
}

func (m *hostListModel) handleHostChange() tea.Cmd {
	selected := m.list.SelectedItem()
	if selected != nil && selected != m.prevItem {
		m.prevItem = selected
		host := string(selected.(hostItem))

		return func() tea.Msg {
			return hostChangedMsg{hostName: host}
		}
	}

	return nil
}

// View implements tea.Model.
func (m hostListModel) View() string {
	return m.list.View()
}

// SetSize controls the size of list rendering.
func (m *hostListModel) SetSize(width, height int) {
	m.list.SetSize(width, height)
	m.list.SetDelegate(newItemDelegate(width))
	m.list.Styles.StatusBar.Width(width)
}

// FilterState of the embedded list.
func (m *hostListModel) FilterState() list.FilterState {
	return m.list.FilterState()
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
	itemStyle := lipgloss.NewStyle().PaddingLeft(1)
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
			return d.selectedItemStyle.MaxWidth(d.maxWidth).Render("»" + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(string(item)))
}
