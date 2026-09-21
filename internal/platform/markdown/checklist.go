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
//
// The second group captures the item's text, so the marker and the text
// come from one match rather than a match followed by a hand-rolled split:
// [^\S\r\n] is "horizontal whitespace" (space or tab, never a newline,
// which matters because the body is split on "\n" but a "\r" can survive),
// and the lazy (.*?) with a trailing trim group leaves text free of
// surrounding blanks without a second pass.
//
// The order of the two tail elements is load-bearing: the trim class has
// to come before \r?, so that a line ending in blanks AND a carriage
// return has the blanks eaten before the \r is consumed. Swap them and the
// trim class finds nothing left to eat, the lazy group swallows both, and
// "- [ ] two \r" yields "two \r" instead of "two".
var checklistItemRe = regexp.MustCompile(`^\s*- \[([ xX])\][^\S\r\n]*(.*?)[^\S\r\n]*\r?$`)

// FirstUnchecked returns the first checklist item in the section under
// heading in body that is not ticked: line is its 1-based line number in
// the whole of body — not within the section — so a caller can report it
// against the file body was read from; text is the item's content, with
// horizontal whitespace at either end, and a trailing "\r" a CRLF body
// leaves attached, all trimmed. A line inside a fenced code block (``` or
// ~~~) is never treated as an item, matching Section and
// UnterminatedFence. found is
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

		return i + 1, m[2], true
	}

	return 0, "", false
}
