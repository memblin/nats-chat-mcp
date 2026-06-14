package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// Column geometry for a rendered message line.
const (
	senderColWidth = 12 // fixed-width sender column
	tsColWidth     = 5  // "15:04"
)

// sortMessages returns a copy of msgs ordered for display. Messages are kept in
// time order; when newestBottom is false the order is reversed so the newest is
// at the top. The input slice is never mutated.
func sortMessages(msgs []natsclient.Message, newestBottom bool) []natsclient.Message {
	out := make([]natsclient.Message, len(msgs))
	copy(out, msgs)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time().Before(out[j].Time())
	})
	if !newestBottom {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

// filterMessages returns the messages whose sender or body contains query
// (case-insensitive). An empty query returns a copy of the input unchanged.
// Purely client-side; it never touches NATS.
func filterMessages(msgs []natsclient.Message, query string) []natsclient.Message {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]natsclient.Message, len(msgs))
		copy(out, msgs)
		return out
	}
	out := make([]natsclient.Message, 0, len(msgs))
	for _, m := range msgs {
		if strings.Contains(strings.ToLower(m.From), q) ||
			strings.Contains(strings.ToLower(m.Content), q) {
			out = append(out, m)
		}
	}
	return out
}

// renderFeed lays out the message slice (already sorted/filtered) into a block
// sized to width: a fixed sender column, a dimmed HH:MM timestamp, and a wrapped
// body whose continuation lines align under the first. selfID highlights the
// operator's own messages.
//
// Consecutive messages from the same sender are grouped: the sender name prints
// once at the top of the group, following messages blank that column (keeping
// their own timestamp + body), and a blank line separates one sender's group
// from the next — so it's easy to see where one post ends and another begins.
func renderFeed(msgs []natsclient.Message, width int, selfID string) string {
	if width <= 0 {
		return ""
	}
	indent := senderColWidth + 1 + tsColWidth + 1
	bodyWidth := width - indent
	if bodyWidth < 8 {
		bodyWidth = 8
	}

	var b strings.Builder
	prevKey := ""
	for i, m := range msgs {
		key := senderKey(m)
		grouped := i > 0 && key == prevKey
		if i > 0 && !grouped {
			b.WriteString("\n") // blank line between sender groups
		}

		// First message of a group shows the (styled) sender name; a grouped
		// message blanks the same-width column so timestamps stay aligned.
		senderCell := strings.Repeat(" ", senderColWidth)
		if !grouped {
			senderStyle := styleSender
			if m.FromID == selfID {
				senderStyle = styleSenderSelf
			}
			senderCell = senderStyle.Render(truncate(m.From, senderColWidth))
		}

		ts := m.Time().Format("15:04")
		lines := wrapCells(parseInline(m.Content), bodyWidth)
		b.WriteString(senderCell)
		b.WriteString(" ")
		b.WriteString(styleTimestamp.Render(ts))
		b.WriteString(" ")
		b.WriteString(renderCells(lines[0]))
		b.WriteString("\n")
		for _, cont := range lines[1:] {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(renderCells(cont))
			b.WriteString("\n")
		}
		prevKey = key
	}
	return strings.TrimRight(b.String(), "\n")
}

// senderKey identifies a message's sender for grouping: the stable FromID when
// present, otherwise the display name.
func senderKey(m natsclient.Message) string {
	if m.FromID != "" {
		return m.FromID
	}
	return m.From
}

// truncate shortens s to at most n runes, trailing the cut with "…".
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// --- inline markdown ---
//
// Message bodies may carry a little inline markdown — `code`, **bold**, *italic*
// — which the feed renders with styled spans rather than printing the markers.
// Parsing happens first, then wrapping operates on styled cells, so a style
// boundary in the middle of a word never throws off the width accounting (the
// reason this is cell-based rather than string-based).

// inlineKind tags a run of body text with the inline style to render it with.
type inlineKind int

const (
	kindBody inlineKind = iota
	kindCode
	kindBold
	kindItalic
)

// style maps an inline kind to its Lip Gloss style.
func (k inlineKind) style() lipgloss.Style {
	switch k {
	case kindCode:
		return styleCode
	case kindBold:
		return styleBold
	case kindItalic:
		return styleItalic
	default:
		return styleBody
	}
}

// cell is one visible rune tagged with its inline style. Wrapping operates on
// cells, so a style boundary mid-word never desynchronises the width math.
type cell struct {
	r    rune
	kind inlineKind
}

// parseInline turns a body into styled cells, stripping the markdown markers it
// recognises: `code` (literal content, no nesting), **bold**, and *italic*.
// Emphasis requires a non-space just inside its markers, so arithmetic like
// "2 * 3" stays literal; an unmatched marker is emitted as plain text.
func parseInline(s string) []cell {
	runes := []rune(s)
	var cells []cell
	emit := func(text []rune, kind inlineKind) {
		for _, r := range text {
			cells = append(cells, cell{r: r, kind: kind})
		}
	}
	for i, n := 0, len(runes); i < n; {
		switch {
		case runes[i] == '`':
			if end := indexRune(runes, '`', i+1); end > i+1 {
				emit(runes[i+1:end], kindCode)
				i = end + 1
				continue
			}
		case runes[i] == '*' && i+1 < n && runes[i+1] == '*':
			if end := closingMarker(runes, "**", i+2); end > 0 {
				emit(runes[i+2:end], kindBold)
				i = end + 2
				continue
			}
		case runes[i] == '*':
			if end := closingMarker(runes, "*", i+1); end > 0 {
				emit(runes[i+1:end], kindItalic)
				i = end + 1
				continue
			}
		}
		cells = append(cells, cell{r: runes[i], kind: kindBody})
		i++
	}
	return cells
}

// closingMarker returns the index of the matching closing marker for an emphasis
// span whose content starts at `from`, or -1. The character at `from` and the
// one just before the closer must both be non-space (so " * " and "** " don't
// open a span), and the span must be non-empty.
func closingMarker(runes []rune, marker string, from int) int {
	if from >= len(runes) || isSpace(runes[from]) {
		return -1
	}
	for end := indexSeq(runes, marker, from); end >= 0; end = indexSeq(runes, marker, end+len(marker)) {
		if end > from && !isSpace(runes[end-1]) {
			return end
		}
	}
	return -1
}

// wrapCells lays styled cells into lines no wider than width visible cells,
// breaking on spaces and hard-breaking any single word longer than width; a
// newline forces a break. Whitespace runs collapse to a single space. It always
// returns at least one (possibly empty) line.
func wrapCells(cells []cell, width int) [][]cell {
	if width <= 0 {
		return [][]cell{cells}
	}
	var lines [][]cell
	var line, word []cell
	flushWord := func() {
		if len(word) == 0 {
			return
		}
		for len(word) > width { // hard-break an over-long word
			if len(line) > 0 {
				lines = append(lines, line)
				line = nil
			}
			lines = append(lines, word[:width:width])
			word = word[width:]
		}
		switch {
		case len(line) == 0:
			line = append([]cell(nil), word...)
		case len(line)+1+len(word) <= width:
			line = append(line, cell{r: ' ', kind: kindBody})
			line = append(line, word...)
		default:
			lines = append(lines, line)
			line = append([]cell(nil), word...)
		}
		word = nil
	}
	for _, c := range cells {
		switch {
		case c.r == '\n':
			flushWord()
			lines = append(lines, line)
			line = nil
		case isSpace(c.r):
			flushWord()
		default:
			word = append(word, c)
		}
	}
	flushWord()
	if line != nil || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}

// renderCells renders a styled line, coalescing consecutive same-kind cells so
// each style's escape sequence is emitted once per run rather than per rune.
func renderCells(line []cell) string {
	if len(line) == 0 {
		return ""
	}
	var b strings.Builder
	start := 0
	for i := 1; i <= len(line); i++ {
		if i == len(line) || line[i].kind != line[start].kind {
			seg := make([]rune, 0, i-start)
			for _, c := range line[start:i] {
				seg = append(seg, c.r)
			}
			b.WriteString(line[start].kind.style().Render(string(seg)))
			start = i
		}
	}
	return b.String()
}

// indexRune returns the index of target at or after from, or -1.
func indexRune(runes []rune, target rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}

// indexSeq returns the index of the rune sequence seq at or after from, or -1.
func indexSeq(runes []rune, seq string, from int) int {
	s := []rune(seq)
	for i := from; i+len(s) <= len(runes); i++ {
		match := true
		for j := range s {
			if runes[i+j] != s[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// isSpace reports whether r is an in-line whitespace rune wrapping breaks on.
func isSpace(r rune) bool { return r == ' ' || r == '\t' }
