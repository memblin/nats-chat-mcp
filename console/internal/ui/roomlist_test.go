package ui

import (
	"strings"
	"testing"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// The room list, its header, the divider, and the presence panel must together
// fit exactly within paneContentH — otherwise the bordered box overflows. This
// guards the leftDividerLines accounting in leftSplit.
func TestLeftSplitBudgetsDivider(t *testing.T) {
	m := Model{
		paneContentH: 20,
		presence: []natsclient.Presence{
			pres("Amy", "2026-06-13T14:00:00.000Z"),
			pres("Bob", "2026-06-13T14:00:00.000Z"),
		},
	}
	roomLines, dmLines, presenceLines := m.leftSplit()
	const roomsHeader = 1
	// No DM threads here, so the DM section collapses (dmLines == 0, no header).
	used := roomsHeader + roomLines + dmLines + leftDividerLines + presenceLines
	if used != m.paneContentH {
		t.Errorf("left column uses %d rows, want exactly paneContentH=%d (rooms hdr %d + rooms %d + dm %d + divider %d + presence %d)",
			used, m.paneContentH, roomsHeader, roomLines, dmLines, leftDividerLines, presenceLines)
	}
}

func TestRenderLeftDividerShape(t *testing.T) {
	m := Model{leftContentW: 12}
	div := m.renderLeftDivider()
	lines := strings.Split(div, "\n")
	if len(lines) != leftDividerLines {
		t.Fatalf("divider has %d lines, want %d", len(lines), leftDividerLines)
	}
	if strings.TrimSpace(lines[0]) != "" {
		t.Errorf("first divider line should be a blank spacer, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "─") {
		t.Errorf("second divider line should be a rule, got %q", lines[1])
	}
}
