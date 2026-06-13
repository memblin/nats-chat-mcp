package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// onKey routes a key press: global shortcuts first, then by search mode and
// focus zone.
func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+]":
		cmd := m.nextRoom()
		return m, cmd
	case "ctrl+[":
		cmd := m.prevRoom()
		return m, cmd
	case "tab":
		cmd := m.cycleFocus(1)
		return m, cmd
	case "shift+tab":
		cmd := m.cycleFocus(-1)
		return m, cmd
	}

	if m.searching {
		return m.onSearchKey(msg)
	}
	switch m.focus {
	case zoneCompose:
		return m.onComposeKey(msg)
	case zoneFeed:
		return m.onFeedKey(msg)
	case zoneRooms:
		return m.onRoomsKey(msg)
	}
	return m, nil
}

// cycleFocus advances the focused zone (Rooms → Feed → Compose → …), managing
// the compose input's focus state.
func (m *Model) cycleFocus(dir int) tea.Cmd {
	return m.setFocus(focusZone((int(m.focus) + dir + 3) % 3))
}

// setFocus moves focus to zone, focusing the text input only when composing.
func (m *Model) setFocus(zone focusZone) tea.Cmd {
	m.focus = zone
	if zone == zoneCompose && !m.searching {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

// onComposeKey handles keys while the input bar is focused.
func (m Model) onComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.input.SetValue("")
		return m, nil
	case tea.KeyEnter:
		content := strings.TrimSpace(m.input.Value())
		room := m.activeName()
		m.input.SetValue("")
		if content != "" && room != "" && room != directBucket {
			return m, m.sendCmd(room, content)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// onFeedKey handles keys while the message feed is focused.
func (m Model) onFeedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.viewport.ScrollDown(1)
	case "k", "up":
		m.viewport.ScrollUp(1)
	case "pgdown":
		m.viewport.PageDown()
	case "pgup":
		m.viewport.PageUp()
	case "G":
		m.viewport.GotoBottom()
	case "g":
		m.viewport.GotoTop()
	case "s":
		m.newestBottom = !m.newestBottom
		m.refreshViewport()
	case "/":
		m.searching = true
		m.searchQuery = ""
		m.input.Blur()
		m.refreshViewport()
	case "enter":
		return m, m.setFocus(zoneCompose)
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// onRoomsKey handles keys while the room list is focused.
func (m Model) onRoomsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "k", "up":
		cmd := m.activate(clamp(m.roomList.Index()-1, 0, len(m.rooms)-1))
		return m, cmd
	case "j", "down":
		cmd := m.activate(clamp(m.roomList.Index()+1, 0, len(m.rooms)-1))
		return m, cmd
	case "enter":
		return m, m.setFocus(zoneFeed)
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// onSearchKey handles keys while the "/" search filter is being typed.
func (m Model) onSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel: drop the filter entirely.
		m.searching = false
		m.searchQuery = ""
		m.refreshViewport()
		return m, nil
	case tea.KeyEnter:
		// Commit: keep the filter, leave search mode so the feed is scrollable.
		m.searching = false
		m.refreshViewport()
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		r := []rune(m.searchQuery)
		if len(r) > 0 {
			m.searchQuery = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.searchQuery += " "
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
	}
	m.refreshViewport()
	return m, nil
}

// clamp constrains v to [lo, hi]; if the range is empty (hi < lo) it returns lo.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
