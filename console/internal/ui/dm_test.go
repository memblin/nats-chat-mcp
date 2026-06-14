package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// dmTestModel builds a Model with the components onMessage / compose / picker
// touch initialised, but no NATS client — every assertion below exercises pure
// in-memory state transitions, never a command's NATS side effect.
func dmTestModel() Model {
	return Model{
		active:   -1,
		feeds:    make(map[string][]natsclient.Message),
		loaded:   make(map[string]bool),
		roomList: newEntryList(),
		dmList:   newEntryList(),
		viewport: viewport.New(80, 24),
		input:    textinput.New(),
		self:     natsclient.Identity{ID: "hself", Name: "me"},
	}
}

// dmEvent is an inbound direct message (empty Room) from a given sender.
func dmEvent(fromID, from, content string) natsclient.MessageEvent {
	return natsclient.MessageEvent{Msg: natsclient.Message{
		FromID:    fromID,
		From:      from,
		Content:   content,
		Timestamp: "2026-06-14T10:00:00.000Z",
	}}
}

// Direct messages from different senders land in separate per-sender threads,
// each with its own feed and unread count.
func TestDirectMessagesGroupedBySender(t *testing.T) {
	m := dmTestModel()
	for _, ev := range []natsclient.MessageEvent{
		dmEvent("a1", "alice", "hi"),
		dmEvent("b2", "bob", "yo"),
		dmEvent("a1", "alice", "again"),
	} {
		mi, _ := m.onMessage(ev)
		m = mi.(Model)
	}

	if m.dmCount() != 2 {
		t.Fatalf("want 2 DM threads, got %d", m.dmCount())
	}
	if got := len(m.feeds[dmKey("a1")]); got != 2 {
		t.Errorf("alice thread should have 2 messages, got %d", got)
	}
	if got := len(m.feeds[dmKey("b2")]); got != 1 {
		t.Errorf("bob thread should have 1 message, got %d", got)
	}
	// Nothing is active, so every arrival bumps the thread's unread badge.
	if i := m.roomIndex(dmKey("a1")); m.rooms[i].unread != 2 {
		t.Errorf("alice unread = %d, want 2", m.rooms[i].unread)
	}
	if i := m.roomIndex(dmKey("a1")); m.rooms[i].label != "alice" {
		t.Errorf("alice thread label = %q, want display name", m.rooms[i].label)
	}
}

// A DM thread keyed by a peer id never collides with a room of the same name.
func TestDMKeyDoesNotCollideWithRoom(t *testing.T) {
	m := dmTestModel()
	m.ensureRoom("alice")                                 // a room literally named "alice"
	mi, _ := m.onMessage(dmEvent("alice", "alice", "hi")) // a peer whose id is "alice"
	m = mi.(Model)

	if m.roomCount() != 1 || m.dmCount() != 1 {
		t.Fatalf("room and DM must not collide: roomCount=%d dmCount=%d", m.roomCount(), m.dmCount())
	}
	if len(m.feeds["alice"]) != 0 {
		t.Errorf("room feed should stay empty, got %d", len(m.feeds["alice"]))
	}
	if len(m.feeds[dmKey("alice")]) != 1 {
		t.Errorf("DM feed should hold the message under dmKey, got %d", len(m.feeds[dmKey("alice")]))
	}
}

// Composing Enter in a DM thread returns a send command, clears the input, and a
// dmSentMsg echo is appended locally (our own DM never returns via our consumer).
func TestComposeEnterOnDMSendsAndEchoes(t *testing.T) {
	m := dmTestModel()
	mi, _ := m.onMessage(dmEvent("a1", "alice", "hi"))
	m = mi.(Model)
	m.active = m.roomIndex(dmKey("a1"))
	m.input.SetValue("hello back")

	mi, cmd := m.onComposeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(Model)
	if cmd == nil {
		t.Fatal("composing in a DM thread should return a send command")
	}
	if m.input.Value() != "" {
		t.Errorf("input should clear after send, got %q", m.input.Value())
	}

	sent := natsclient.Message{FromID: m.self.ID, From: m.self.Name, Content: "hello back"}
	mi, _ = m.Update(dmSentMsg{bucket: dmKey("a1"), msg: sent})
	m = mi.(Model)
	feed := m.feeds[dmKey("a1")]
	if len(feed) != 2 || feed[1].Content != "hello back" {
		t.Errorf("local echo not appended to the thread: %+v", feed)
	}
}

// Closing a DM thread removes its entry and feed and keeps the active selection
// pointing at a still-present thread.
func TestCloseDMRemovesThreadAndFixesActive(t *testing.T) {
	m := dmTestModel()
	for _, ev := range []natsclient.MessageEvent{
		dmEvent("a1", "alice", "x"),
		dmEvent("b2", "bob", "y"),
	} {
		mi, _ := m.onMessage(ev)
		m = mi.(Model)
	}
	m.active = m.roomIndex(dmKey("b2")) // bob is active

	mi, _ := m.Update(dmClosedMsg{bucket: dmKey("a1")}) // close alice
	m = mi.(Model)

	if m.roomIndex(dmKey("a1")) != -1 {
		t.Error("alice thread should be removed")
	}
	if _, ok := m.feeds[dmKey("a1")]; ok {
		t.Error("alice feed should be deleted")
	}
	if m.activeName() != dmKey("b2") {
		t.Errorf("active should still be bob's thread, got %q", m.activeName())
	}
}

// Choosing a peer in the picker opens (and activates) that peer's DM thread.
func TestPickerEnterOpensThread(t *testing.T) {
	m := dmTestModel()
	m.picker = &pickerState{
		agents: []natsclient.Presence{{ID: "a1", Name: "alice"}, {ID: "b2", Name: "bob"}},
		idx:    1,
	}

	mi, _ := m.onPickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(Model)

	if m.picker != nil {
		t.Error("picker should close on Enter")
	}
	if m.roomIndex(dmKey("b2")) < 0 {
		t.Fatal("bob's thread should be created")
	}
	if m.activeName() != dmKey("b2") {
		t.Errorf("bob's thread should be active, got %q", m.activeName())
	}
	if e := m.activeEntry(); e == nil || !e.isDM {
		t.Error("active entry should be a DM thread")
	}
}

// navOrder lists rooms first then DMs, regardless of arrival interleaving, so
// section navigation never jumps out of visual order.
func TestNavOrderGroupsRoomsThenDMs(t *testing.T) {
	m := dmTestModel()
	m.ensureRoom("general")
	mi, _ := m.onMessage(dmEvent("a1", "alice", "x"))
	m = mi.(Model)
	m.ensureRoom("random") // a room discovered after a DM already exists

	order := m.navOrder()
	if len(order) != 3 {
		t.Fatalf("want 3 entries, got %d", len(order))
	}
	if m.rooms[order[0]].isDM || m.rooms[order[1]].isDM {
		t.Error("rooms should come before DMs in nav order")
	}
	if !m.rooms[order[2]].isDM {
		t.Error("the DM thread should sort last in nav order")
	}
}
