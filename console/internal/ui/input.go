package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFeedPane draws the bordered message-feed pane: a title row (active room
// + hints), a horizontal rule, and the scrollable viewport. Its border is
// accented when the feed zone has focus.
func (m Model) renderFeedPane() string {
	room := m.activeName()
	if room == "" {
		room = "no room"
	}
	focused := m.focus == zoneFeed
	title := paneTitle(focused).Render(room)
	hints := styleFeedHint.Render("[s]ort  [/]search")
	gap := m.feedContentW - lipgloss.Width(title) - lipgloss.Width(hints)
	if gap < 1 {
		gap = 1
	}
	header := title + strings.Repeat(" ", gap) + hints
	rule := styleFeedRule.Render(strings.Repeat("─", max(1, m.feedContentW)))
	body := lipgloss.NewStyle().Width(m.feedContentW).Height(m.viewport.Height).Render(m.viewport.View())

	content := lipgloss.JoinVertical(lipgloss.Left, header, rule, body)
	return paneStyle(focused).
		Width(m.feedContentW).
		Height(m.paneContentH).
		Render(content)
}

// renderInput draws the bottom bar: either the compose prompt or, in search
// mode, the live filter query. A left bar marks whether composing has focus.
func (m Model) renderInput() string {
	if m.searching {
		bar := styleInputActive.Render("▌")
		label := styleSearchLabel.Render(" /")
		return lipgloss.NewStyle().Width(m.width).Render(bar + label + m.searchQuery + "▏")
	}

	if m.focus == zoneCompose {
		bar := styleInputActive.Render("▌")
		return lipgloss.NewStyle().Width(m.width).Render(bar + " " + m.input.View())
	}

	bar := styleInputIdle.Render("▌")
	hint := styleInputIdle.Render(" press Tab or click here to type")
	return lipgloss.NewStyle().Width(m.width).Render(bar + hint)
}
