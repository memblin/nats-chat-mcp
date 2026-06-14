package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// presenceStaleThreshold is how long since a participant's last_seen before its
// row is dimmed as "probably gone". It is tied to the heartbeat cadence: live
// participants refresh on a fixed interval (the console every 30s, a registered
// MCP agent every 60s — see src/heartbeat.ts), so 3 min is roughly three missed
// agent heartbeats — a row only dims when an entity is genuinely lapsing toward
// the 5-min presenceTTL, not during normal between-heartbeat gaps.
const presenceStaleThreshold = 3 * time.Minute

// dedupePresence collapses records that share a display name, keeping the one
// with the freshest last_seen, and returns them sorted by name. A session that
// restarts mints a fresh id, so its dead record lingers under the same name
// until its TTL expires; without this the panel would show it alongside the
// live one. Trade-off: two genuinely-live participants sharing a display name
// collapse to a single row — operators give distinct names to disambiguate.
func dedupePresence(in []natsclient.Presence) []natsclient.Presence {
	freshest := make(map[string]natsclient.Presence, len(in))
	for _, p := range in {
		if cur, ok := freshest[p.Name]; !ok || p.LastSeenTime().After(cur.LastSeenTime()) {
			freshest[p.Name] = p
		}
	}
	out := make([]natsclient.Presence, 0, len(freshest))
	for _, p := range freshest {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// staleParticipants returns the presence records eligible for eviction: those
// whose last_seen is older than presenceStaleThreshold (the dimmed rows),
// excluding this console's own identity. When room is non-empty only
// participants in that room are returned; otherwise every stale participant
// across all rooms. It works over the RAW slice (not the deduped view) so every
// ghost id hiding behind a shared display name is caught. Sorted by name.
func staleParticipants(ps []natsclient.Presence, now time.Time, room, selfID string) []natsclient.Presence {
	out := make([]natsclient.Presence, 0)
	for _, p := range ps {
		if p.ID == selfID {
			continue
		}
		if now.Sub(p.LastSeenTime()) <= presenceStaleThreshold {
			continue
		}
		if room != "" && !contains(p.Rooms, room) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// contains reports whether room is present in rooms.
func contains(rooms []string, room string) bool {
	for _, r := range rooms {
		if r == room {
			return true
		}
	}
	return false
}

// presenceRows returns the agents to show in the panel: those present in the
// active room, or everyone when no room is active. Same-name duplicates are
// collapsed to their freshest record.
func (m Model) presenceRows() []natsclient.Presence {
	e := m.activeEntry()
	if e == nil || e.isDM {
		return dedupePresence(m.presence)
	}
	room := e.name
	out := make([]natsclient.Presence, 0, len(m.presence))
	for _, p := range m.presence {
		for _, r := range p.Rooms {
			if r == room {
				out = append(out, p)
				break
			}
		}
	}
	return dedupePresence(out)
}

// renderPresence draws the PRESENCE section: each agent's name with its idle
// time (now minus last-seen) right-aligned.
func (m Model) renderPresence() string {
	_, _, presenceLines := m.leftSplit()
	header := styleSectionHeader.Render("PRESENCE")

	rows := m.presenceRows()
	visible := presenceLines - 1
	if visible < 0 {
		visible = 0
	}
	if len(rows) > visible {
		rows = rows[:visible]
	}

	lines := make([]string, 0, len(rows))
	for _, p := range rows {
		elapsed := m.now.Sub(p.LastSeenTime())
		idle := natsclient.FormatIdle(elapsed)
		nameStyle := stylePresenceName
		if elapsed > presenceStaleThreshold {
			nameStyle = stylePresenceNameStale // probably gone — haven't heard from it
		}
		name := nameStyle.Render(truncate(p.Name, m.leftContentW-len(idle)-1))
		gap := m.leftContentW - lipgloss.Width(name) - len(idle)
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, name+strings.Repeat(" ", gap)+stylePresenceIdle.Render(idle))
	}
	body := strings.Join(lines, "\n")
	if len(rows) == 0 {
		body = stylePresenceIdle.Render("  (nobody here)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
