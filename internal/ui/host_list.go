package ui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hostListModel is the list of hosts to manage.
type hostListModel struct {
	list     list.Model
	prevItem list.Item // Used to detect when selected host changes for hover.
	spinner  *spinner.Model
}

type jumpToLetterMsg string

func newHostList(hosts []string) hostListModel {
	items := make([]list.Item, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, hostItem{
			name:      host,
			busyCount: 0,
		})
	}

	// Shared spinner for all list items, as the itemDelegate is not visible/writable from its
	// own Update() method.
	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = spinnerStyle

	hl := list.New(items, newItemDelegate(&spin, 10), 10, 10)
	hl.Title = "Hosts"
	hl.DisableQuitKeybindings()
	hl.SetShowHelp(false)

	hl.Styles.TitleBar.Padding(0)
	hl.Styles.StatusBar.Padding(0, 0, 1, 0)

	return hostListModel{
		list:    hl,
		spinner: &spin,
	}
}

// Init implements tea.Model.
func (m hostListModel) Init() tea.Cmd {
	// TODO causes dup hostChangedMsgs due to `m` being read-only in Init.
	return tea.Batch(m.handleHostChange(), m.spinner.Tick)
}

// Update implements tea.Model.
func (m hostListModel) Update(msg tea.Msg) (hostListModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case hostListIncrBusyMsg:
		cmd = m.handleHostListBusyMsg(msg.hostName, 1, hostItemStatusEmpty)
		cmds = append(cmds, cmd)
	case hostListDecrBusyMsg:
		cmd = m.handleHostListBusyMsg(msg.hostName, -1, msg.status)
		cmds = append(cmds, cmd)
	case jumpToLetterMsg:
		cmd = m.handleJumpToLetterMsg(msg)
		cmds = append(cmds, cmd)
	case spinner.TickMsg:
		*m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	default:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	cmds = append(cmds, m.handleHostChange())

	return m, tea.Batch(cmds...)
}

func (m *hostListModel) handleHostListBusyMsg(hostName string, delta int, status int) tea.Cmd {
	for i, h := range m.list.Items() {
		if h.(hostItem).name == hostName {
			item := h.(hostItem)
			item.busyCount = max(0, item.busyCount+delta)
			item.status = status
			slog.Debug("hostListBusyMsg update", "host", hostName, "busyCount", item.busyCount, "status", status)

			m.list.SetItem(i, item)
			return nil
		}
	}

	return nil
}

func (m *hostListModel) handleJumpToLetterMsg(msg jumpToLetterMsg) tea.Cmd {
	letter := string(msg)

	for i, h := range m.list.Items() {
		if strings.HasPrefix(h.FilterValue(), letter) {
			m.list.Select(i)
			break
		}
	}

	return nil
}

func (m *hostListModel) handleHostChange() tea.Cmd {
	selected := m.list.SelectedItem()

	// Comparing to previous item is not 100% reliable, this just cuts down on noise.
	if selected != nil && selected != m.prevItem {
		m.prevItem = selected

		host, ok := selected.(hostItem)
		if !ok {
			slog.Error("Selected item is not a hostItem (bug)")
			return nil
		}

		return func() tea.Msg {
			return hostChangedMsg{hostName: host.name}
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
	m.list.SetDelegate(newItemDelegate(m.spinner, width))
	m.list.Styles.StatusBar.Width(width)
}

// FilterState of the embedded list.
func (m *hostListModel) FilterState() list.FilterState {
	return m.list.FilterState()
}

// hostItem represents an entry in the host list.
type hostItem struct {
	name      string
	busyCount int // Number of active jobs for this host.
	status    int // Status of most recent job for this host.
}

const (
	hostItemStatusEmpty = iota
	hostItemStatusSuccess
	hostItemStatusFailed
)

func (item hostItem) FilterValue() string { return string(item.name) }
func (item hostItem) String() string      { return string(item.name) }

type itemDelegate struct {
	spinner           *spinner.Model // Shared spinner for all items, updates handled by hostListModel.
	itemStyle         lipgloss.Style
	selectedItemStyle lipgloss.Style
	maxWidth          int
}

func newItemDelegate(spinner *spinner.Model, maxWidth int) itemDelegate {
	itemStyle := lipgloss.NewStyle()
	selectedItemStyle := itemStyle.Foreground(highlightColor)

	return itemDelegate{
		spinner:           spinner,
		itemStyle:         itemStyle,
		selectedItemStyle: selectedItemStyle,
		maxWidth:          maxWidth,
	}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

var (
	renderedStatusSuccess = lipgloss.NewStyle().Foreground(successColor).Render("✓")
	renderedStatusFailed  = lipgloss.NewStyle().Foreground(failureColor).Render("✗")
)

// Render a particular hostList entry.
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(hostItem)
	if !ok {
		slog.Error("Rendered listItem is not a hostItem (bug)")
		return
	}

	// Setup list decorations.
	style := d.itemStyle

	status := " "
	if item.busyCount > 0 {
		status = d.spinner.View()
	} else {
		switch item.status {
		case hostItemStatusSuccess:
			status = renderedStatusSuccess
		case hostItemStatusFailed:
			status = renderedStatusFailed
		}
	}

	selected := " "
	if index == m.Index() {
		style = d.selectedItemStyle
		selected = "»"
	}

	line := status + style.Render(selected+item.name)
	fmt.Fprint(w, d.itemStyle.MaxWidth(d.maxWidth).Render(line))
}

type hostListIncrBusyMsg struct {
	hostName string
}

func hostListIncrBusyCmd(hostName string) tea.Cmd {
	return func() tea.Msg {
		return hostListIncrBusyMsg{hostName: hostName}
	}
}

type hostListDecrBusyMsg struct {
	hostName string
	status   int // hostItemStatus
}

func hostListDecrBusyCmd(hostName string, status int) tea.Cmd {
	return func() tea.Msg {
		return hostListDecrBusyMsg{hostName: hostName, status: status}
	}
}
