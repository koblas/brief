package scaffold

import (
	"regexp"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
)

// DropRule is the stable id FinishResult.Dropped's Rule field carries:
// dropped-debt for an entry originally under the configured Open debts
// heading, dropped-entry for one under any of the other three.
type DropRule string

const (
	// DropRuleEntry marks an entry dropped from a non-Open-debts state
	// heading.
	DropRuleEntry DropRule = "dropped-entry"
	// DropRuleDebt marks an entry dropped from the configured Open debts
	// heading.
	DropRuleDebt DropRule = "dropped-debt"
)

// DroppedEntry is one state-file entry Finish's replacement body no longer
// carries: Rule classifies it, Heading is the display text of the
// configured heading it was found under (its leading "#" run and one
// space stripped — "## Traps" becomes "Traps"), Line is its 1-based line
// number in the old state file, Tag is the entry's own trailing
// "(<token>)" extracted from Text, "" when the entry carries none, and
// Text is its whitespace-normalized text, never cut.
type DroppedEntry struct {
	Rule    DropRule
	Heading string
	Line    int
	Tag     string
	Text    string
}

// trailingTagRe matches a trailing tag: a "(<token>)" group, the token
// one or more characters excluding whitespace and parens, anchored at the
// end of the string.
var trailingTagRe = regexp.MustCompile(`\(([^\s()]+)\)$`)

// trailingTag returns text's trailing tag: the token inside a trailing
// "(<token>)", the token non-empty and free of whitespace. It
// returns "" when text carries no such trailing group — no parens, an
// empty pair, a token containing whitespace, or parens that are not the
// text's own tail.
func trailingTag(text string) string {
	m := trailingTagRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}

	return m[1]
}

// normalizeEntryText collapses raw's whitespace: a carriage return
// stripped first, then every run of whitespace (including a line join)
// collapsed to a single space, the result trimmed. Two entries with the
// same normalized text are the same identity regardless of wrapping or
// indentation.
func normalizeEntryText(raw string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(raw, "\r", "")), " ")
}

// headingDisplay returns heading's display text: its leading "#" run and
// the one space after it stripped ("## Traps" becomes "Traps").
func headingDisplay(heading string) string {
	return strings.TrimSpace(strings.TrimLeft(heading, "#"))
}

// dropRuleFor classifies heading — one of the four configured state
// headings a dropped entry was found under — against openDebts
// (cfg.StateHeadings.OpenDebts): DropRuleDebt when they are the same
// configured heading, DropRuleEntry otherwise. The comparison is against
// the configured heading text, never the display text headingDisplay
// derives from it.
func dropRuleFor(heading, openDebts string) DropRule {
	if heading == openDebts {
		return DropRuleDebt
	}

	return DropRuleEntry
}

// scannableHeadings filters headings.Ordered() to the heading strings a scan
// may use: non-empty and appearing exactly once. An empty
// value would match markdown.Section's first-blank-line fallback and a
// value shared by more than one configured field would scan its section
// twice; both are excluded wholesale, from every position they occupy, not
// just narrowed to their first occurrence.
func scannableHeadings(headings config.StateHeadings) []string {
	all := headings.Ordered()

	counts := make(map[string]int, len(all))
	for _, h := range all {
		counts[h]++
	}

	out := make([]string, 0, len(all))

	for _, h := range all {
		if h == "" || counts[h] > 1 {
			continue
		}

		out = append(out, h)
	}

	return out
}

// droppedEntries computes FinishResult.Dropped: oldBody is the state file
// bytes already on disk, newBody is Finish's incoming state argument, and
// headings is cfg.StateHeadings. An entry present under one of
// scannableHeadings(headings)'s sections in oldBody and absent, by
// normalized-text identity, from the pooled set of newBody's own sections
// is a drop; an empty or duplicated configured heading contributes no
// entries, from either body.
//
// The diff is pooled across the scannable headings, both for old and for
// new, matched as one multiset: a text appearing more often in oldBody
// than in the pooled newBody set has its surplus occurrences — the last
// ones in old-file order — reported as drops, so an entry moved between
// two state headings is never a drop, and a text duplicated in oldBody but
// dropped only some of its occurrences is reported for its later
// occurrence. The returned slice is never nil, and is already in old-file
// line order.
func droppedEntries(oldBody, newBody []byte, headings config.StateHeadings) []DroppedEntry {
	type occurrence struct {
		heading string
		entry   markdown.Entry
		text    string
	}

	var old []occurrence

	for _, h := range scannableHeadings(headings) {
		for _, e := range markdown.Entries(string(oldBody), h) {
			old = append(old, occurrence{heading: h, entry: e, text: normalizeEntryText(e.Text)})
		}
	}

	sort.Slice(old, func(i, j int) bool { return old[i].entry.Line < old[j].entry.Line })

	newCounts := map[string]int{}

	for _, h := range scannableHeadings(headings) {
		for _, e := range markdown.Entries(string(newBody), h) {
			newCounts[normalizeEntryText(e.Text)]++
		}
	}

	groups := map[string][]int{}
	for i, o := range old {
		groups[o.text] = append(groups[o.text], i)
	}

	dropped := map[int]bool{}

	for text, idxs := range groups {
		surplus := len(idxs) - newCounts[text]
		if surplus <= 0 {
			continue
		}

		for _, i := range idxs[len(idxs)-surplus:] {
			dropped[i] = true
		}
	}

	out := make([]DroppedEntry, 0, len(dropped))

	for i, o := range old {
		if !dropped[i] {
			continue
		}

		out = append(out, DroppedEntry{
			Rule:    dropRuleFor(o.heading, headings.OpenDebts),
			Heading: headingDisplay(o.heading),
			Line:    o.entry.Line,
			Tag:     trailingTag(o.text),
			Text:    o.text,
		})
	}

	return out
}
