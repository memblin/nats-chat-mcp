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
	if m.confirm != nil {
		action := "evict"
		if m.confirm.kind == confirmCloseDM {
			action = "close DM"
		}
		return "CONFIRM", []hint{{"y", action}, {"any key", "cancel"}}
	}
	if m.picker != nil {
		return "NEW DM", []hint{{"↑/↓", "select"}, {"Enter", "open"}, {"Esc", "cancel"}}
	}
	if m.searching {
		return "SEARCH", []hint{{"type", "filter"}, {"Enter", "apply"}, {"Esc", "cancel"}}
	}
	switch m.focus {
	case zoneFeed:
		return "FEED", []hint{
			{"j/k", "scroll"}, {"/", "search"}, {"s", "sort"},
			{"g/G", "top/bottom"}, {"m", m.mouseHint()}, {"Tab", "next pane"}, {"q", "quit"},
		}
	case zoneRooms:
		hints := []hint{
			{"↑/↓", "select"}, {"Enter", "open"}, {"^]/^[", "switch"}, {"d", "new DM"},
		}
		if e := m.activeEntry(); e != nil && e.isDM {
			hints = append(hints, hint{"c", "close DM"})
		} else {
			hints = append(hints, hint{"x/X", "evict stale"})
		}
		hints = append(hints, hint{"m", m.mouseHint()}, hint{"Tab", "next pane"}, hint{"q", "quit"})
		return "ROOMS", hints
	default: // zoneCompose
		return "COMPOSE", []hint{
			{"Enter", "send"}, {"Esc", "to feed"}, {"Tab", "next pane"},
			{"mouse", "click panes"}, {"^C", "quit"},
		}
	}
}

// mouseHint is the description for the "m" key: it names the current mouse state
// so the operator can tell at a glance whether copy/paste selection is available.
func (m Model) mouseHint() string {
	if m.mouseOn {
		return "mouse on"
	}
	return "mouse off"
}
