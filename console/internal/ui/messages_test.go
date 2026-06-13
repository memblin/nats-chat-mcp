package ui

import (
	"testing"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

func msg(from, content, ts string) natsclient.Message {
	return natsclient.Message{From: from, Content: content, Timestamp: ts}
}

// three messages deliberately out of chronological order in the slice.
func sample() []natsclient.Message {
	return []natsclient.Message{
		msg("chris", "Starting new sprint", "2026-06-13T14:28:00.000Z"),
		msg("Dev-Lead", "Dispatching PR review", "2026-06-13T14:32:00.000Z"),
		msg("UAT-Lead", "Test suite green", "2026-06-13T14:31:00.000Z"),
	}
}

func contents(msgs []natsclient.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortMessagesNewestBottom(t *testing.T) {
	got := contents(sortMessages(sample(), true))
	want := []string{"Starting new sprint", "Test suite green", "Dispatching PR review"}
	if !eq(got, want) {
		t.Errorf("newest-bottom order = %v, want %v", got, want)
	}
}

func TestSortMessagesNewestTop(t *testing.T) {
	got := contents(sortMessages(sample(), false))
	want := []string{"Dispatching PR review", "Test suite green", "Starting new sprint"}
	if !eq(got, want) {
		t.Errorf("newest-top order = %v, want %v", got, want)
	}
}

func TestSortMessagesDoesNotMutateInput(t *testing.T) {
	in := sample()
	before := contents(in)
	_ = sortMessages(in, false)
	if !eq(contents(in), before) {
		t.Error("sortMessages mutated its input slice")
	}
}

func TestFilterMessagesBySender(t *testing.T) {
	got := contents(filterMessages(sample(), "dev-lead")) // case-insensitive
	want := []string{"Dispatching PR review"}
	if !eq(got, want) {
		t.Errorf("filter by sender = %v, want %v", got, want)
	}
}

func TestFilterMessagesByBody(t *testing.T) {
	got := contents(filterMessages(sample(), "suite"))
	want := []string{"Test suite green"}
	if !eq(got, want) {
		t.Errorf("filter by body = %v, want %v", got, want)
	}
}

func TestFilterMessagesEmptyQueryReturnsAll(t *testing.T) {
	if got := filterMessages(sample(), "   "); len(got) != 3 {
		t.Errorf("empty query should return all 3, got %d", len(got))
	}
}

func TestFilterMessagesNoMatch(t *testing.T) {
	if got := filterMessages(sample(), "nonexistent"); len(got) != 0 {
		t.Errorf("expected no matches, got %d", len(got))
	}
}
