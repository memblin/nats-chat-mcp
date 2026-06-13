package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// hint is a single key/description pair shown in the help bar.
type hint struct{ key, desc string }

// renderHelp draws the bottom help line: the focused zone, then context-specific
// key hints. This is the primary cue for where focus is and how to navigate.
func (m Model) renderHelp() string {
	zone, hints := m.helpFor()
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, styleHelpKey.Render(h.key)+" "+styleHelpBar.Render(h.desc))
	}
	line := styleHelpZone.Render(zone) + " " + strings.Join(parts, styleHelpSep.Render(" · "))
	return lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Render(line)
}

// helpFor returns the zone label and key hints for the current mode/focus.
func (m Model) helpFor() (string, []hint) {
	if m.searching {
		return "SEARCH", []hint{{"type", "filter"}, {"Enter", "apply"}, {"Esc", "cancel"}}
	}
	switch m.focus {
	case zoneFeed:
		return "FEED", []hint{
			{"j/k", "scroll"}, {"/", "search"}, {"s", "sort"},
			{"g/G", "top/bottom"}, {"Tab", "next pane"}, {"q", "quit"},
		}
	case zoneRooms:
		return "ROOMS", []hint{
			{"↑/↓", "select"}, {"Enter", "open"}, {"^]/^[", "switch room"},
			{"Tab", "next pane"}, {"q", "quit"},
		}
	default: // zoneCompose
		return "COMPOSE", []hint{
			{"Enter", "send"}, {"Esc", "to feed"}, {"Tab", "next pane"},
			{"mouse", "click panes"}, {"^C", "quit"},
		}
	}
}
