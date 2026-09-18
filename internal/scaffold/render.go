package scaffold

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
)

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
// deliberately omitted — SCENARIO-05 replaces the whole state body on
// "finish", and a title line would be silently dropped by that replacement.
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
		if strings.TrimRight(line, " \t") == heading {
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
