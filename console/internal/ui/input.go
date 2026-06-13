package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFeedPanel draws the message feed: a header row (active room + hints), a
// horizontal rule, and the scrollable viewport.
func (m Model) renderFeedPanel() string {
	room := m.activeName()
	if room == "" {
		room = "no room"
	}
	title := styleFeedHeader.Render(room)
	hints := styleFeedHint.Render("[s]ort  [/]search")
	gap := m.feedW - lipgloss.Width(title) - lipgloss.Width(hints)
	if gap < 1 {
		gap = 1
	}
	header := title + strings.Repeat(" ", gap) + hints
	rule := styleFeedRule.Render(strings.Repeat("─", max(1, m.feedW)))

	body := lipgloss.NewStyle().Width(m.feedW).Height(m.viewport.Height).Render(m.viewport.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, rule, body)
}

// renderInput draws the bottom bar: either the compose prompt or, in search
// mode, the live filter query.
func (m Model) renderInput() string {
	if m.searching {
		label := styleSearchLabel.Render("/")
		return lipgloss.NewStyle().Width(m.width).Render(label + m.searchQuery + "▏")
	}
	return lipgloss.NewStyle().Width(m.width).Render(m.input.View())
}
