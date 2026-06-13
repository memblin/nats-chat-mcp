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

// Left column: room list + presence.
var (
	styleLeftPanel = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(colDimmer)

	styleSectionHeader = lipgloss.NewStyle().
				Foreground(colDim).
				Bold(true)

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
	styleFeedHeader = lipgloss.NewStyle().
			Foreground(colWhite).
			Bold(true)

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

// Input bar.
var styleSearchLabel = lipgloss.NewStyle().Foreground(colYellow).Bold(true)

// Focus indicator applied to the focused region's header.
var styleFocused = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
