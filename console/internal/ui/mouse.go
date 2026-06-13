package ui

import tea "github.com/charmbracelet/bubbletea"

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
			// Inside the bordered box: y=1 is the top border, y=2 the "ROOMS"
			// header, y=3 the first room row.
			if roomIdx := msg.Y - 3; roomIdx >= 0 && roomIdx < len(m.rooms) {
				cmd = tea.Batch(cmd, m.activate(roomIdx))
			}
			return m, cmd
		}
		return m, m.setFocus(zoneFeed)
	}
	return m, nil
}
