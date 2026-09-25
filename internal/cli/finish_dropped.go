package cli

import (
	"fmt"

	"github.com/koblas/brief/internal/scaffold"
)

// dropSeverity is the fixed severity every dropped-entry row and JSON
// element carries: every drop is reported as a WARN, never an ERROR.
const dropSeverity = "WARN"

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
