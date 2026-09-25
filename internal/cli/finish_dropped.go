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

// finishDroppedJSON is one finishDocument "dropped_entries" element:
// severity, rule, path, line, detail, heading, tag and text, in that key
// order. Path is res.StatePath verbatim, repeated per element. Detail is
// dropDetail(d), the same string the text-mode WARN row renders after its
// location. Tag is nil (JSON null) for an untagged entry, else a plain
// string — never wrapped in the text row's "tagged <token>" prose. Text is
// the entry's full normalized text, never cut.
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

// finishDroppedEntries maps dropped to finishDocument's own "dropped_entries"
// array, path repeated on every element: a sized, non-nil slice so zero
// drops encode as "[]" rather than "null" — mirroring checkFeatures' own
// empty-vs-nil discipline.
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

// writeDroppedRows writes one WARN row per entry in dropped to w, in the
// order dropped already carries (old-file line order): "<severity>
// <stateRel>:<line>  <detail>\n", detail from dropDetail. It returns the
// first write error, stopping before any later row — the state file is
// already replaced by the time a caller reaches this point, so a partial
// write here must not be papered over by going on to print a success
// line that claims the drop report reached the user.
func writeDroppedRows(w io.Writer, dropped []scaffold.DroppedEntry, stateRel string) error {
	for _, d := range dropped {
		if _, err := fmt.Fprintf(w, "%s  %s:%d  %s\n", scaffold.SeverityWarn, stateRel, d.Line, dropDetail(d)); err != nil {
			return fmt.Errorf("write dropped-entry row: %w", err)
		}
	}

	return nil
}
