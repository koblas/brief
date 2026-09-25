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

// thematicBreakRe matches a CommonMark thematic break line (CommonMark
// §4.1): a run of three or more of the same marker character — "-", "_"
// or "*" — each optionally followed by spaces or tabs, and nothing else
// on the line. "* * *" and "- - -" both start with what entryItemRe reads
// as a bullet marker; this is checked first so a horizontal rule is never
// mistaken for one.
var thematicBreakRe = regexp.MustCompile(`^(?:(?:-[ \t]*){3,}|(?:_[ \t]*){3,}|(?:\*[ \t]*){3,})$`)

// Entry is one list item Entries found under a configured heading: Line is
// its 1-based line number counted over the whole body it was scanned from,
// matching FirstUnchecked's own convention, fixed at the item's own first
// line and never advancing as later continuation lines fold in. Text is
// the item's content: its own leading marker and surrounding horizontal
// whitespace stripped, followed by every folded continuation line (see
// Entries), each joined in with a single space.
type Entry struct {
	Line int
	Text string
}

// Entries returns every column-0 list item ("- ", "* ", or "N. ") in the
// section under heading in body, in document order. A line inside a
// fenced code block is never treated as an item, matching Section and
// FirstUnchecked. Entries returns nil when heading is absent from body or
// its section holds no column-0 list item.
//
// An item folds in every following line as continuation text — regardless
// of that line's own indentation, so an indented sub-item folds in too,
// its own list marker kept verbatim (only the parent item's own leading
// marker is ever stripped, once, at Text's start) — until a blank line,
// the next column-0 item, a heading of any level, or a fence delimiter.
// A fence delimiter ends the entry in progress and begins the ordinary
// fence-skip scan: its contents are never entries and never fold into the
// entry before it, and the line immediately after the closing fence starts
// a fresh scan rather than folding into the entry before the fence either.
func Entries(body, heading string) []Entry {
	lines := strings.Split(body, "\n")

	headingIdx, sectionEnd, ok := sectionSpan(lines, heading)
	if !ok {
		return nil
	}

	var entries []Entry

	var fence fenceState

	i := headingIdx + 1
	for i < sectionEnd {
		line := lines[i]

		if fence.step(line) {
			i++
			continue
		}

		if fence.open {
			i++
			continue
		}

		text, ok := entryItemText(line)
		if !ok {
			i++
			continue
		}

		entryLine := i + 1

		parts := []string{text}

		j := i + 1
		for j < sectionEnd && isContinuationLine(lines[j]) {
			parts = append(parts, strings.TrimSpace(lines[j]))
			j++
		}

		entries = append(entries, Entry{Line: entryLine, Text: strings.Join(parts, " ")})

		i = j
	}

	return entries
}

// isContinuationLine reports whether line folds into the entry in
// progress as continuation text: it is not blank, not itself a column-0
// item, not a heading of any level, and not a fence delimiter.
func isContinuationLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}

	if _, ok := entryItemText(line); ok {
		return false
	}

	if headingLevelOf(line) > 0 {
		return false
	}

	if _, _, _, ok := fenceDelim(line); ok {
		return false
	}

	return true
}

// entryItemText reports line's item text when line is a column-0 list
// item, with the marker stripped and the remainder trimmed of trailing
// " \t\r" (trimEOL) — the marker match is already anchored at column 0, so
// no leading-whitespace trim is needed or wanted. It returns ("", false)
// for anything else: an indented line, a paragraph line, a thematic break
// (thematicBreakRe), or a line that merely starts with a marker character
// without the space that makes it one.
func entryItemText(line string) (string, bool) {
	trimmed := trimEOL(line)

	if thematicBreakRe.MatchString(trimmed) {
		return "", false
	}

	loc := entryItemRe.FindStringIndex(trimmed)
	if loc == nil {
		return "", false
	}

	return trimmed[loc[1]:], true
}
