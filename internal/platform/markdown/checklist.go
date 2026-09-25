package markdown

import (
	"regexp"
	"strings"
)

// checklistItemRe matches an indented "- [ ]"/"- [x]"/"- [X]" item, capturing the tick and trimmed text.
var checklistItemRe = regexp.MustCompile(`^\s*- \[([ xX])\][^\S\r\n]*(.*?)[^\S\r\n]*\r?$`)

// FirstUnchecked returns the first unticked checklist item in the section
// under heading in body: line is its 1-based line number in the whole
// body, text is the item's trimmed content. found is false when heading
// is absent, the section has no checklist items, or every item is ticked.
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
