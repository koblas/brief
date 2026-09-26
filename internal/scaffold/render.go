package scaffold

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
)

// identRune is the set of runes id-boundary matching treats as part of an
// identifier.
const identRune = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"

// checklistItemRe matches a checklist item line: "- [ ]" or "- [x]",
// allowing leading whitespace.
var checklistItemRe = regexp.MustCompile(`^\s*- \[[ x]\]`)

// specificationSkeleton renders a new feature's specification.md: a title
// line, a blank line, and the configured progress heading, empty.
func specificationSkeleton(cfg config.Config, name string) string {
	return fmt.Sprintf("# %s\n\n%s\n", name, cfg.ProgressHeading)
}

// stateSkeleton renders a new feature's state file: the four configured
// state headings, blank-line separated, with no title line — Finish
// replaces the whole state body, and a title would be silently dropped.
func stateSkeleton(cfg config.Config) string {
	return strings.Join(cfg.StateHeadings.Ordered(), "\n\n") + "\n"
}

// stepSkeleton renders a new step file: YAML frontmatter (id, status: open,
// an empty depends-on list), a title heading equal to id, and the
// configured acceptance and checklist headings, both empty.
func stepSkeleton(cfg config.Config, id string) string {
	return fmt.Sprintf(
		"---\nid: %s\nstatus: open\ndepends-on: []\n---\n\n# %s\n\n%s\n\n%s\n",
		id, id, cfg.AcceptanceHeading, cfg.ChecklistHeading,
	)
}

// progressEntry renders the progress-list line for a newly scaffolded
// step: the id alone. A reader matching an entry by id tolerates a
// trailing ": <title>" a human added.
func progressEntry(id string) string {
	return "- [ ] " + id
}

// progressSection locates heading within lines (exact right-trimmed
// equality) and the end of the section it introduces: the index of the
// next line starting with "#", or len(lines) if none. ok is false when no
// line equals heading.
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
// equals heading exactly, inserting after the last existing checklist item
// in that section (progressSection). A section with no checklist item yet
// gets entry on a fresh line after the heading, preceded by a blank line.
// It returns ErrNoProgressHeading, with body untouched, when no line
// equals heading.
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
// marker is replaced only within the span checklistItemRe itself matched,
// so a title text containing a literal "[ ]" is never mistaken for the
// marker. It returns ErrNoProgressHeading when no line equals heading, and
// ErrNoProgressEntry when the section has no item naming id.
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

// tickMarker flips line's leading "- [ ]" marker to "- [x]", operating
// only within the span checklistItemRe matched. line must already satisfy
// checklistItemRe; a line whose marker is already "[x]" is returned
// unchanged.
func tickMarker(line string) string {
	loc := checklistItemRe.FindStringIndex(line)
	marker, rest := line[:loc[1]], line[loc[1]:]

	return strings.Replace(marker, "[ ]", "[x]", 1) + rest
}

// progressItemNames reports whether line — already matched by
// checklistItemRe — names id: its text after the marker begins with id,
// followed by end of line or a rune outside identRune.
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
