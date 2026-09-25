package cli

import (
	"fmt"
	"io"

	"github.com/koblas/brief/internal/scaffold"
)

// dropExcerptRunes is the rune length a dropped entry's text is cut to
// before "…" is appended.
const dropExcerptRunes = 80

// dropExcerpt returns text cut to dropExcerptRunes runes, with a trailing
// "…" appended when it was cut; text at or under the limit is returned
// unchanged. Cutting counts runes, never bytes, so multibyte text is never
// split mid-rune.
func dropExcerpt(text string) string {
	runes := []rune(text)
	if len(runes) <= dropExcerptRunes {
		return text
	}

	return string(runes[:dropExcerptRunes]) + "…"
}

// dropDetail renders one scaffold.DroppedEntry's row detail — the part
// after the location: "dropped from <heading>, tagged <tag>: <excerpt>",
// or "dropped from <heading>, untagged: <excerpt>" when d.Tag is "".
func dropDetail(d scaffold.DroppedEntry) string {
	tag := "untagged"
	if d.Tag != "" {
		tag = "tagged " + d.Tag
	}

	return fmt.Sprintf("dropped from %s, %s: %s", d.Heading, tag, dropExcerpt(d.Text))
}

// finishDroppedJSON is one finishDocument "dropped_entries" element. Detail
// is dropDetail(d), the same string the text-mode WARN row renders. Tag is
// nil for an untagged entry. Text is the entry's full text, never cut.
type finishDroppedJSON struct {
	Severity string  `json:"severity"`
	Rule     string  `json:"rule"`
	Path     string  `json:"path"`
	Line     int     `json:"line"`
	Detail   string  `json:"detail"`
	Heading  string  `json:"heading"`
	Tag      *string `json:"tag"`
	Text     string  `json:"text"`
}

// finishDroppedEntries maps dropped to finishDocument's "dropped_entries"
// array, path repeated on every element: a sized, non-nil slice so zero
// drops encode as "[]" rather than "null".
func finishDroppedEntries(dropped []scaffold.DroppedEntry, path string) []finishDroppedJSON {
	out := make([]finishDroppedJSON, 0, len(dropped))

	for _, d := range dropped {
		var tag *string
		if d.Tag != "" {
			t := d.Tag
			tag = &t
		}

		out = append(out, finishDroppedJSON{
			Severity: string(scaffold.SeverityWarn),
			Rule:     string(d.Rule),
			Path:     path,
			Line:     d.Line,
			Detail:   dropDetail(d),
			Heading:  d.Heading,
			Tag:      tag,
			Text:     d.Text,
		})
	}

	return out
}

// dropCountSuffix renders the parenthetical the stderr success line's
// "replaced <path>" clause gains when Finish reports one or more drops:
// "" when n is 0, else " (dropped 1 entry, listed on stdout)" (singular)
// or " (dropped N entries, listed on stdout)".
func dropCountSuffix(n int) string {
	if n == 0 {
		return ""
	}

	noun := "entries"
	if n == 1 {
		noun = "entry"
	}

	return fmt.Sprintf(" (dropped %d %s, listed on stdout)", n, noun)
}

// writeDroppedRows writes one WARN row per entry in dropped to w, in
// dropped's own order: "<severity> <stateRel>:<line>  <detail>\n". It
// stops at the first write failure and returns how many rows landed plus
// the raw write error, unwrapped, so runFinish can report exactly what
// the writer refused instead of claiming the drop report reached the user.
func writeDroppedRows(w io.Writer, dropped []scaffold.DroppedEntry, stateRel string) (int, error) {
	for i, d := range dropped {
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s\n", scaffold.SeverityWarn, stateRel, d.Line, dropDetail(d)); err != nil {
			return i, fmt.Errorf("%w", err)
		}
	}

	return len(dropped), nil
}

// droppedWriteError is runFinish's error when writeDroppedRows fails
// partway through the WARN rows, after the state file has already been
// replaced. It carries scaffold.ErrPartialWrite so filesChangedFor still
// reports true even though writeDroppedRows runs only from the text
// branch today.
type droppedWriteError struct {
	err error
}

func (e *droppedWriteError) Error() string   { return e.err.Error() }
func (e *droppedWriteError) Unwrap() []error { return []error{e.err, scaffold.ErrPartialWrite} }

// newDroppedWriteError builds droppedWriteError's message: "<feature>
// <step> done, but the dropped-entries report failed after <written> of
// <total> rows: <cause>; compare <stateRel> with its previous version to
// see what was removed — a retry reports nothing". cause is the raw error
// writeDroppedRows returned.
func newDroppedWriteError(feature, step string, written, total int, stateRel string, cause error) error {
	msg := fmt.Errorf("%s %s done, but the dropped-entries report failed after %d of %d rows: %w; "+
		"compare %s with its previous version to see what was removed — a retry reports nothing",
		feature, step, written, total, cause, stateRel)

	return &droppedWriteError{err: msg}
}
