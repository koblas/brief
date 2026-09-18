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

// startUsage is "brief start"'s help text.
const startUsage = `Usage:
  brief start <feature>

Prints the next open step's id, title, acceptance criteria and checklist,
and the decisions and constraints inherited from the feature's state
file. brief start reads; it never writes.
`

// runStart implements "brief start <feature>".
func runStart(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, startUsage)
			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief start: %s; run 'brief start <feature>'", err))
	}

	rest := fs.Args()

	switch {
	case len(rest) == 0:
		return usageError(stderr, "brief start: no feature given; run 'brief start <feature>'")
	case len(rest) > 1:
		return usageError(stderr, "brief start: too many arguments; run 'brief start <feature>'")
	}

	feature := rest[0]

	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return renderRefusal(stderr, "start", err)
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(ctx, feature)
	if err != nil {
		return renderRefusal(stderr, "start", err)
	}

	if err := assemble.RenderText(stdout, brief); err != nil {
		return fmt.Errorf("brief start: %w", err)
	}

	return nil
}
