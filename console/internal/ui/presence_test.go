package ui

import (
	"testing"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

func pres(name, lastSeen string, rooms ...string) natsclient.Presence {
	return natsclient.Presence{Name: name, Rooms: rooms, LastSeen: lastSeen}
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
