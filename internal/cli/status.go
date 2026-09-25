package cli

import (
	"context"
	"fmt"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// statusLong is "brief status"'s help prose.
var statusLong = `Prints a table of every feature: name, steps done over total, how many
steps are blocked on an unfinished dependency, and the next open step's id
and title — "(complete)" once every step is done, "-" when there are no
step files at all. A feature whose specification is missing or carries no
progress heading, whose state file is missing or unreadable, or whose step
files cannot be read or parsed — the same faults brief start itself refuses
over — prints "-  -  (malformed, see below)" in its row; stderr names the
file and the reason, one line per malformed feature, then a summary line
counting features in progress, complete and malformed — brief status
still exits 0, because check (not status) is where that becomes a
failure. A repository with no features prints nothing and exits 0, with
one line on stderr saying so. brief status reads; it never writes.

` + jsonFieldsParagraph("features") + " " + jsonScriptHint

// statusInvocation is the invocation string every "brief status" usage
// error names as how to fix it.
const statusInvocation = "brief status"

// statusDocument is status's --json success document: the common header,
// then one row per feature, no "data" wrapper.
type statusDocument struct {
	jsonHeader

	Features []statusFeatureJSON `json:"features"`
}

// statusFeatureJSON is one row of statusDocument's "features" array. Done,
// Total and Blocked are nil exactly when Problem is non-nil, so a
// malformed row reports no counts rather than zeroes that would look
// measured. Next is nil when the row has no open step.
type statusFeatureJSON struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Done     *int               `json:"done"`
	Total    *int               `json:"total"`
	Blocked  *int               `json:"blocked"`
	Complete bool               `json:"complete"`
	Next     *statusNextJSON    `json:"next"`
	Problem  *statusProblemJSON `json:"problem"`
}

// statusNextJSON is a statusFeatureJSON row's "next" member: id, title and
// path exactly as assemble.NextStep carries them — title is raw, never
// flattened for a tabwriter column the way the text table's NEXT cell is.
type statusNextJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// statusProblemJSON is a statusFeatureJSON row's "problem" member: path and
// detail/fix exactly as assemble.Problem carries them, raw and never
// flattened. Line is assemble.Problem.Line, non-nil only when it is greater
// than 0 — a whole-file fault carries no line number.
type statusProblemJSON struct {
	Path   string `json:"path"`
	Line   *int   `json:"line"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// statusFeatures maps rows to statusDocument's "features" array: a sized,
// non-nil slice so zero rows encode as "[]" rather than "null".
func statusFeatures(rows []assemble.FeatureStatus) []statusFeatureJSON {
	out := make([]statusFeatureJSON, 0, len(rows))

	for _, row := range rows {
		f := statusFeatureJSON{Name: row.Name, Path: row.Path, Complete: row.Complete()}

		if row.Problem == nil {
			done, total, blocked := row.Done, row.Total, row.Blocked
			f.Done, f.Total, f.Blocked = &done, &total, &blocked
		} else {
			f.Problem = &statusProblemJSON{Path: row.Problem.Path, Detail: row.Problem.Detail, Fix: row.Problem.Fix}
			if row.Problem.Line > 0 {
				line := row.Problem.Line
				f.Problem.Line = &line
			}
		}

		if row.Next != nil {
			f.Next = &statusNextJSON{ID: row.Next.ID, Title: row.Next.Title, Path: row.Next.Path}
		}

		out = append(out, f)
	}

	return out
}

// runStatus implements "brief status"; rest is its positional arguments,
// flags already parsed away, and must be empty.
func runStatus(ctx context.Context, wd string, rest []string, out reporter, rootFS rwfs.FS) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief status: too many arguments; run '%s'", statusInvocation))
	}

	cfg, root, err := resolveRoot(rootFS, wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root, assemble.WithFS(rootFS))

	rows, err := srv.Status(ctx)
	if err != nil {
		return out.refusal(err)
	}

	// A successful --json run writes zero stderr bytes, so this branch
	// runs before the zero-rows and malformed-row cases below can write.
	if out.json {
		doc := statusDocument{jsonHeader: out.successHeader(), Features: statusFeatures(rows)}

		return out.document(doc)
	}

	if len(rows) == 0 {
		fmt.Fprintf(out.stderr, "brief status: no features found in %s; run 'brief new feature <name>' to create one\n", cfg.FeatureDirectory)

		return nil
	}

	if err := assemble.RenderStatusText(out.stdout, rows); err != nil {
		return fmt.Errorf("brief status: %w", err)
	}

	// A malformed feature always yields a row, never dropped, so this loop
	// and the len(rows) == 0 notice above are mutually exclusive by
	// construction. out.refusal is not used here: it returns err for
	// ExitCode to classify, and a malformed feature must not drive a
	// non-zero exit from status — that failing role belongs to check.
	for _, row := range rows {
		if row.Problem == nil {
			continue
		}

		location := displayPath(wd, row.Problem.Path)
		if row.Problem.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, row.Problem.Line)
		}

		fmt.Fprintf(out.stderr, "brief status: %s: %s: %s; %s\n",
			row.Name, location, flattenOneLine(row.Problem.Detail), flattenOneLine(row.Problem.Fix))
	}

	fmt.Fprintf(out.stderr, "brief status: %s\n", statusSummary(rows))

	return nil
}

// statusSummary renders rows' one-line count summary: "N features: A in
// progress, B complete, C malformed" — "1 feature" singular when N is 1 —
// with every bucket printed even when it is zero. A row whose Problem is
// set counts as malformed; otherwise a row for which
// (assemble.FeatureStatus).Complete reports true counts as complete; every
// other row, including a feature with no step files at all, counts as in
// progress.
func statusSummary(rows []assemble.FeatureStatus) string {
	var inProgress, complete, malformed int

	for _, row := range rows {
		switch {
		case row.Problem != nil:
			malformed++
		case row.Complete():
			complete++
		default:
			inProgress++
		}
	}

	noun := "features"
	if len(rows) == 1 {
		noun = "feature"
	}

	return fmt.Sprintf("%d %s: %d in progress, %d complete, %d malformed", len(rows), noun, inProgress, complete, malformed)
}
