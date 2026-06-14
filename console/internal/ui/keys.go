package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// onKey routes a key press: global shortcuts first, then by search mode and
// focus zone.
func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits, even mid-search or mid-confirm.
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// A confirmation modal is fully modal: it captures every key (y confirms,
	// anything else cancels) so nothing underneath can react.
	if m.confirm != nil {
		return m.onConfirmKey(msg)
	}

	// The "new DM" picker is likewise modal while open.
	if m.picker != nil {
		return m.onPickerKey(msg)
	}

	// Search mode is self-contained: every other key edits/commits the query so
	// a stray Tab or "/" can't silently change panes underneath it.
	if m.searching {
		return m.onSearchKey(msg)
	}

	switch msg.String() {
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
		// Clear a draft, or — when already empty — hop to the feed where search
		// and scrolling live. This gives a one-key path out of "typing mode".
		if m.input.Value() == "" {
			return m, m.setFocus(zoneFeed)
		}
		m.input.SetValue("")
		return m, nil
	case tea.KeyEnter:
		content := strings.TrimSpace(m.input.Value())
		m.input.SetValue("")
		if content == "" {
			return m, nil
		}
		if e := m.activeEntry(); e != nil {
			if e.isDM {
				return m, m.sendDMCmd(e.peerID, content)
			}
			return m, m.sendCmd(e.name, content)
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
	case "m":
		return m, m.toggleMouse()
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
		return m, m.moveSelection(-1)
	case "j", "down":
		return m, m.moveSelection(1)
	case "enter":
		return m, m.setFocus(zoneFeed)
	case "m":
		return m, m.toggleMouse()
	case "d":
		// Open the "new DM" picker, seeded from the live presence list.
		return m.startNewDM()
	case "c":
		// Close the active DM thread (purging its NATS messages).
		return m.startCloseDM()
	case "x":
		// Evict stale participants in the active room.
		return m.startEvict(m.activeName())
	case "X":
		// Evict stale participants everywhere.
		return m.startEvict("")
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// startEvict opens the eviction confirmation for the stale participants in room
// (or everywhere when room is ""). The modal opens even when none are stale, so
// the key never feels dead — it then just reports there is nothing to do.
func (m Model) startEvict(room string) (tea.Model, tea.Cmd) {
	targets := staleParticipants(m.presence, m.now, room, m.self.ID)
	m.confirm = &confirmState{kind: confirmEvict, scope: room, targets: targets}
	return m, nil
}

// startCloseDM opens the close confirmation for the active DM thread. It is a
// no-op when the active entry is not a DM, so "c" is inert outside a thread.
func (m Model) startCloseDM() (tea.Model, tea.Cmd) {
	e := m.activeEntry()
	if e == nil || !e.isDM {
		return m, nil
	}
	m.confirm = &confirmState{
		kind:     confirmCloseDM,
		peerID:   e.peerID,
		peerName: e.label,
		msgCount: len(m.feeds[e.name]),
	}
	return m, nil
}

// startNewDM opens the agent picker, seeded from the live presence list minus
// this console's own identity. With nobody else present the modal still opens and
// reports there is nobody to message.
func (m Model) startNewDM() (tea.Model, tea.Cmd) {
	var agents []natsclient.Presence
	for _, p := range dedupePresence(m.presence) {
		if p.ID != m.self.ID {
			agents = append(agents, p)
		}
	}
	m.picker = &pickerState{agents: agents}
	return m, nil
}

// onConfirmKey handles keys while a destructive-action modal is open: y/Y
// confirms, every other key (including Esc) cancels.
func (m Model) onConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	m.confirm = nil
	s := msg.String()
	confirmed := s == "y" || s == "Y"
	if !confirmed {
		return m, nil
	}
	switch c.kind {
	case confirmEvict:
		if len(c.targets) > 0 {
			return m, m.evictCmd(c.targets)
		}
	case confirmCloseDM:
		return m, m.purgeDMCmd(c.peerID)
	}
	return m, nil
}

// onPickerKey handles keys while the "new DM" picker is open: up/down (j/k) move
// the highlight, Enter opens the chosen peer's thread, any other key cancels.
func (m Model) onPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.picker
	switch msg.String() {
	case "k", "up":
		if p.idx > 0 {
			p.idx--
		}
		return m, nil
	case "j", "down":
		if p.idx < len(p.agents)-1 {
			p.idx++
		}
		return m, nil
	case "enter":
		m.picker = nil
		if p.idx < 0 || p.idx >= len(p.agents) {
			return m, nil
		}
		peer := p.agents[p.idx]
		m.ensureDM(peer.ID, peer.Name)
		cmd := m.activate(m.roomIndex(dmKey(peer.ID)))
		return m, tea.Batch(cmd, m.setFocus(zoneCompose))
	default:
		m.picker = nil
		return m, nil
	}
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
