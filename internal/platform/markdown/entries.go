package markdown

import (
	"regexp"
	"strings"
)

// entryItemRe matches a column-0 list item marker: "- ", "* ", or "N. ".
var entryItemRe = regexp.MustCompile(`^(?:[-*] |\d+\. )`)

// thematicBreakRe matches a CommonMark thematic break: up to three leading spaces then three or more of the same "-", "_" or "*" marker.
var thematicBreakRe = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:_[ \t]*){3,}|(?:\*[ \t]*){3,})$`)

// Entry is one list item Entries found under a configured heading. Line
// is its 1-based line number, fixed at the item's own first line. Text is
// the item's content with its own leading marker stripped, followed by
// every folded continuation line joined in with a single space.
type Entry struct {
	Line int
	Text string
}

// Entries returns every column-0 list item ("- ", "* ", or "N. ") in the
// section under heading in body, in document order, folding each item's
// wrapped and indented continuation lines into its Text until a blank
// line, the next item, a heading, a thematic break, or a fence. Entries
// returns nil when heading is absent from body or its section holds none.
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
// progress: not blank, not itself a column-0 item, thematic break, heading, or fence delimiter.
func isContinuationLine(line string) bool {
	trimmed := trimEOL(line)

	if strings.TrimSpace(trimmed) == "" {
		return false
	}

	if thematicBreakRe.MatchString(trimmed) {
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

// entryItemText reports line's item text with the marker stripped, or
// ("", false) when line is not a column-0 list item.
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
