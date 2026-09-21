package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
)

// newFeatureLong is "brief new feature"'s help prose.
const newFeatureLong = `Scaffolds docs/specifications/<name> (or the configured feature directory)
with an empty specification skeleton and an empty state file.

A name may not be empty or contain whitespace.`

// newStepLong is "brief new step"'s help prose.
const newStepLong = `Scaffolds the next step file for feature and appends its entry to the
feature's progress list.`

// runNew handles "brief new <type> ..." when type names neither feature
// nor step: nothing at all, or something unknown.
func runNew(args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief new: no type given; expected one of: feature, step")
	}

	return usageError(stderr, fmt.Sprintf("brief new: unknown type %q; expected one of: feature, step", args[0]))
}

// runNewFeature implements "brief new feature <name>"; rest is its
// positional arguments, flags already parsed away.
func runNewFeature(ctx context.Context, wd string, rest []string, stdout, stderr io.Writer) error {
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
		if errors.Is(err, scaffold.ErrInvalidFeatureName) {
			if name == "" {
				return usageError(stderr, "brief new feature: name is empty; run 'brief new feature <name>' with a non-empty name")
			}

			return usageError(stderr, fmt.Sprintf("brief new feature: name %q contains whitespace; run 'brief new feature <name>' with a name containing no whitespace", name))
		}

		return renderRefusal(stderr, "new feature", err)
	}

	rel, err := filepath.Rel(wd, path)
	if err != nil {
		rel = path
	}

	fmt.Fprintln(stdout, rel)

	return nil
}

// runNewStep implements "brief new step <feature>"; rest is its
// positional arguments, flags already parsed away.
func runNewStep(ctx context.Context, wd string, rest []string, stdout, stderr io.Writer) error {
	switch {
	case len(rest) == 0:
		return usageError(stderr, "brief new step: no feature given; run 'brief new step <feature>'")
	case len(rest) > 1:
		return usageError(stderr, "brief new step: too many arguments; run 'brief new step <feature>'")
	}

	feature := rest[0]

	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return renderRefusal(stderr, "new step", err)
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	srv := scaffold.NewServer(cfg, root)

	path, err := srv.NewStep(ctx, feature)
	if err != nil {
		return renderRefusal(stderr, "new step", err)
	}

	rel, err := filepath.Rel(wd, path)
	if err != nil {
		rel = path
	}

	fmt.Fprintln(stdout, rel)

	return nil
}
