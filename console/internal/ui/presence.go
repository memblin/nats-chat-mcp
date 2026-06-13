package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// presenceRows returns the agents to show in the panel: those present in the
// active room, or everyone when no room is active.
func (m Model) presenceRows() []natsclient.Presence {
	room := m.activeName()
	if room == "" || room == directBucket {
		return m.presence
	}
	out := make([]natsclient.Presence, 0, len(m.presence))
	for _, p := range m.presence {
		for _, r := range p.Rooms {
			if r == room {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// renderPresence draws the PRESENCE section: each agent's name with its idle
// time (now minus last-seen) right-aligned.
func (m Model) renderPresence() string {
	_, presenceLines := m.leftSplit()
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
		idle := natsclient.FormatIdle(m.now.Sub(p.LastSeenTime()))
		name := stylePresenceName.Render(truncate(p.Name, m.leftContentW-len(idle)-1))
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
