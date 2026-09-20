package assemble

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// RenderStatusText writes rows to w, one line per feature:
//
//	<name> <done>/<total> <next> <blocked>
//
// Fields are single-0x20-space separated, with no padding and no trailing
// space — padding would make one feature's line depend on the longest
// other feature's name. "-" is substituted for a row whose Next is empty;
// FeatureStatus.Next itself stays empty so a later JSON caller sees an
// empty field rather than the literal string "-". A row whose Problem is
// set renders as "<name> ! ! !" instead — "!" in each of the three
// computed fields, never a single-field marker, so the line still carries
// exactly four single-token fields: "-" already means "no next step" and
// "0/0" already means an empty feature directory, so either would
// fabricate a count that was never measured. RenderStatusText writes no
// header and no legend: rows is already the machine format.
func RenderStatusText(w io.Writer, rows []FeatureStatus) error {
	for _, row := range rows {
		if row.Problem != nil {
			if _, err := fmt.Fprintf(w, "%s ! ! !\n", row.Name); err != nil {
				return fmt.Errorf("assemble: render: %w", err)
			}

			continue
		}

		next := row.Next
		if next == "" {
			next = "-"
		}

		if _, err := fmt.Fprintf(w, "%s %d/%d %s %d\n", row.Name, row.Done, row.Total, next, row.Blocked); err != nil {
			return fmt.Errorf("assemble: render: %w", err)
		}
	}

	return nil
}

// RenderFindings writes findings to w, one per line, in the profile's
// finding shape: "[SEVERITY] <path>:<line> — <problem>". It carries no Fix:
// the write-path refusal's "... and retry" copy has no meaning in a report
// about a tree scaffold.Finish was never asked to write. RenderFindings
// decides nothing about severity or ordering — Check has already decided
// both — it only renders the slice it is given, in the order given.
func RenderFindings(w io.Writer, findings []Finding) error {
	for _, f := range findings {
		if _, err := fmt.Fprintf(w, "[%s] %s:%d — %s\n", f.Severity, f.Path, f.Line, f.Problem); err != nil {
			return fmt.Errorf("assemble: render: %w", err)
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
