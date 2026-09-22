package assemble

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
)

// RenderText writes b to w as the markdown payload an implementer pastes
// straight into its own context: "<id> — <done> done, <open> open", a
// blank line, "# <title>", then the step's acceptance-criteria and
// checklist sections and every inherited state-file section, each under
// its configured heading verbatim. A section whose body is empty is
// omitted entirely rather than rendered as a bare heading. RenderText
// writes nothing when b.Step is nil; the caller decides what to say about
// a feature with no open step.
func RenderText(w io.Writer, b Brief) error {
	if b.Step == nil {
		return nil
	}

	if _, err := fmt.Fprintf(w, "%s — %d done, %d open\n\n# %s\n", b.Step.ID, b.Done, b.Open, b.Step.Title); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	sections := append([]Section{b.Step.Acceptance, b.Step.Checklist}, b.Inherited...)

	for _, section := range sections {
		if err := writeSection(w, section); err != nil {
			return err
		}
	}

	return nil
}

// RenderJSON writes b to w as one compact JSON document followed by a
// single newline: b.Done, b.Open, b.Step, b.Inherited and b.Shortfalls
// exactly as Brief's json tags define them, with no field omitted
// regardless of its zero value. Unlike RenderText, RenderJSON always
// writes a document — including when b.Step is nil, which marshals to
// "step":null rather than producing empty output — and it never omits a
// Section whose body is empty. '<' and '&' in a section body are left
// unescaped: the payload is not HTML. RenderJSON encodes into a buffer
// before writing to w, so w sees either the complete document or nothing.
func RenderJSON(w io.Writer, b Brief) error {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(b); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	return nil
}

// RenderStatusText writes rows to w as a table for a person to read: a
// header row "FEATURE  DONE  BLOCKED  NEXT", then one row per feature,
// columns aligned with text/tabwriter (2-space padding, the FEATURE/DONE/
// BLOCKED columns padded to the longest value in that column, the NEXT
// column unpadded since it is last). RenderStatusText writes nothing at
// all — not even the header — when rows is empty (R9): an empty table
// still has a header, but "no rows" is a state the header must not claim
// otherwise exists. A row whose Problem is set renders "-", "-" and
// "(malformed, see below)" in DONE, BLOCKED and NEXT — never a real count,
// since none was measured. A complete row (row.Complete()) renders
// "(complete)" in NEXT. Otherwise NEXT is "-" when row.Next is nil, the
// step's id alone when its Title is empty or equal to its ID — a freshly
// scaffolded step file opens with "# <id>" as its only heading, so this
// keeps that common case from doubling the id in the column — or
// "<id>  <title>" (two literal spaces, not a tab) otherwise. A tab or
// newline embedded in a feature name or a step title is flattened to a
// space first, so it cannot corrupt the table's own column alignment.
func RenderStatusText(w io.Writer, rows []FeatureStatus) error {
	if len(rows) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, "FEATURE\tDONE\tBLOCKED\tNEXT"); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	for _, row := range rows {
		done, blocked, next := statusRowCells(row)

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", flattenTabwriterField(row.Name), done, blocked, next); err != nil {
			return fmt.Errorf("assemble: render: %w", err)
		}
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	return nil
}

// statusRowCells renders row's DONE, BLOCKED and NEXT cells per
// RenderStatusText's contract.
func statusRowCells(row FeatureStatus) (string, string, string) {
	if row.Problem != nil {
		return "-", "-", "(malformed, see below)"
	}

	done := fmt.Sprintf("%d/%d", row.Done, row.Total)
	blocked := strconv.Itoa(row.Blocked)

	var next string

	switch {
	case row.Complete():
		next = "(complete)"
	case row.Next == nil:
		next = "-"
	case row.Next.Title == "" || row.Next.Title == row.Next.ID:
		next = flattenTabwriterField(row.Next.ID)
	default:
		next = flattenTabwriterField(row.Next.ID) + "  " + flattenTabwriterField(row.Next.Title)
	}

	return done, blocked, next
}

// flattenTabwriterField replaces every tab and newline in s with a single
// space: either would be read by text/tabwriter as a cell or line
// terminator and corrupt the table's own column alignment.
func flattenTabwriterField(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ").Replace(s)
}

// RenderFindings writes groups to w: one blank line between feature groups
// (none before the first, none after the last), each opening with
// "<name>  (in flight)" or "<name>  (complete)" — from InFlight, never
// recomputed from severity — then each finding as
// "  <SEVERITY>  <path>[:<line>]  <detail>", two-space indented, the
// ":<line>" suffix omitted when Line is 0 (a whole-file finding). It
// carries no Fix: the write-path refusal's "... and retry" copy has no
// meaning in a report about a tree scaffold.Finish was never asked to
// write. RenderFindings renders Path verbatim — the caller relativizes it
// for text mode — and decides nothing about grouping, severity or
// ordering, all decided upstream; a tab or newline in the group's own Name
// or in a finding's Detail is flattened to a single space, the same
// tabwriter-safety stance RenderStatusText takes, even though this output
// is not itself a tabwriter table.
func RenderFindings(w io.Writer, groups []FeatureFindings) error {
	for i, g := range groups {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return fmt.Errorf("assemble: render: %w", err)
			}
		}

		status := "(complete)"
		if g.InFlight {
			status = "(in flight)"
		}

		if _, err := fmt.Fprintf(w, "%s  %s\n", flattenTabwriterField(g.Name), status); err != nil {
			return fmt.Errorf("assemble: render: %w", err)
		}

		for _, f := range g.Findings {
			location := f.Path
			if f.Line > 0 {
				location = fmt.Sprintf("%s:%d", f.Path, f.Line)
			}

			if _, err := fmt.Fprintf(w, "  %s  %s  %s\n", f.Severity, location, flattenTabwriterField(f.Detail)); err != nil {
				return fmt.Errorf("assemble: render: %w", err)
			}
		}
	}

	return nil
}

// writeSection writes a blank line, s.Heading, a blank line and s.Body to
// w, or nothing at all when s.Body is empty.
func writeSection(w io.Writer, s Section) error {
	if strings.TrimSpace(s.Body) == "" {
		return nil
	}

	if _, err := fmt.Fprintf(w, "\n%s\n\n%s\n", s.Heading, s.Body); err != nil {
		return fmt.Errorf("assemble: render: %w", err)
	}

	return nil
}
