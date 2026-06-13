// Package nats holds every NATS/JetStream interaction the console performs,
// plus the on-the-wire schema and subject conventions. These are ported
// verbatim from the TypeScript MCP server (src/stream-manager.ts, src/types.ts)
// so the console interoperates transparently with agent sessions.
package nats

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// Message mirrors the MCP server's Message interface (src/types.ts) field for
// field, including the JSON tag names, so a message published by the console is
// indistinguishable from one published by an agent and vice versa.
type Message struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	FromID    string `json:"from_id"`
	Room      string `json:"room,omitempty"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	ReplyTo   string `json:"reply_to,omitempty"`
}

// ParseMessage decodes a raw JetStream payload into a Message. A decode error
// is returned to the caller, which typically skips the malformed payload rather
// than crashing — matching the MCP's tolerant drain loop.
func ParseMessage(data []byte) (Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return Message{}, fmt.Errorf("decode message: %w", err)
	}
	return m, nil
}

// encode serialises a Message for publishing.
func (m Message) encode() ([]byte, error) {
	return json.Marshal(m)
}

// Time parses the ISO-8601 timestamp the MCP writes (Date.toISOString()). Go's
// RFC3339 layout accepts the fractional-second "...Z" form on parse. A zero
// time is returned for an unparseable value so callers can sort/format safely.
func (m Message) Time() time.Time {
	t, err := time.Parse(time.RFC3339, m.Timestamp)
	if err != nil {
		return time.Time{}
	}
	return t
}

// newMessage stamps an outgoing message with this identity, mirroring
// identity.ts#newMessage: a fresh UUID id, the sender name and id, an ISO
// timestamp, and the optional room / reply_to.
func newMessage(id Identity, content, room, replyTo string) Message {
	return Message{
		ID:        uuidv4(),
		From:      id.Name,
		FromID:    id.ID,
		Room:      room,
		Content:   content,
		Timestamp: nowISO(),
		ReplyTo:   replyTo,
	}
}

// nowISO renders the current time the way JavaScript's Date.toISOString() does:
// UTC, millisecond precision, trailing "Z".
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// uuidv4 generates a random RFC-4122 v4 UUID string without pulling in a
// dependency, matching the shape the MCP stamps onto message ids.
func uuidv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is catastrophic; fall back to a time-derived id so
		// the message still has a unique-enough handle rather than panicking.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
