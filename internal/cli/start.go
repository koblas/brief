package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/koblas/brief/internal/assemble"
)

// startLong is "brief start"'s help prose.
const startLong = `Prints the next open step's id, title, acceptance criteria and checklist,
and the decisions and constraints inherited from the feature's state
file. A feature whose steps are all done, or that has no step files yet,
prints nothing and says so on stderr instead, still exiting 0.
brief start refuses, naming the file and the fix, rather than print a
partial brief: a missing or unreadable specification or state file, an
unclosed fenced code block in either, a specification with no progress
heading, or a next step whose frontmatter has no id or no checklist. A
missing optional convention — the step's acceptance heading, or a state
file heading — is named on stderr instead, one line each, and the brief
still prints on stdout, still exiting 0.
brief start reads; it never writes.`

// startInvocation is the invocation string every "brief start" usage error
// names as how to fix it.
const startInvocation = "brief start <feature>"

// runStart implements "brief start [--json] <feature>"; rest is its
// positional arguments, from either side of --json, flags already parsed
// away.
func runStart(ctx context.Context, wd string, rest []string, jsonOut bool, stdout, stderr io.Writer) error {
	switch {
	case len(rest) == 0:
		return usageError(stderr, fmt.Sprintf("brief start: no feature given; run '%s'", startInvocation))
	case len(rest) > 1:
		return usageError(stderr, fmt.Sprintf("brief start: too many arguments; run '%s'", startInvocation))
	}

	feature := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return renderRefusal(stderr, "start", err)
	}

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(ctx, feature)
	if err != nil {
		return renderRefusal(stderr, "start", err)
	}

	for _, s := range brief.Shortfalls {
		fmt.Fprintf(stderr, "brief start: %s: %s; %s\n", s.Path, flattenOneLine(s.Detail), flattenOneLine(s.Fix))
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
	}

	// --json always writes a document, even with no open step: "step"
	// marshals to null rather than the document being omitted, giving a
	// structured caller the same discriminator the stderr notice above
	// gives a human.
	if jsonOut {
		if err := assemble.RenderJSON(stdout, brief); err != nil {
			return fmt.Errorf("brief start: %w", err)
		}

		return nil
	}

	if err := assemble.RenderText(stdout, brief); err != nil {
		return fmt.Errorf("brief start: %w", err)
	}

	return nil
}
