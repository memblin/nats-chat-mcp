package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyM() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")} }

// Pressing "m" in the feed zone toggles mouse reporting and emits the command
// that applies the change, so the operator can free the mouse for copy/paste.
func TestMouseToggleInFeedZone(t *testing.T) {
	m := Model{focus: zoneFeed, mouseOn: true}

	model, cmd := m.onKey(keyM())
	off := model.(Model)
	if off.mouseOn {
		t.Error("first press should turn mouse off")
	}
	if cmd == nil {
		t.Error("toggling should return a command (DisableMouse)")
	}

	model, cmd = off.onKey(keyM())
	on := model.(Model)
	if !on.mouseOn {
		t.Error("second press should turn mouse back on")
	}
	if cmd == nil {
		t.Error("toggling should return a command (EnableMouseCellMotion)")
	}
}

// In the rooms zone "m" also toggles (it's not a text-entry zone).
func TestMouseToggleInRoomsZone(t *testing.T) {
	m := Model{focus: zoneRooms, mouseOn: true}
	model, _ := m.onKey(keyM())
	if model.(Model).mouseOn {
		t.Error("rooms-zone m should toggle mouse off")
	}
}
