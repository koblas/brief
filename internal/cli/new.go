package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/spf13/cobra"
)

// newLong is "brief new"'s one-sentence help prose, rendered above its two
// children's rows in the "cmdList" group body.
const newLong = "Scaffolds a new feature, or the next step of an existing feature."

// newFeatureLong is "brief new feature"'s help prose.
const newFeatureLong = `Scaffolds docs/specifications/<name> (or the configured feature directory)
with an empty specification skeleton and an empty state file.

A name may not be empty or contain whitespace.`

// newStepLong is "brief new step"'s help prose.
const newStepLong = `Scaffolds the next step file for feature and appends its entry to the
feature's progress list.`

// newFeatureInvocation is the invocation string every "brief new feature"
// usage error names as how to fix it.
const newFeatureInvocation = "brief new feature <name>"

// newStepInvocation is the invocation string every "brief new step" usage
// error names as how to fix it.
const newStepInvocation = "brief new step <feature>"

// runNew handles "brief new <type> ...": routes a sole "-h"/"--help"
// argument to cmd.Help() (new's flag parsing is disabled, so cobra's own
// help check never sees it), and otherwise reports type is neither
// feature nor step — nothing at all, or something unknown, including a
// "-h"/"--help" alongside any other argument.
func runNew(cmd *cobra.Command, args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief new: no type given; expected one of: feature, step")
	}

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		return cmd.Help()
	}

	return usageError(stderr, fmt.Sprintf("brief new: unknown type %q; expected one of: feature, step", args[0]))
}

// runNewFeature implements "brief new feature <name>"; rest is its
// positional arguments, flags already parsed away.
func runNewFeature(ctx context.Context, wd string, rest []string, stdout, stderr io.Writer) error {
	switch {
	case len(rest) == 0:
		return usageError(stderr, fmt.Sprintf("brief new feature: no name given; run '%s'", newFeatureInvocation))
	case len(rest) > 1:
		return usageError(stderr, fmt.Sprintf("brief new feature: too many arguments; run '%s'", newFeatureInvocation))
	}

	name := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return renderRefusal(stderr, "new feature", err)
	}

	srv := scaffold.NewServer(cfg, root)

	path, err := srv.NewFeature(ctx, name)
	if err != nil {
		if errors.Is(err, scaffold.ErrInvalidFeatureName) {
			if name == "" {
				return usageError(stderr, fmt.Sprintf("brief new feature: name is empty; run '%s' with a non-empty name", newFeatureInvocation))
			}

			return usageError(stderr, fmt.Sprintf("brief new feature: name %q contains whitespace; run '%s' with a name containing no whitespace", name, newFeatureInvocation))
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
		return usageError(stderr, fmt.Sprintf("brief new step: no feature given; run '%s'", newStepInvocation))
	case len(rest) > 1:
		return usageError(stderr, fmt.Sprintf("brief new step: too many arguments; run '%s'", newStepInvocation))
	}

	feature := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return renderRefusal(stderr, "new step", err)
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
