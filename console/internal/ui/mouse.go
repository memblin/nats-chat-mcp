package ui

import tea "github.com/charmbracelet/bubbletea"

// entryAtClick maps a click's Y to the global entry index it lands on, or -1.
// Inside the bordered left box: y=1 is the top border, y=2 the "ROOMS" header,
// y=3 the first room row; the room list body is roomListHeight rows, then (when
// any DM thread exists) the "DIRECT MESSAGES" header and the DM rows. Visual rows
// are grouped rooms-first, so they map onto navOrder.
func (m Model) entryAtClick(y int) int {
	order := m.navOrder()
	roomLines := m.roomListHeight()
	if r := y - 3; r >= 0 && r < roomLines && r < m.roomCount() {
		return order[r]
	}
	if m.dmCount() > 0 {
		base := 3 + roomLines + 1 // +1 for the DIRECT MESSAGES header
		if d := y - base; d >= 0 && d < m.dmCount() {
			return order[m.roomCount()+d]
		}
	}
	return -1
}

// onMouse handles mouse input: the wheel scrolls the feed from anywhere, and a
// left click focuses the pane under the cursor (selecting a room when clicked in
// the room list, or the compose bar when clicked on the input line).
func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.viewport.ScrollUp(3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.viewport.ScrollDown(3)
		return m, nil
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// A deliberate click cancels an in-progress search and drops the filter.
	if m.searching {
		m.searching = false
		m.searchQuery = ""
		m.refreshViewport()
	}

	// Vertical layout: status (y=0), panes (y in [1,midH]), help (midH+1),
	// input (height-1).
	switch {
	case msg.Y == m.height-1:
		return m, m.setFocus(zoneCompose)

	case msg.Y >= 1 && msg.Y <= m.midH:
		if msg.X < m.leftW {
			cmd := m.setFocus(zoneRooms)
			if gi := m.entryAtClick(msg.Y); gi >= 0 {
				cmd = tea.Batch(cmd, m.activate(gi))
			}
			return m, cmd
		}
		return m, m.setFocus(zoneFeed)
	}
	return m, nil
}
