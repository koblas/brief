package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
)

// statusUsage is "brief status"'s help text.
const statusUsage = `Usage:
  brief status

Prints one line per feature: name, steps done over total, the next open
step's id (or "-" when there is none), and how many steps are blocked on
an unfinished dependency. A feature whose step files cannot be read or
parsed prints "<name> ! ! !" and names the reason on stderr, one line per
malformed feature — brief status still exits 0, because check (not
status) is where that becomes a failure. A repository with no features
prints nothing and exits 0, with one line on stderr saying so. brief
status reads; it never writes.
`

// runStatus implements "brief status".
func runStatus(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, statusUsage)
			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief status: %s; run 'brief status'", err))
	}

	if len(fs.Args()) > 0 {
		return usageError(stderr, "brief status: too many arguments; run 'brief status'")
	}

	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return renderRefusal(stderr, "status", err)
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(ctx)
	if err != nil {
		return renderRefusal(stderr, "status", err)
	}

	if len(rows) == 0 {
		fmt.Fprintf(stderr, "brief status: no features found in %s; run 'brief new feature <name>' to create one\n", cfg.FeatureDirectory)

		return nil
	}

	// A malformed feature always yields a row (never dropped, per
	// SCENARIO-11), so this loop and the len(rows) == 0 notice above are
	// mutually exclusive by construction. renderRefusal is not used here:
	// it returns err for ExitCode to classify, and a malformed feature
	// must not drive a non-zero exit — R18 gives that failing role to
	// check, not status.
	for _, row := range rows {
		if row.Problem == nil {
			continue
		}

		fmt.Fprintf(stderr, "brief status: %s: %s; %s\n",
			row.Problem.Path, flattenOneLine(row.Problem.Detail), flattenOneLine(row.Problem.Fix))
	}

	if err := assemble.RenderStatusText(stdout, rows); err != nil {
		return fmt.Errorf("brief status: %w", err)
	}

	return nil
}
