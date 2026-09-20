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
an unfinished dependency. brief status reads; it never writes.
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

	if err := assemble.RenderStatusText(stdout, rows); err != nil {
		return fmt.Errorf("brief status: %w", err)
	}

	return nil
}
