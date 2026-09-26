package markdown

import (
	"regexp"
	"strings"
)

// checklistItemRe matches an indented "- [ ]"/"- [x]"/"- [X]" item, capturing the tick and trimmed text.
var checklistItemRe = regexp.MustCompile(`^\s*- \[([ xX])\][^\S\r\n]*(.*?)[^\S\r\n]*\r?$`)

// checklistItem is one recognized "- [ ]"/"- [x]" line: its 1-based line
// number in the whole body, whether it is ticked, and its trimmed text.
type checklistItem struct {
	line   int
	ticked bool
	text   string
}

// scanChecklistItems returns every checklist item, in document order, in
// the section under heading in body — the recognition rules FirstUnchecked
// and CountChecklistItems share: fenced blocks, indentation and CRLF
// tolerated, a subsection heading nested inside stays in scope. found is
// false when heading is absent from body.
func scanChecklistItems(body, heading string) ([]checklistItem, bool) {
	lines := strings.Split(body, "\n")

	headingIdx, sectionEnd, ok := sectionSpan(lines, heading)
	if !ok {
		return nil, false
	}

	var items []checklistItem

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

		items = append(items, checklistItem{line: i + 1, ticked: m[1] != " ", text: m[2]})
	}

	return items, true
}

// FirstUnchecked returns the first unticked checklist item in the section
// under heading in body: line is its 1-based line number in the whole
// body, text is the item's trimmed content. found is false when heading
// is absent, the section has no checklist items, or every item is ticked.
func FirstUnchecked(body, heading string) (int, string, bool) {
	items, ok := scanChecklistItems(body, heading)
	if !ok {
		return 0, "", false
	}

	for _, item := range items {
		if !item.ticked {
			return item.line, item.text, true
		}
	}

	return 0, "", false
}

// CountChecklistItems returns the number of checklist items — ticked or
// not — in the section under heading in body, using the same recognition
// rules as FirstUnchecked. found is false when heading is absent from body;
// a present heading with no items returns (0, true).
func CountChecklistItems(body, heading string) (int, bool) {
	items, ok := scanChecklistItems(body, heading)
	if !ok {
		return 0, false
	}

	return len(items), true
}
