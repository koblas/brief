package markdown

import (
	"regexp"
	"strings"
)

// entryItemRe matches a column-0 list item marker: a hyphen or asterisk
// bullet ("- ", "* ") or a decimal ordered marker ("N. "), each followed
// by exactly one space. Unlike checklistItemRe, it carries no leading
// "\s*": an indented line is never an entry.
var entryItemRe = regexp.MustCompile(`^(?:[-*] |\d+\. )`)

// Entry is one list item Entries found under a configured heading: Line is
// its 1-based line number counted over the whole body it was scanned
// from, matching FirstUnchecked's own convention, and Text is the item's
// content with its marker and surrounding horizontal whitespace stripped.
type Entry struct {
	Line int
	Text string
}

// Entries returns every column-0 list item ("- ", "* ", or "N. ") in the
// section under heading in body, in document order. A line inside a
// fenced code block is never treated as an item, matching Section and
// FirstUnchecked. Entries returns nil when heading is absent from body or
// its section holds no column-0 list item — an indented line and a
// paragraph line are both ignored, never entries of their own.
func Entries(body, heading string) []Entry {
	lines := strings.Split(body, "\n")

	headingIdx, sectionEnd, ok := sectionSpan(lines, heading)
	if !ok {
		return nil
	}

	var entries []Entry

	var fence fenceState

	for i := headingIdx + 1; i < sectionEnd; i++ {
		line := lines[i]

		if fence.step(line) {
			continue
		}

		if fence.open {
			continue
		}

		text, ok := entryItemText(line)
		if !ok {
			continue
		}

		entries = append(entries, Entry{Line: i + 1, Text: text})
	}

	return entries
}

// entryItemText reports line's item text when line is a column-0 list
// item, with the marker stripped and the remainder trimmed of trailing
// " \t\r" (trimEOL) — the marker match is already anchored at column 0, so
// no leading-whitespace trim is needed or wanted. It returns ("", false)
// for anything else: an indented line, a paragraph line, or a line that
// merely starts with a marker character without the space that makes it
// one.
func entryItemText(line string) (string, bool) {
	trimmed := trimEOL(line)

	loc := entryItemRe.FindStringIndex(trimmed)
	if loc == nil {
		return "", false
	}

	return trimmed[loc[1]:], true
}
