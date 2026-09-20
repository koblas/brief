package markdown

import (
	"regexp"
	"strings"
)

// checklistItemRe matches a checklist item line: a hyphen bullet
// introducing "[ ]", "[x]" or "[X]", with any amount of leading
// whitespace so a hand-indented item still counts. "* [ ]", "+ [ ]" and
// "- []" are prose, not items — this grammar is deliberately the same
// shape as scaffold/render.go's own checklistItemRe, widened only to
// accept an uppercase tick, since two disagreeing definitions of
// "checklist item" in one binary is a trap. render.go's own regexp is not
// widened to match: its tickProgressEntry flips a literal "[ ]" to "[x]"
// by string replacement, so accepting "[X]" there would report a tick it
// never performed.
var checklistItemRe = regexp.MustCompile(`^\s*- \[([ xX])\]`)

// FirstUnchecked returns the first checklist item in the section under
// heading in body that is not ticked: line is its 1-based line number in
// the whole of body — not within the section — so a caller can report it
// against the file body was read from; text is the item's content, with
// leading whitespace and a trailing "\r" a CRLF body leaves attached both
// trimmed. A line inside a fenced code block (``` or ~~~) is never
// treated as an item, matching Section and UnterminatedFence. found is
// false when heading is absent from body, the section holds no checklist
// items, or every item in it is already ticked ("- [x]" or "- [X]") —
// FirstUnchecked never distinguishes those three cases, since a caller
// refusing on the trigger treats them identically.
func FirstUnchecked(body, heading string) (int, string, bool) {
	lines := strings.Split(body, "\n")

	headingIdx, sectionEnd, ok := sectionSpan(lines, heading)
	if !ok {
		return 0, "", false
	}

	var fence fenceState

	for i := headingIdx + 1; i < sectionEnd; i++ {
		line := lines[i]

		if fence.step(line) {
			continue
		}

		if fence.open {
			continue
		}

		m := checklistItemRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		if m[1] != " " {
			continue
		}

		_, after, cut := strings.Cut(line, "]")
		if !cut {
			continue
		}

		text := strings.TrimRight(strings.TrimLeft(after, " \t"), " \t\r")

		return i + 1, text, true
	}

	return 0, "", false
}
