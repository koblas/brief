package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/spf13/cobra"
)

// newShort is "new"'s one-line description: never rendered in help, since
// "new" always contributes its children's rows in place of its own (see
// helpTemplate), but read by completion's descriptions.
const newShort = "scaffold a feature or its next step"

// newLong is "brief new"'s one-sentence help prose, rendered above its two
// children's rows in the "cmdList" group body.
const newLong = "Scaffolds a new feature, or the next step of an existing feature."

// newFeatureLong is "brief new feature"'s help prose.
var newFeatureLong = `Scaffolds docs/specifications/<name> (or the configured feature directory)
with an empty specification skeleton and an empty state file.

A name may not be empty or contain whitespace.

` + jsonFieldsParagraph("feature", "step", "path", "created")

// newStepLong is "brief new step"'s help prose.
var newStepLong = `Scaffolds the next step file for feature and appends its entry to the
feature's progress list.

` + jsonFieldsParagraph("feature", "step", "path", "created")

// newFeatureInvocation is the invocation string every "brief new feature"
// usage error names as how to fix it.
const newFeatureInvocation = "brief new feature <name>"

// newStepInvocation is the invocation string every "brief new step" usage
// error names as how to fix it.
const newStepInvocation = "brief new step <feature>"

// newDocument is "new feature"'s and "new step"'s shared --json success
// document: the common header first, then the scaffolded feature, the
// step id (null for "new feature" — no call creates a feature and a step
// together), the single path the text-mode contract prints, and every
// path this call created, absolute throughout (R6). Created is never nil,
// so it encodes "[]" rather than "null" if ever empty.
type newDocument struct {
	jsonHeader

	Feature string   `json:"feature"`
	Step    *string  `json:"step"`
	Path    string   `json:"path"`
	Created []string `json:"created"`
}

// runNew handles "brief new <type> ...": rejects "-h"/"--help" given an
// attached value, routes a sole "-h"/"--help" argument to cmd.Help()
// (new's flag parsing is disabled, so cobra's own help check never sees
// it), rejects a "-h"/"--help" alongside any other argument and any other
// dash-prefixed type with the same flag-shaped wording runRoot uses — "--"
// excluded, since it is pflag's own flag-parsing terminator rather than a
// flag itself — and otherwise reports type is neither feature nor step —
// nothing at all, or something unknown.
func runNew(cmd *cobra.Command, args []string, out reporter) error {
	if len(args) == 0 {
		return out.usageError("brief new: no type given; expected one of: feature, step")
	}

	switch kind, msg := classifyDashArg(args[0]); kind {
	case argHelpFlagWithValue:
		return out.usageError(fmt.Sprintf("brief new: '%s' takes no value; run 'brief new --help'", msg))
	case argHelpFlag:
		if len(args) == 1 {
			return cmd.Help()
		}

		return out.usageError(fmt.Sprintf("brief new: '%s' takes no arguments; run 'brief help new <type>'", args[0]))
	case argUnknownFlag, argVersionFlag, argVersionFlagWithValue:
		return out.usageError(fmt.Sprintf("brief new: %s; run 'brief new <type> --help'", msg))
	case argNotFlag:
	}

	return out.usageError(fmt.Sprintf("brief new: unknown type %q; expected one of: feature, step", args[0]))
}

// runNewFeature implements "brief new feature <name>"; rest is its
// positional arguments, flags already parsed away.
func runNewFeature(ctx context.Context, wd string, rest []string, out reporter) error {
	switch {
	case len(rest) == 0:
		return out.usageError(fmt.Sprintf("brief new feature: no name given; run '%s'", newFeatureInvocation))
	case len(rest) > 1:
		return out.usageError(fmt.Sprintf("brief new feature: too many arguments; run '%s'", newFeatureInvocation))
	}

	name := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.NewFeature(ctx, name)
	if err != nil {
		if errors.Is(err, scaffold.ErrInvalidFeatureName) {
			if name == "" {
				return out.usageError(fmt.Sprintf("brief new feature: name is empty; run '%s' with a non-empty name", newFeatureInvocation))
			}

			return out.usageError(fmt.Sprintf("brief new feature: name %q contains whitespace; run '%s' with a name containing no whitespace", name, newFeatureInvocation))
		}

		return out.refusal(err)
	}

	if out.json {
		doc := newDocument{jsonHeader: out.successHeader(), Feature: res.Feature, Path: res.Path, Created: res.Created}

		return out.document(doc)
	}

	fmt.Fprintln(out.stdout, displayPath(wd, res.Path))
	fmt.Fprintf(out.stderr, "brief new feature: created %s (%s, %s); add a step with 'brief new step %s'\n",
		res.Feature, displayPath(wd, res.Created[0]), displayPath(wd, res.Created[1]), res.Feature)

	return nil
}

// runNewStep implements "brief new step <feature>"; rest is its
// positional arguments, flags already parsed away.
func runNewStep(ctx context.Context, wd string, rest []string, out reporter) error {
	switch {
	case len(rest) == 0:
		return out.usageError(fmt.Sprintf("brief new step: no feature given; run '%s'", newStepInvocation))
	case len(rest) > 1:
		return out.usageError(fmt.Sprintf("brief new step: too many arguments; run '%s'", newStepInvocation))
	}

	feature := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.NewStep(ctx, feature)
	if err != nil {
		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err))
	}

	if out.json {
		step := res.Step
		doc := newDocument{jsonHeader: out.successHeader(), Feature: res.Feature, Step: &step, Path: res.Path, Created: res.Created}

		return out.document(doc)
	}

	fmt.Fprintln(out.stdout, displayPath(wd, res.Path))
	fmt.Fprintf(out.stderr, "brief new step: created %s in %s; fill in its acceptance criteria and checklist, then 'brief start %s'\n",
		res.Step, res.Feature, res.Feature)

	return nil
}
