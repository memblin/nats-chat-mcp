// Package ui is the Bubbletea TUI: a root model composing four regions — status
// bar, room list, presence panel, message feed — plus a compose input. Every
// Lip Gloss style lives in this file; no other file in the package constructs a
// style literal.
package ui

import "github.com/charmbracelet/lipgloss"

// Palette — a small, terminal-friendly set of colors used throughout.
var (
	colWhite  = lipgloss.Color("231")
	colDim    = lipgloss.Color("245")
	colDimmer = lipgloss.Color("240")
	colAccent = lipgloss.Color("39")  // active / focus highlight (cyan-blue)
	colGreen  = lipgloss.Color("42")  // connected
	colRed    = lipgloss.Color("196") // disconnected
	colYellow = lipgloss.Color("214") // reconnecting / unread badge
	colBg     = lipgloss.Color("236")
)

// Status bar.
var (
	styleStatusBar = lipgloss.NewStyle().
			Background(colBg).
			Foreground(colWhite)

	styleStatusApp = lipgloss.NewStyle().
			Background(colBg).
			Foreground(colAccent).
			Bold(true).
			Padding(0, 1)

	styleStatusSeg = lipgloss.NewStyle().
			Background(colBg).
			Foreground(colWhite).
			Padding(0, 1)

	styleDotConnected    = lipgloss.NewStyle().Background(colBg).Foreground(colGreen)
	styleDotDisconnected = lipgloss.NewStyle().Background(colBg).Foreground(colRed)
	styleDotReconnecting = lipgloss.NewStyle().Background(colBg).Foreground(colYellow)
)

// Panes — bordered boxes whose border color signals focus. The focused pane
// gets an accent border (and an accent title); the rest are dim.
var (
	stylePaneFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colAccent)

	stylePaneBlurred = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colDimmer)

	stylePaneTitle       = lipgloss.NewStyle().Foreground(colDim).Bold(true)
	stylePaneTitleActive = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
)

// paneStyle picks the focused or blurred pane frame.
func paneStyle(focused bool) lipgloss.Style {
	if focused {
		return stylePaneFocused
	}
	return stylePaneBlurred
}

// paneTitle picks the focused or blurred pane-title style.
func paneTitle(focused bool) lipgloss.Style {
	if focused {
		return stylePaneTitleActive
	}
	return stylePaneTitle
}

// Left column: room list + presence.
var (
	styleSectionHeader = lipgloss.NewStyle().Foreground(colDim).Bold(true)

	styleRoomNormal = lipgloss.NewStyle().Foreground(colWhite)

	styleRoomActive = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true)

	styleUnreadBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")).
				Background(colYellow).
				Padding(0, 1)

	stylePresenceName = lipgloss.NewStyle().Foreground(colWhite)
	stylePresenceIdle = lipgloss.NewStyle().Foreground(colDim)
)

// Message feed.
var (
	styleFeedHint = lipgloss.NewStyle().Foreground(colDim)

	styleFeedRule = lipgloss.NewStyle().Foreground(colDimmer)

	styleSender = lipgloss.NewStyle().
			Foreground(colAccent).
			Width(senderColWidth)

	styleSenderSelf = lipgloss.NewStyle().
			Foreground(colGreen).
			Width(senderColWidth)

	styleTimestamp = lipgloss.NewStyle().Foreground(colDimmer)

	styleBody = lipgloss.NewStyle().Foreground(colWhite)
)

// Help bar (bottom): the focused zone name plus context-sensitive key hints.
var (
	styleHelpBar = lipgloss.NewStyle().Foreground(colDim)

	styleHelpZone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("232")).
			Background(colAccent).
			Bold(true).
			Padding(0, 1)

	styleHelpKey = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleHelpSep = lipgloss.NewStyle().Foreground(colDimmer)
)

// Input bar.
var (
	styleSearchLabel = lipgloss.NewStyle().Foreground(colYellow).Bold(true)

	// styleInputActive marks the focus bar at the left of the compose line when
	// it is the focused zone; styleInputIdle when it is not.
	styleInputActive = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleInputIdle   = lipgloss.NewStyle().Foreground(colDimmer)
)
