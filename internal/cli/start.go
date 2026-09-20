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
file. A feature whose steps are all done, or that has no step files yet,
prints nothing and says so on stderr instead, still exiting 0.
brief start reads; it never writes.
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

	if brief.Step == nil {
		featureDir := filepath.Join(root, cfg.FeatureDirectory, feature)

		if brief.Done+brief.Open > 0 {
			fmt.Fprintf(stderr, "brief start: %s: feature is complete, %d of %d steps done; run 'brief new step %s' to add the next one\n",
				featureDir, brief.Done, brief.Done+brief.Open, feature)
		} else {
			fmt.Fprintf(stderr, "brief start: %s: no step files yet; run 'brief new step %s' to scaffold the first one\n",
				featureDir, feature)
		}

		return nil
	}

	if err := assemble.RenderText(stdout, brief); err != nil {
		return fmt.Errorf("brief start: %w", err)
	}

	return nil
}
