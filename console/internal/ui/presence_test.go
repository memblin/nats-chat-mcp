package ui

import (
	"strings"
	"testing"
	"time"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

func pres(name, lastSeen string, rooms ...string) natsclient.Presence {
	return natsclient.Presence{Name: name, Rooms: rooms, LastSeen: lastSeen}
}

func presID(id, name, lastSeen string, rooms ...string) natsclient.Presence {
	p := pres(name, lastSeen, rooms...)
	p.ID = id
	return p
}

func names(ps []natsclient.Presence) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

// A session that restarts mints a fresh id, so the dead record lingers under the
// same display name until its TTL. The panel collapses those to one row.
func TestDedupePresenceKeepsFreshestPerName(t *testing.T) {
	in := []natsclient.Presence{
		pres("Chris", "2026-06-13T14:00:00.000Z"), // stale ghost
		pres("Chris", "2026-06-13T14:30:00.000Z"), // live restart — newer
		pres("Dev-Lead", "2026-06-13T14:25:00.000Z"),
	}
	got := dedupePresence(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows after dedup, got %d (%v)", len(got), names(got))
	}
	var chris natsclient.Presence
	for _, p := range got {
		if p.Name == "Chris" {
			chris = p
		}
	}
	if chris.LastSeen != "2026-06-13T14:30:00.000Z" {
		t.Errorf("kept stale Chris record %q, want the freshest", chris.LastSeen)
	}
}

func TestDedupePresenceSortedByName(t *testing.T) {
	in := []natsclient.Presence{
		pres("Zara", "2026-06-13T14:00:00.000Z"),
		pres("Amy", "2026-06-13T14:00:00.000Z"),
	}
	got := names(dedupePresence(in))
	want := []string{"Amy", "Zara"}
	if !eq(got, want) {
		t.Errorf("dedup order = %v, want sorted %v", got, want)
	}
}

func TestDedupePresenceDistinctNamesPreserved(t *testing.T) {
	in := []natsclient.Presence{
		pres("Amy", "2026-06-13T14:00:00.000Z"),
		pres("Bob", "2026-06-13T14:00:00.000Z"),
		pres("Cy", "2026-06-13T14:00:00.000Z"),
	}
	if got := dedupePresence(in); len(got) != 3 {
		t.Errorf("distinct names should all survive, got %d", len(got))
	}
}

func TestDedupePresenceEmpty(t *testing.T) {
	if got := dedupePresence(nil); len(got) != 0 {
		t.Errorf("nil input should yield empty, got %d", len(got))
	}
}

// staleParticipants fixtures share a fixed "now" so idle ages are exact.
var staleNow = time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)

func ago(d time.Duration) string { return staleNow.Add(-d).Format(time.RFC3339) }

func TestStaleParticipantsActiveRoom(t *testing.T) {
	ps := []natsclient.Presence{
		presID("self", "Me", ago(10*time.Minute), "planning"),     // excluded: self
		presID("g1", "Dev-Lead", ago(7*time.Minute), "planning"),  // stale, in room
		presID("l1", "UAT-Lead", ago(30*time.Second), "planning"), // fresh
		presID("g2", "Zed", ago(10*time.Minute), "other"),         // stale, wrong room
	}
	got := staleParticipants(ps, staleNow, "planning", "self")
	if !eq(names(got), []string{"Dev-Lead"}) {
		t.Errorf("active-room stale = %v, want [Dev-Lead]", names(got))
	}
}

func TestStaleParticipantsBulkEverywhere(t *testing.T) {
	ps := []natsclient.Presence{
		presID("self", "Me", ago(10*time.Minute), "planning"),
		presID("g1", "Dev-Lead", ago(7*time.Minute), "planning"),
		presID("l1", "UAT-Lead", ago(30*time.Second), "planning"),
		presID("g2", "Zed", ago(10*time.Minute), "other"),
	}
	got := staleParticipants(ps, staleNow, "", "self")
	if !eq(names(got), []string{"Dev-Lead", "Zed"}) {
		t.Errorf("bulk stale = %v, want [Dev-Lead Zed]", names(got))
	}
}

func TestStaleParticipantsThresholdBoundary(t *testing.T) {
	ps := []natsclient.Presence{
		presID("u", "Under", ago(170*time.Second)), // <= 3m: not stale
		presID("o", "Over", ago(190*time.Second)),  // >  3m: stale
	}
	got := staleParticipants(ps, staleNow, "", "")
	if !eq(names(got), []string{"Over"}) {
		t.Errorf("threshold stale = %v, want [Over]", names(got))
	}
}

// Even when a live and a dead record share a display name, only the stale id is
// selected — eviction works on raw records, not the deduped view.
func TestStaleParticipantsCatchesGhostBehindSharedName(t *testing.T) {
	ps := []natsclient.Presence{
		presID("ghost", "Dev-Lead", ago(8*time.Minute), "planning"),
		presID("live", "Dev-Lead", ago(20*time.Second), "planning"),
	}
	got := staleParticipants(ps, staleNow, "planning", "")
	if len(got) != 1 || got[0].ID != "ghost" {
		t.Errorf("expected only the ghost id, got %+v", got)
	}
}

func TestRenderConfirmModalListsTargets(t *testing.T) {
	m := Model{
		width: 70, midH: 20, now: staleNow,
		confirm: &confirmState{
			scope:   "planning",
			targets: []natsclient.Presence{presID("g1", "Dev-Lead", ago(7*time.Minute), "planning")},
		},
	}
	out := m.renderConfirmModal()
	for _, want := range []string{"Evict", "planning", "Dev-Lead", "No messages are deleted"} {
		if !strings.Contains(out, want) {
			t.Errorf("modal missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderConfirmModalEmpty(t *testing.T) {
	m := Model{
		width: 70, midH: 20, now: staleNow,
		confirm: &confirmState{scope: "", targets: nil},
	}
	out := m.renderConfirmModal()
	if !strings.Contains(out, "No stale participants") {
		t.Errorf("empty modal should say there is nothing to evict:\n%s", out)
	}
}
