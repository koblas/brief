package scaffold

import (
	"regexp"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
)

// DropRule is FinishResult.Dropped's Rule field: dropped-debt for an entry
// originally under the configured Open debts heading, dropped-entry for
// one under any of the other three.
type DropRule string

const (
	// DropRuleEntry marks an entry dropped from a non-Open-debts state
	// heading.
	DropRuleEntry DropRule = "dropped-entry"
	// DropRuleDebt marks an entry dropped from the configured Open debts
	// heading.
	DropRuleDebt DropRule = "dropped-debt"
)

// Severity is the urgency a DroppedEntry carries.
type Severity string

// SeverityWarn is the only Severity value a DroppedEntry ever carries:
// dropping an entry never refuses the write.
const SeverityWarn Severity = "WARN"

// DroppedEntry is one state-file entry Finish's replacement body no longer
// carries: Rule classifies it, Heading is the display text of the
// configured heading it was found under, Line is its 1-based line number
// in the old state file, Tag is its trailing "(<token>)" ("" if none), and
// Text is its whitespace-normalized text.
type DroppedEntry struct {
	Rule    DropRule
	Heading string
	Line    int
	Tag     string
	Text    string
}

// trailingTagRe matches a trailing "(<token>)" group at the end of a string.
var trailingTagRe = regexp.MustCompile(`\(([^\s()]+)\)$`)

// trailingTag returns text's trailing "(<token>)" tag, or "" when text
// carries no such trailing group.
func trailingTag(text string) string {
	m := trailingTagRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}

	return m[1]
}

// normalizeEntryText collapses raw's whitespace (stripping "\r" first) to
// single spaces, trimmed, so two entries differing only in wrapping or
// indentation compare equal.
func normalizeEntryText(raw string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(raw, "\r", "")), " ")
}

// headingDisplay strips heading's leading "#" run and following space
// ("## Traps" becomes "Traps").
func headingDisplay(heading string) string {
	return strings.TrimSpace(strings.TrimLeft(heading, "#"))
}

// dropRuleFor classifies heading against openDebts
// (cfg.StateHeadings.OpenDebts): DropRuleDebt when equal, DropRuleEntry
// otherwise.
func dropRuleFor(heading, openDebts string) DropRule {
	if heading == openDebts {
		return DropRuleDebt
	}

	return DropRuleEntry
}

// scannableHeadings filters headings.Ordered() to heading strings a scan
// may use: non-empty and appearing exactly once, excluded wholesale rather
// than narrowed to their first occurrence.
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

// occurrence is one entry found in a body, pooled with the configured
// heading it is attributed to — since two configured headings can nest,
// more than one heading's own scan can reach the same physical entry line.
type occurrence struct {
	heading string
	entry   markdown.Entry
	text    string
}

// poolOccurrences scans every heading in scannable's own section of body
// for entries and pools them into one line-ordered list, each physical
// line kept once.
func poolOccurrences(body []byte, scannable []string) []occurrence {
	text := string(body)

	headingLine := make(map[string]int, len(scannable))
	for _, h := range scannable {
		if line, ok := markdown.HeadingLine(text, h); ok {
			headingLine[h] = line
		}
	}

	return sortedByLine(attributeByLine(text, scannable, headingLine))
}

// attributeByLine scans every heading in scannable for entries under its
// own section of text and attributes each physical line to at most one
// heading: when more than one heading's own scan reaches the same line
// (nested sections, or a heading with no leading "#" that never
// terminates its own section), the line goes to whichever of those
// headings has the greatest headingLine[...] — the most specific heading
// that genuinely encloses it, not merely the nearest by line number.
func attributeByLine(text string, scannable []string, headingLine map[string]int) map[int]occurrence {
	byLine := make(map[int]occurrence)

	for _, h := range scannable {
		for _, e := range markdown.Entries(text, h) {
			if existing, found := byLine[e.Line]; found && headingLine[existing.heading] >= headingLine[h] {
				continue
			}

			byLine[e.Line] = occurrence{
				heading: h,
				entry:   e,
				text:    normalizeEntryText(e.Text),
			}
		}
	}

	return byLine
}

// sortedByLine returns byLine's occurrences ordered by entry.Line.
func sortedByLine(byLine map[int]occurrence) []occurrence {
	out := make([]occurrence, 0, len(byLine))
	for _, o := range byLine {
		out = append(out, o)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].entry.Line < out[j].entry.Line })

	return out
}

// countByText tallies occurrences by normalized text.
func countByText(occurrences []occurrence) map[string]int {
	counts := make(map[string]int, len(occurrences))
	for _, o := range occurrences {
		counts[o.text]++
	}

	return counts
}

// surplusIndices returns, for old (pooled and line-ordered), the indices
// the multiset rule marks dropped: for each distinct text, the last
// occurrences (by index) in excess of newCounts[text].
func surplusIndices(old []occurrence, newCounts map[string]int) map[int]bool {
	groups := make(map[string][]int)
	for i, o := range old {
		groups[o.text] = append(groups[o.text], i)
	}

	dropped := make(map[int]bool)

	for text, idxs := range groups {
		surplus := len(idxs) - newCounts[text]
		if surplus <= 0 {
			continue
		}

		for _, i := range idxs[len(idxs)-surplus:] {
			dropped[i] = true
		}
	}

	return dropped
}

// projectDropped renders old's surplus occurrences (per dropped) as
// DroppedEntry, in old's own line order.
func projectDropped(old []occurrence, dropped map[int]bool, openDebts string) []DroppedEntry {
	out := make([]DroppedEntry, 0, len(dropped))

	for i, o := range old {
		if !dropped[i] {
			continue
		}

		out = append(out, DroppedEntry{
			Rule:    dropRuleFor(o.heading, openDebts),
			Heading: headingDisplay(o.heading),
			Line:    o.entry.Line,
			Tag:     trailingTag(o.text),
			Text:    o.text,
		})
	}

	return out
}

// droppedEntries computes FinishResult.Dropped: an entry present under one
// of scannableHeadings(headings)'s sections in oldBody and absent, by
// normalized-text identity, from the pooled set of newBody's own sections
// is a drop. The diff is a multiset across the scannable headings, so an
// entry moved between two state headings is never a drop; the returned
// slice is never nil, and is in old-file line order.
func droppedEntries(oldBody, newBody []byte, headings config.StateHeadings) []DroppedEntry {
	scannable := scannableHeadings(headings)

	old := poolOccurrences(oldBody, scannable)
	newCounts := countByText(poolOccurrences(newBody, scannable))

	return projectDropped(old, surplusIndices(old, newCounts), headings.OpenDebts)
}
