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
// straight into its own context: a status line, "# <title>", then the
// step's sections and every inherited state-file section under its
// configured heading. RenderText writes nothing when b.Step is nil; the
// caller decides what to say about a feature with no open step.
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
// newline, per Brief's json tags, with no field omitted. Unlike RenderText
// it always writes a document, including when b.Step is nil. It encodes
// into a buffer first, so w sees either the complete document or nothing.
func RenderJSON(w io.Writer, b Brief) error {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	// '<' and '&' in a section body are left unescaped: the payload is not HTML.
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
// columns aligned with text/tabwriter. It writes nothing at all, not even
// the header, when rows is empty — an empty table still has a header, but
// "no rows" must not look like one exists.
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

// statusRowCells renders row's DONE, BLOCKED and NEXT cells: "-", "-" and
// "(malformed, see below)" when row.Problem is set, since no count was
// measured.
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
		// A freshly scaffolded step file opens with "# <id>" as its only
		// heading; this keeps that common case from doubling the id.
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

// RenderFindings writes groups to w: one blank line between feature groups,
// each opening with "<name>  (in flight)" or "<name>  (complete)", then
// each finding as "  <SEVERITY>  <path>[:<line>]  <detail>". It carries no
// Fix and renders Path verbatim; grouping, severity and ordering are all
// decided upstream.
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
