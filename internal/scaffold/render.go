package scaffold

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
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
// open, an empty depends-on list), a title heading equal to id, and the
// configured checklist heading, written bare with nothing under it, since
// the tool writes no prose. NewStep writes no handoff file — only Finish
// does, naming it beside the step file (stepfile.CompileHandoff) — so a
// freshly scaffolded step file carries no handoff heading at all.
func stepSkeleton(cfg config.Config, id string) string {
	return fmt.Sprintf(
		"---\nid: %s\nstatus: open\ndepends-on: []\n---\n\n# %s\n\n%s\n",
		id, id, cfg.ChecklistHeading,
	)
}

// progressEntry renders the progress-list line for a newly scaffolded
// step: the id alone, no title, since the tool writes no prose. A reader
// matching an entry by id tolerates a trailing ": <title>" a human added.
func progressEntry(id string) string {
	return "- [ ] " + id
}

// progressSection locates heading within lines — matched by exact
// right-trimmed equality — and the end of the section it introduces: the
// index of the next line starting with "#" (leading whitespace allowed),
// or len(lines) if none. insertProgressEntry and tickProgressEntry both
// call this, so new step and finish agree about where a progress section
// ends in the same file.
//
// progressSection reports ok == false when no line in lines equals
// heading; headingIdx and sectionEnd carry no meaning in that case.
func progressSection(lines []string, heading string) (int, int, bool) {
	headingIdx := -1

	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") == heading {
			headingIdx = i

			break
		}
	}

	if headingIdx == -1 {
		return 0, 0, false
	}

	sectionEnd := len(lines)

	for i := headingIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "#") {
			sectionEnd = i

			break
		}
	}

	return headingIdx, sectionEnd, true
}

// insertProgressEntry splices entry into body under the first line that
// equals heading exactly (right-trimmed), inserting after the last
// existing checklist item ("- [ ]" or "- [x]", leading whitespace
// allowed) in that section (progressSection). A section with no checklist
// item yet gets entry on a fresh line immediately after the heading,
// preceded by a blank line — the shape of a freshly scaffolded feature.
// Every other byte of body, including whether it ends with a trailing
// newline, is preserved.
//
// insertProgressEntry returns ErrNoProgressHeading, with body untouched,
// when no line in body equals heading.
func insertProgressEntry(body, heading, entry string) (string, error) {
	hadTrailingNewline := strings.HasSuffix(body, "\n")
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	headingIdx, sectionEnd, ok := progressSection(lines, heading)
	if !ok {
		return "", ErrNoProgressHeading
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

// tickProgressEntry flips to "[x]" the first checklist item under heading
// in body whose text names id, leaving every other byte unchanged. The
// section is located by progressSection, the same helper
// insertProgressEntry uses, so new step and finish agree about where the
// progress section ends in the same file. A line already "[x]" is left
// unchanged — the marker is replaced within the span checklistItemRe
// itself matched, never by scanning the rest of the line, so an item whose
// own title text happens to contain a literal "[ ]" is never mistaken for
// an unticked marker and rewritten. An item's text, taken after "- [ ] "
// or "- [x] ", names id when it begins with id followed by end of line or
// a rune outside identRune, so "STEP-10" is never matched when id is
// "STEP-1".
//
// tickProgressEntry returns ErrNoProgressHeading when no line in body
// equals heading, and ErrNoProgressEntry when the section has no item
// naming id.
func tickProgressEntry(body, heading, id string) (string, error) {
	hadTrailingNewline := strings.HasSuffix(body, "\n")
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	headingIdx, sectionEnd, ok := progressSection(lines, heading)
	if !ok {
		return "", ErrNoProgressHeading
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

	lines[matchIdx] = tickMarker(lines[matchIdx])

	result := strings.Join(lines, "\n")
	if hadTrailingNewline {
		result += "\n"
	}

	return result, nil
}

// tickMarker flips line's leading "- [ ]" marker to "- [x]", operating only
// within the span checklistItemRe matched at the start of line — never a
// scan of the whole line — so a "[ ]" occurring later in the line's own
// text is never mistaken for the marker and rewritten. line must already
// satisfy checklistItemRe; a line whose marker is already "[x]" is
// returned unchanged.
func tickMarker(line string) string {
	loc := checklistItemRe.FindStringIndex(line)
	marker, rest := line[:loc[1]], line[loc[1]:]

	return strings.Replace(marker, "[ ]", "[x]", 1) + rest
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
