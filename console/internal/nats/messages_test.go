package nats

import (
	"testing"
	"time"
)

func TestParseMessageRoundTrip(t *testing.T) {
	// A payload shaped exactly like the MCP server publishes (src/types.ts).
	raw := []byte(`{
		"id": "abc-123",
		"from": "Dev-Lead",
		"from_id": "a1b2c3",
		"room": "go-virt",
		"content": "Dispatching PR review",
		"timestamp": "2026-06-13T14:32:00.000Z",
		"reply_to": "prev-1"
	}`)

	m, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage returned error: %v", err)
	}
	if m.ID != "abc-123" || m.From != "Dev-Lead" || m.FromID != "a1b2c3" {
		t.Errorf("identity fields wrong: %+v", m)
	}
	if m.Room != "go-virt" || m.Content != "Dispatching PR review" || m.ReplyTo != "prev-1" {
		t.Errorf("content fields wrong: %+v", m)
	}
	if got := m.Time().UTC(); got != time.Date(2026, 6, 13, 14, 32, 0, 0, time.UTC) {
		t.Errorf("Time() = %v, want 2026-06-13T14:32:00Z", got)
	}
}

func TestParseMessageOmitsOptionalFields(t *testing.T) {
	// A direct message has no room; reply_to absent.
	m, err := ParseMessage([]byte(`{"id":"x","from":"chris","from_id":"h9","content":"hi","timestamp":"2026-06-13T14:00:00.000Z"}`))
	if err != nil {
		t.Fatalf("ParseMessage error: %v", err)
	}
	if m.Room != "" || m.ReplyTo != "" {
		t.Errorf("expected empty room/reply_to, got %+v", m)
	}
}

func TestParseMessageMalformed(t *testing.T) {
	if _, err := ParseMessage([]byte("not json")); err == nil {
		t.Error("expected error for malformed payload, got nil")
	}
}

func TestMessageTimeUnparseable(t *testing.T) {
	m := Message{Timestamp: "whenever"}
	if !m.Time().IsZero() {
		t.Errorf("expected zero time for bad stamp, got %v", m.Time())
	}
}

func TestFormatIdle(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{32 * time.Second, "32s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{74 * time.Second, "1m14s"},
		{8 * time.Minute, "8m"},
		{59*time.Minute + 59*time.Second, "59m59s"},
		{time.Hour, "1h0m"},
		{time.Hour + 5*time.Minute, "1h5m"},
		{-5 * time.Second, "0s"}, // clock skew clamps to zero
	}
	for _, c := range cases {
		if got := FormatIdle(c.d); got != c.want {
			t.Errorf("FormatIdle(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
