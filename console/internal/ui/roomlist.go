package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// roomItem adapts a roomEntry to bubbles/list.
type roomItem struct {
	name   string
	unread int
	active bool
}

// FilterValue satisfies list.Item (filtering is disabled, but the method is
// required).
func (r roomItem) FilterValue() string { return r.name }

// roomDelegate renders one room row: an active marker, the room name, and an
// unread badge when there are unseen messages.
type roomDelegate struct{}

func (roomDelegate) Height() int                         { return 1 }
func (roomDelegate) Spacing() int                        { return 0 }
func (roomDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (roomDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(roomItem)
	if !ok {
		return
	}
	width := m.Width()
	if width < 1 {
		width = 1
	}

	marker := "  "
	nameStyle := styleRoomNormal
	if index == m.Index() {
		marker = "▶ "
		nameStyle = styleRoomActive
	}

	badge := ""
	if it.unread > 0 {
		badge = styleUnreadBadge.Render(fmt.Sprintf("%d", it.unread))
	}

	// Reserve room for the marker and badge, then truncate the name.
	badgeW := lipgloss.Width(badge)
	nameW := width - lipgloss.Width(marker) - badgeW - 1
	if nameW < 1 {
		nameW = 1
	}
	name := nameStyle.Render(truncate(it.name, nameW))

	gap := width - lipgloss.Width(marker) - lipgloss.Width(name) - badgeW
	if gap < 1 {
		gap = 1
	}
	_, _ = fmt.Fprint(w, marker+name+strings.Repeat(" ", gap)+badge)
}

// syncRoomItems pushes the current room slice into the bubbles list, preserving
// the active selection.
func (m *Model) syncRoomItems() {
	items := make([]list.Item, len(m.rooms))
	for i, r := range m.rooms {
		items[i] = roomItem{name: r.name, unread: r.unread, active: i == m.active}
	}
	m.roomList.SetItems(items)
	if m.active >= 0 && m.active < len(m.rooms) {
		m.roomList.Select(m.active)
	}
}

// leftSplit divides the left pane's content height between the room list and
// the presence panel (each including its one-line header).
func (m Model) leftSplit() (roomLines, presenceLines int) {
	rows := len(m.presenceRows())
	presenceLines = rows + 1 // header
	if presenceLines > m.paneContentH/2 {
		presenceLines = m.paneContentH / 2
	}
	if presenceLines < 1 {
		presenceLines = 1
	}
	roomLines = m.paneContentH - presenceLines - 1 // rooms header
	if roomLines < 1 {
		roomLines = 1
	}
	return roomLines, presenceLines
}

// roomListHeight is the number of list rows available given the current split.
func (m Model) roomListHeight() int {
	rl, _ := m.leftSplit()
	return rl
}

// renderRoomList draws the ROOMS section header and the list body.
func (m Model) renderRoomList() string {
	h := m.roomListHeight()
	header := styleSectionHeader.Render("ROOMS")

	rl := m.roomList
	rl.SetSize(m.leftContentW, h)
	body := rl.View()
	if len(m.rooms) == 0 {
		body = stylePresenceIdle.Render("(no rooms yet)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
