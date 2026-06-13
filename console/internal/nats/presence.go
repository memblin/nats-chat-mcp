package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// heartbeatInterval is how often the console refreshes its presence record (and
// thus its KV TTL). presencePollInterval is how often it re-reads the registry
// to refresh the panel; the spec asks for "every 10 seconds minimum".
const (
	heartbeatInterval    = 30 * time.Second
	presencePollInterval = 10 * time.Second
)

// Presence mirrors the MCP's AgentPresence (src/types.ts): identity plus the
// rooms the participant is in and an ISO last-seen stamp. The console writes one
// of these to the presence KV so it appears in list_agents like an agent.
type Presence struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Rooms    []string `json:"rooms"`
	LastSeen string   `json:"last_seen"`
}

// LastSeenTime parses the ISO last_seen stamp; zero time if unparseable.
func (p Presence) LastSeenTime() time.Time {
	t, err := time.Parse(time.RFC3339, p.LastSeen)
	if err != nil {
		return time.Time{}
	}
	return t
}

// PublishPresence writes (or refreshes) this console's presence record into the
// shared KV registry, stamping the current joined rooms and a fresh last_seen.
// Calling it repeatedly is how the heartbeat keeps the TTL-backed entry alive.
func (c *Client) PublishPresence(ctx context.Context) error {
	p := Presence{
		ID:       c.id.ID,
		Name:     c.id.Name,
		Rooms:    c.JoinedRooms(),
		LastSeen: nowISO(),
	}
	if p.Rooms == nil {
		p.Rooms = []string{}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err := c.kv.Put(ctx, c.id.ID, data); err != nil {
		return fmt.Errorf("publish presence: %w", err)
	}
	return nil
}

// DeletePresence removes this console's presence entry — the prompt departure
// signal published on leave/quit so peers don't wait out the TTL.
func (c *Client) DeletePresence(ctx context.Context) error {
	if c.kv == nil {
		return nil
	}
	if err := c.kv.Delete(ctx, c.id.ID); err != nil {
		return fmt.Errorf("delete presence: %w", err)
	}
	return nil
}

// ListPresence reads every live presence record from the registry, sorted by
// name for a stable panel ordering.
func (c *Client) ListPresence(ctx context.Context) ([]Presence, error) {
	keys, err := c.kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("list presence keys: %w", err)
	}

	out := make([]Presence, 0, len(keys))
	for _, k := range keys {
		entry, gerr := c.kv.Get(ctx, k)
		if gerr != nil {
			continue // entry expired between listing and reading; skip it
		}
		var p Presence
		if json.Unmarshal(entry.Value(), &p) != nil {
			continue // skip malformed record
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// StartHeartbeat publishes presence immediately and then on heartbeatInterval
// until ctx is cancelled. Run it in a goroutine.
func (c *Client) StartHeartbeat(ctx context.Context) {
	_ = c.PublishPresence(ctx)
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = c.PublishPresence(ctx)
		}
	}
}

// StartPresencePoll reads the registry on presencePollInterval and emits a
// PresenceEvent so the UI refreshes the panel. Run it in a goroutine.
func (c *Client) StartPresencePoll(ctx context.Context) {
	emit := func() {
		if agents, err := c.ListPresence(ctx); err == nil {
			c.emit(PresenceEvent{Agents: agents})
		}
	}
	emit()
	t := time.NewTicker(presencePollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			emit()
		}
	}
}

// FormatIdle renders an elapsed duration the way the presence panel shows it:
//   - under a minute:      "32s"
//   - under an hour:       "1m14s"  (or "8m" when the seconds are zero)
//   - an hour or more:     "1h5m"
//
// A negative duration (clock skew) is clamped to zero.
func FormatIdle(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		m, s := secs/60, secs%60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		h, m := secs/3600, (secs%3600)/60
		return fmt.Sprintf("%dh%dm", h, m)
	}
}
