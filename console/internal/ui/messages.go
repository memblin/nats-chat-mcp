package ui

import (
	"sort"
	"strings"

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
	for _, m := range msgs {
		sender := truncate(m.From, senderColWidth)
		senderStyle := styleSender
		if m.FromID == selfID {
			senderStyle = styleSenderSelf
		}
		ts := m.Time().Format("15:04")

		lines := wrap(m.Content, bodyWidth)
		if len(lines) == 0 {
			lines = []string{""}
		}
		b.WriteString(senderStyle.Render(sender))
		b.WriteString(" ")
		b.WriteString(styleTimestamp.Render(ts))
		b.WriteString(" ")
		b.WriteString(styleBody.Render(lines[0]))
		b.WriteString("\n")
		for _, cont := range lines[1:] {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(styleBody.Render(cont))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
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

// wrap breaks s into lines no wider than width runes, splitting on spaces and
// hard-breaking any single word longer than width.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := ""
		for _, w := range words {
			for len([]rune(w)) > width { // hard-break an over-long word
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				rw := []rune(w)
				lines = append(lines, string(rw[:width]))
				w = string(rw[width:])
			}
			switch {
			case cur == "":
				cur = w
			case len([]rune(cur))+1+len([]rune(w)) <= width:
				cur += " " + w
			default:
				lines = append(lines, cur)
				cur = w
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	return lines
}
