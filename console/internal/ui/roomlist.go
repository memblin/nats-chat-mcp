package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// roomItem adapts a roomEntry to bubbles/list. active is set on exactly one item
// across both the room and DM lists, so the active marker is drawn from the item
// (not the list's own cursor) and never lights up a row in the inactive section.
type roomItem struct {
	label  string
	unread int
	active bool
}

// FilterValue satisfies list.Item (filtering is disabled, but the method is
// required).
func (r roomItem) FilterValue() string { return r.label }

// roomDelegate renders one row: an active marker, the label, and an unread badge
// when there are unseen messages.
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
	if it.active {
		marker = "▶ "
		nameStyle = styleRoomActive
	}

	badge := ""
	if it.unread > 0 {
		badge = styleUnreadBadge.Render(fmt.Sprintf("%d", it.unread))
	}

	// Reserve room for the marker and badge, then truncate the label.
	badgeW := lipgloss.Width(badge)
	nameW := width - lipgloss.Width(marker) - badgeW - 1
	if nameW < 1 {
		nameW = 1
	}
	name := nameStyle.Render(truncate(it.label, nameW))

	gap := width - lipgloss.Width(marker) - lipgloss.Width(name) - badgeW
	if gap < 1 {
		gap = 1
	}
	_, _ = fmt.Fprint(w, marker+name+strings.Repeat(" ", gap)+badge)
}

// roomCount / dmCount count the two kinds of entry in the combined slice.
func (m Model) roomCount() int { return len(m.rooms) - m.dmCount() }

func (m Model) dmCount() int {
	n := 0
	for _, r := range m.rooms {
		if r.isDM {
			n++
		}
	}
	return n
}

// syncRoomItems partitions the combined entry slice into the room list and the
// DM list (each in slice order), tagging the single active row so its marker is
// drawn in whichever section owns it.
func (m *Model) syncRoomItems() {
	var roomItems, dmItems []list.Item
	activeRoom, activeDM := -1, -1
	for i, r := range m.rooms {
		it := roomItem{label: r.label, unread: r.unread, active: i == m.active}
		if r.isDM {
			if it.active {
				activeDM = len(dmItems)
			}
			dmItems = append(dmItems, it)
		} else {
			if it.active {
				activeRoom = len(roomItems)
			}
			roomItems = append(roomItems, it)
		}
	}
	m.roomList.SetItems(roomItems)
	m.dmList.SetItems(dmItems)
	if activeRoom >= 0 {
		m.roomList.Select(activeRoom)
	}
	if activeDM >= 0 {
		m.dmList.Select(activeDM)
	}
}

// leftSplit divides the left pane's content height between the room list, the DM
// list, and the presence panel. The DM section collapses to nothing until at
// least one DM thread exists, so the layout is unchanged for rooms-only use.
func (m Model) leftSplit() (roomLines, dmLines, presenceLines int) {
	presenceLines = len(m.presenceRows()) + 1 // header
	if presenceLines > m.paneContentH/2 {
		presenceLines = m.paneContentH / 2
	}
	if presenceLines < 1 {
		presenceLines = 1
	}

	hasDM := m.dmCount() > 0
	headers := 1 // ROOMS
	if hasDM {
		headers = 2 // + DIRECT MESSAGES
	}
	avail := m.paneContentH - presenceLines - leftDividerLines - headers
	if avail < 1 {
		avail = 1
	}
	if hasDM {
		dmLines = m.dmCount()
		if dmLines > avail/2 {
			dmLines = avail / 2
		}
		if dmLines < 1 {
			dmLines = 1
		}
	}
	roomLines = avail - dmLines
	if roomLines < 1 {
		roomLines = 1
	}
	return roomLines, dmLines, presenceLines
}

// roomListHeight / dmListHeight are the list-body row counts from the split.
func (m Model) roomListHeight() int { rl, _, _ := m.leftSplit(); return rl }
func (m Model) dmListHeight() int   { _, dl, _ := m.leftSplit(); return dl }

// renderRoomList draws the ROOMS section header and the list body.
func (m Model) renderRoomList() string {
	header := styleSectionHeader.Render("ROOMS")

	rl := m.roomList
	rl.SetSize(m.leftContentW, m.roomListHeight())
	body := rl.View()
	if m.roomCount() == 0 {
		body = stylePresenceIdle.Render("(no rooms yet)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// renderDMList draws the DIRECT MESSAGES section header and the DM list body.
func (m Model) renderDMList() string {
	header := styleSectionHeader.Render("DIRECT MESSAGES")

	dl := m.dmList
	dl.SetSize(m.leftContentW, m.dmListHeight())
	body := dl.View()
	if m.dmCount() == 0 {
		body = stylePresenceIdle.Render("(no direct messages)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
