package cli

import (
	"context"
	"fmt"

	"github.com/koblas/brief/internal/assemble"
)

// statusLong is "brief status"'s help prose.
const statusLong = `Prints a table of every feature: name, steps done over total, how many
steps are blocked on an unfinished dependency, and the next open step's id
and title — "(complete)" once every step is done, "-" when there are no
step files at all. A feature whose step files cannot be read or parsed
prints "-  -  (malformed, see below)" in its row; stderr names the file
and the reason, one line per malformed feature, then a summary line
counting features in progress, complete and malformed — brief status
still exits 0, because check (not status) is where that becomes a
failure. A repository with no features prints nothing and exits 0, with
one line on stderr saying so. brief status reads; it never writes.`

// statusInvocation is the invocation string every "brief status" usage
// error names as how to fix it.
const statusInvocation = "brief status"

// runStatus implements "brief status"; rest is its positional arguments,
// flags already parsed away, and must be empty.
func runStatus(ctx context.Context, wd string, rest []string, out reporter) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief status: too many arguments; run '%s'", statusInvocation))
	}

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(ctx)
	if err != nil {
		return out.refusal(err)
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

		fmt.Fprintf(out.stderr, "brief status: %s: %s: %s; %s\n",
			row.Name, displayPath(wd, row.Problem.Path), flattenOneLine(row.Problem.Detail), flattenOneLine(row.Problem.Fix))
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
