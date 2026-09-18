package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
)

// newFeatureUsage is "brief new feature"'s help text.
const newFeatureUsage = `Usage:
  brief new feature <name>

Scaffolds docs/specifications/<name> (or the configured feature directory)
with an empty specification skeleton and an empty state file.
`

// runNew dispatches "brief new <type> ...".
func runNew(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief new: no type given; expected one of: feature")
	}

	switch args[0] {
	case "feature":
		return runNewFeature(ctx, wd, args[1:], stdout, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("brief new: unknown type %q; expected one of: feature", args[0]))
	}
}

// runNewFeature implements "brief new feature <name>".
func runNewFeature(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("new feature", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, newFeatureUsage)
			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief new feature: %s; run 'brief new feature <name>'", err))
	}

	rest := fs.Args()

	switch {
	case len(rest) == 0:
		return usageError(stderr, "brief new feature: no name given; run 'brief new feature <name>'")
	case len(rest) > 1:
		return usageError(stderr, "brief new feature: too many arguments; run 'brief new feature <name>'")
	}

	name := rest[0]

	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return renderRefusal(stderr, "new feature", err)
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	srv := scaffold.NewServer(cfg, root)

	path, err := srv.NewFeature(ctx, name)
	if err != nil {
		return renderRefusal(stderr, "new feature", err)
	}

	rel, err := filepath.Rel(wd, path)
	if err != nil {
		rel = path
	}

	fmt.Fprintln(stdout, rel)

	return nil
}
