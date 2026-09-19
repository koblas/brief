package scaffold

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
)

// identRune is the set of runes id-boundary matching treats as part of an
// identifier: a rune outside this set (including end of line) ends an id,
// so "STEP-10" is never mistaken for a match on "STEP-1".
const identRune = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"

// checklistItemRe matches a checklist item line: "- [ ]" or "- [x]",
// allowing leading whitespace.
var checklistItemRe = regexp.MustCompile(`^\s*- \[[ x]\]`)

// specificationSkeleton renders a new feature's specification.md: a title
// line, a blank line, and the configured progress heading — nothing under
// it, since no scenario work has happened yet.
func specificationSkeleton(cfg config.Config, name string) string {
	return fmt.Sprintf("# %s\n\n%s\n", name, cfg.ProgressHeading)
}

// stateSkeleton renders a new feature's state file: the four configured
// state headings, blank-line separated, with no title line. A title is
// deliberately omitted — Finish replaces the whole state body, and a
// title line would be silently dropped by that replacement.
func stateSkeleton(cfg config.Config) string {
	return strings.Join(cfg.StateHeadings.Ordered(), "\n\n") + "\n"
}

// stepSkeleton renders a new step file: YAML frontmatter (id, status:
// open, an empty depends-on list), a title heading equal to id, the
// configured checklist heading and the configured handoff heading — each
// heading written bare, with nothing under it, since the tool writes no
// prose. The handoff heading, present with nothing under it, is what makes
// a step's handoff anchor "present and empty" from the moment it is
// scaffolded.
func stepSkeleton(cfg config.Config, id string) string {
	return fmt.Sprintf(
		"---\nid: %s\nstatus: open\ndepends-on: []\n---\n\n# %s\n\n%s\n\n%s\n",
		id, id, cfg.ChecklistHeading, cfg.HandoffHeading,
	)
}

// progressEntry renders the progress-list line for a newly scaffolded
// step: the id alone, no title, since the tool writes no prose. A reader
// matching an entry by id tolerates a trailing ": <title>" a human added.
func progressEntry(id string) string {
	return "- [ ] " + id
}

// insertProgressEntry splices entry into body under the first line that
// equals heading exactly (right-trimmed), inserting after the last
// existing checklist item ("- [ ]" or "- [x]", leading whitespace
// allowed) in that section. A section with no checklist item yet gets
// entry on a fresh line immediately after the heading, preceded by a
// blank line — the shape of a freshly scaffolded feature. The section
// ends at the next line starting with "#", or at end of file. Every other
// byte of body, including whether it ends with a trailing newline, is
// preserved.
//
// insertProgressEntry returns ErrNoProgressHeading, with body untouched,
// when no line in body equals heading.
func insertProgressEntry(body, heading, entry string) (string, error) {
	hadTrailingNewline := strings.HasSuffix(body, "\n")
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	headingIdx := -1

	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") == heading {
			headingIdx = i

			break
		}
	}

	if headingIdx == -1 {
		return "", ErrNoProgressHeading
	}

	sectionEnd := len(lines)

	for i := headingIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "#") {
			sectionEnd = i

			break
		}
	}

	lastItem := -1

	for i := headingIdx + 1; i < sectionEnd; i++ {
		if checklistItemRe.MatchString(lines[i]) {
			lastItem = i
		}
	}

	insertAt := lastItem + 1
	toInsert := []string{entry}

	if lastItem == -1 {
		insertAt = headingIdx + 1
		toInsert = []string{"", entry}
	}

	spliced := make([]string, 0, len(lines)+len(toInsert))
	spliced = append(spliced, lines[:insertAt]...)
	spliced = append(spliced, toInsert...)
	spliced = append(spliced, lines[insertAt:]...)

	result := strings.Join(spliced, "\n")
	if hadTrailingNewline {
		result += "\n"
	}

	return result, nil
}

// spliceHandoff replaces the section under heading in body — the
// frontmatter-stripped step body — with handoff's trimmed text, leaving
// every other byte, including the terminating heading's own indentation,
// identical. It uses markdown.SectionRange for both the anchor search and
// the end scan, so a fenced example inside body containing a bare heading
// line is never mistaken for the real anchor. The replacement is
// "\n" + trimmed handoff + "\n", plus one further "\n" when a heading
// follows the anchor, so splicing a body that already holds this exact
// shape reproduces it byte for byte. spliceHandoff reports ok == false,
// body unchanged, when heading is not found.
func spliceHandoff(body []byte, heading string, handoff []byte) ([]byte, bool) {
	start, end, ok := markdown.SectionRange(string(body), heading)
	if !ok {
		return body, false
	}

	replacement := "\n" + strings.Trim(string(handoff), "\n") + "\n"
	if end < len(body) {
		replacement += "\n"
	}

	result := make([]byte, 0, len(body)-(end-start)+len(replacement))
	result = append(result, body[:start]...)
	result = append(result, []byte(replacement)...)
	result = append(result, body[end:]...)

	return result, true
}

// tickProgressEntry flips to "[x]" the first checklist item under heading
// in body whose text names id, leaving every other byte unchanged. It
// reuses insertProgressEntry's section scan — heading matched by exact
// right-trimmed equality, the section ending at the next line starting
// with "#" — deliberately: new step and finish must agree about where the
// progress section ends in the same file. A line already "[x]" is left
// unchanged. An item's text, taken after "- [ ] " or "- [x] ", names id
// when it begins with id followed by end of line or a rune outside
// identRune, so "STEP-10" is never matched when id is "STEP-1".
//
// tickProgressEntry returns ErrNoProgressHeading when no line in body
// equals heading, and ErrNoProgressEntry when the section has no item
// naming id.
func tickProgressEntry(body, heading, id string) (string, error) {
	hadTrailingNewline := strings.HasSuffix(body, "\n")
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	headingIdx := -1

	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") == heading {
			headingIdx = i

			break
		}
	}

	if headingIdx == -1 {
		return "", ErrNoProgressHeading
	}

	sectionEnd := len(lines)

	for i := headingIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "#") {
			sectionEnd = i
			break
		}
	}

	matchIdx := -1

	for i := headingIdx + 1; i < sectionEnd; i++ {
		if checklistItemRe.MatchString(lines[i]) && progressItemNames(lines[i], id) {
			matchIdx = i

			break
		}
	}

	if matchIdx == -1 {
		return "", ErrNoProgressEntry
	}

	lines[matchIdx] = strings.Replace(lines[matchIdx], "[ ]", "[x]", 1)

	result := strings.Join(lines, "\n")
	if hadTrailingNewline {
		result += "\n"
	}

	return result, nil
}

// progressItemNames reports whether line — already matched by
// checklistItemRe — names id: the text after its "- [ ] " or "- [x] "
// marker begins with id, followed by end of line or a rune outside
// identRune.
func progressItemNames(line, id string) bool {
	_, after, found := strings.Cut(line, "]")
	if !found {
		return false
	}

	text := strings.TrimLeft(after, " \t")
	if !strings.HasPrefix(text, id) {
		return false
	}

	rest := text[len(id):]
	if rest == "" {
		return true
	}

	return !strings.ContainsRune(identRune, rune(rest[0]))
}
