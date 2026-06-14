package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// renderStatusBar draws the full-width top bar: app name, identity@server, the
// active room, and a colored connection dot.
func (m Model) renderStatusBar() string {
	app := styleStatusApp.Render("nats-chat-console")

	who := m.self.Name + " @ " + hostOf(m.cfg.NatsURL)
	ident := styleStatusSeg.Render(who)

	room := m.activeName()
	if room == "" {
		room = "—"
	}
	roomSeg := styleStatusSeg.Render(room)

	conn := m.connSegment()

	left := lipgloss.JoinHorizontal(lipgloss.Top, app, sep(), ident, sep(), roomSeg)
	if !m.mouseOn {
		left = lipgloss.JoinHorizontal(lipgloss.Top, left, sep(), styleStatusMouseOff.Render("MOUSE OFF"))
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(conn)
	if gap < 1 {
		gap = 1
	}
	bar := left + styleStatusBar.Render(strings.Repeat(" ", gap)) + conn
	return styleStatusBar.Width(m.width).MaxWidth(m.width).Render(bar)
}

// sep is the dim separator between status segments.
func sep() string { return styleStatusBar.Render("│") }

// connSegment renders the connection dot and label for the current state.
func (m Model) connSegment() string {
	switch m.conn {
	case natsclient.Connected:
		return styleDotConnected.Render("● connected") + styleStatusBar.Render(" ")
	case natsclient.Reconnecting:
		return styleDotReconnecting.Render("● reconnecting") + styleStatusBar.Render(" ")
	default:
		return styleDotDisconnected.Render("● disconnected") + styleStatusBar.Render(" ")
	}
}

// hostOf strips the scheme and path from a NATS URL, leaving host[:port].
func hostOf(url string) string {
	s := url
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}
