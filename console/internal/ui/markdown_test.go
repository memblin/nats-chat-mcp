package ui

import (
	"strings"
	"testing"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// visible reconstructs the plain text of a cell slice (markers already stripped).
func visible(cells []cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.r)
	}
	return b.String()
}

// kindsOf returns the inline kind for each rune of substr's first occurrence,
// asserting they are all the same; it is a small helper for span assertions.
func spanKind(t *testing.T, cells []cell, substr string) inlineKind {
	t.Helper()
	text := visible(cells)
	at := strings.Index(text, substr)
	if at < 0 {
		t.Fatalf("substring %q not found in %q", substr, text)
	}
	// Index is byte-based; for the ASCII fixtures here that matches rune offset.
	kind := cells[at].kind
	for i := at; i < at+len(substr); i++ {
		if cells[i].kind != kind {
			t.Fatalf("span %q is not uniformly styled (%d != %d at %d)", substr, cells[i].kind, kind, i)
		}
	}
	return kind
}

// parseInline strips the recognised markers and tags each span with its style.
func TestParseInlineStripsMarkersAndTagsSpans(t *testing.T) {
	cells := parseInline("a `ls -l` b **big** c *lean* d")
	if got, want := visible(cells), "a ls -l b big c lean d"; got != want {
		t.Fatalf("visible text = %q, want %q", got, want)
	}
	if k := spanKind(t, cells, "ls -l"); k != kindCode {
		t.Errorf("`ls -l` kind = %d, want code", k)
	}
	if k := spanKind(t, cells, "big"); k != kindBold {
		t.Errorf("**big** kind = %d, want bold", k)
	}
	if k := spanKind(t, cells, "lean"); k != kindItalic {
		t.Errorf("*lean* kind = %d, want italic", k)
	}
	if k := spanKind(t, cells, "a "); k != kindBody {
		t.Errorf("plain text kind = %d, want body", k)
	}
}

// A lone '*' used as arithmetic (space on both sides) is left as literal text,
// not treated as an emphasis marker.
func TestParseInlineLeavesArithmeticLiteral(t *testing.T) {
	cells := parseInline("2 * 3 = 6")
	if got := visible(cells); got != "2 * 3 = 6" {
		t.Fatalf("visible text = %q, want it unchanged", got)
	}
	for i, c := range cells {
		if c.kind != kindBody {
			t.Fatalf("cell %d (%q) styled %d, want all-body", i, string(c.r), c.kind)
		}
	}
}

// An unterminated marker is emitted verbatim rather than swallowing the rest.
func TestParseInlineUnmatchedMarkerIsLiteral(t *testing.T) {
	cells := parseInline("use `git status to check")
	if got := visible(cells); got != "use `git status to check" {
		t.Errorf("visible text = %q, want the backtick preserved", got)
	}
}

// renderFeed strips the markers from the visible output and keeps the inner text.
func TestRenderFeedStripsInlineMarkers(t *testing.T) {
	msgs := []natsclient.Message{
		msg("dev", "run `make build` then **ship** it", "2026-06-13T14:28:00.000Z"),
	}
	out := renderFeed(msgs, 80, "")
	if strings.Contains(out, "`") || strings.Contains(out, "**") {
		t.Errorf("rendered feed still contains markdown markers:\n%q", out)
	}
	for _, want := range []string{"make build", "ship", "then", "it"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered feed is missing %q:\n%q", want, out)
		}
	}
}

// Wrapping operates on visible width: every wrapped line stays within the width
// even when the text carries (zero-width) style markers in the source.
func TestWrapCellsRespectsVisibleWidth(t *testing.T) {
	const width = 10
	cells := parseInline("alpha **bravo charlie** delta echo foxtrot")
	for _, line := range wrapCells(cells, width) {
		if len(line) > width {
			t.Errorf("line %q has visible width %d, exceeds %d", visible([]cell(line)), len(line), width)
		}
	}
}

// A styled phrase that wraps keeps its style on every line it spans.
func TestWrapCellsPreservesStyleAcrossLines(t *testing.T) {
	cells := parseInline("**bravo charlie delta**")
	lines := wrapCells(cells, 8)
	if len(lines) < 2 {
		t.Fatalf("expected the bold phrase to wrap onto multiple lines, got %d", len(lines))
	}
	for _, line := range lines {
		for _, c := range line {
			if c.r == ' ' {
				continue // the inserted separator is plain
			}
			if c.kind != kindBold {
				t.Errorf("wrapped bold text lost its style: %q kind %d", string(c.r), c.kind)
			}
		}
	}
}
