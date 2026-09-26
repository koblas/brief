package conform

import (
	"errors"
	"fmt"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
)

// ErrOverCap is returned when a body measures more lines, by
// markdown.CountLines, than its configured cap.
var ErrOverCap = errors.New("input is over the configured line cap")

// ErrUnterminatedFence is returned when a body opens a fenced code block it
// never closes.
var ErrUnterminatedFence = errors.New("input has an unterminated fence")

// ErrMissingStateHeading is returned when a state body carries no section
// for one of its four configured headings.
var ErrMissingStateHeading = errors.New("state is missing a required section")

// ErrOpenChecklistItem is returned when a checklist section carries an item
// not ticked with "[x]"/"[X]".
var ErrOpenChecklistItem = errors.New("checklist item is not ticked")

// Violation is one fault a predicate in this package found in a body: the
// 1-based line it concerns (0 for a whole-body fault), what is wrong, the
// one-line remedy, and the sentinel Err wraps for errors.Is. It carries no
// path — the caller attaches the file it came from.
type Violation struct {
	Line    int
	Problem string
	Fix     string
	Err     error
}

// OverCap reports a Violation when body measures more lines, by
// markdown.CountLines, than limit — a body of exactly limit lines is not a
// violation. label names the body ("handoff" or "state") in the returned
// copy, and doubles as the .brief.yaml key stem ("<label>-cap-lines") the
// Fix names.
func OverCap(body []byte, label string, limit int) *Violation {
	n := markdown.CountLines(string(body))
	if n <= limit {
		return nil
	}

	return &Violation{
		Problem: fmt.Sprintf("%s is %d lines, over the cap of %d", label, n, limit),
		Fix: fmt.Sprintf("cut the %s to %d lines or fewer, or raise %s-cap-lines in .brief.yaml, and retry",
			label, limit, label),
		Err: ErrOverCap,
	}
}

// UnterminatedFence reports a Violation when body opens a fenced code block
// it never closes, naming label in the returned copy and the fence's
// opening line as Violation.Line.
func UnterminatedFence(body []byte, label string) *Violation {
	line, delim, unterminated := markdown.UnterminatedFence(string(body))
	if !unterminated {
		return nil
	}

	return &Violation{
		Line:    line,
		Problem: fmt.Sprintf("%s has an unclosed %s fence", label, delim),
		Fix:     "close the fence, or remove the unmatched delimiter, and retry",
		Err:     ErrUnterminatedFence,
	}
}

// MissingHeading reports a Violation for the first of headings.Ordered()'s
// four entries body carries no section for — markdown.Section's found
// return, never section content, so a section with nothing under it is not
// a violation. It returns nil when every configured heading is present.
func MissingHeading(body []byte, label string, headings config.StateHeadings) *Violation {
	for _, heading := range headings.Ordered() {
		if _, found := markdown.Section(string(body), heading); found {
			continue
		}

		return &Violation{
			Problem: fmt.Sprintf("%s is missing the %q section", label, heading),
			Fix:     fmt.Sprintf("add a %q heading to the %s body — an empty section is valid — and retry", heading, label),
			Err:     ErrMissingStateHeading,
		}
	}

	return nil
}

// OpenChecklistItem reports a Violation for the first item in body's
// checklist section — the section under heading — not ticked with
// "[x]"/"[X]" (markdown.FirstUnchecked). Violation.Line is the item's
// 1-based line number in the whole of body. A checklist with no items, or a
// step file with no checklist heading at all, is never a violation.
func OpenChecklistItem(body []byte, heading string) *Violation {
	line, text, found := markdown.FirstUnchecked(string(body), heading)
	if !found {
		return nil
	}

	problem := "checklist item is not ticked"
	if text != "" {
		problem = fmt.Sprintf("checklist item %q is not ticked", text)
	}

	return &Violation{
		Line:    line,
		Problem: problem,
		Fix:     "tick it with [x] once it is done, or remove it, and retry",
		Err:     ErrOpenChecklistItem,
	}
}

// ChecklistItemCount returns the number of checklist items — ticked or
// not — in body's section under heading (markdown.CountChecklistItems).
// found is false when heading is absent from body. It carries no
// Violation: start and finish attach different copy to the same count.
func ChecklistItemCount(body []byte, heading string) (int, bool) {
	return markdown.CountChecklistItems(string(body), heading)
}
