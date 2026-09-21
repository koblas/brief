package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/koblas/brief/internal/assemble"
)

// startLong is "brief start"'s help prose.
var startLong = `Prints the next open step's id, title, acceptance criteria and checklist,
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
brief start reads; it never writes.

` + jsonFieldsParagraph("done", "open", "step", "inherited", "shortfalls") + "\n" +
	wrapWords(`"step" is null when there is no open step, and --json may be given before or after <feature>.`, jsonParagraphWidth)

// startInvocation is the invocation string every "brief start" usage error
// names as how to fix it.
const startInvocation = "brief start <feature>"

// startDocument is start's --json success document: the common header
// first, then assemble.Brief's own fields flattened beside it, no "data"
// wrapper (R2).
type startDocument struct {
	jsonHeader

	assemble.Brief
}

// runStart implements "brief start [--json] <feature>"; rest is its
// positional arguments, from either side of --json, flags already parsed
// away. jsonOut is out.json, read once at the call site: every
// JSON-capable leaf's own pflag "json" flag stays registered only so its
// help table row still renders, since run's own scanJSONFlag strips every
// "--json" token before pflag ever parses one.
func runStart(ctx context.Context, wd string, rest []string, jsonOut bool, out reporter) error {
	switch {
	case len(rest) == 0:
		return out.usageError(fmt.Sprintf("brief start: no feature given; run '%s'", startInvocation))
	case len(rest) > 1:
		return out.usageError(fmt.Sprintf("brief start: too many arguments; run '%s'", startInvocation))
	}

	feature := rest[0]

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(ctx, feature)
	if err != nil {
		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err))
	}

	// In --json mode a successful run writes nothing to stderr (R1): every
	// shortfall is already payload (brief.Shortfalls), and the "complete"
	// / "no step files yet" notices are payload-derivable from step:null
	// plus done/open, so the document alone is the discriminator a
	// structured caller reads.
	if jsonOut {
		doc := startDocument{jsonHeader: out.successHeader(), Brief: brief}

		if err := writeJSONDocument(out.stdout, doc); err != nil {
			return fmt.Errorf("brief start: %w", err)
		}

		return nil
	}

	for _, s := range brief.Shortfalls {
		fmt.Fprintf(out.stderr, "brief start: %s: %s; %s\n", displayPath(wd, s.Path), flattenOneLine(s.Detail), flattenOneLine(s.Fix))
	}

	if brief.Step == nil {
		featureDir := displayPath(wd, filepath.Join(root, cfg.FeatureDirectory, feature))

		if brief.Done+brief.Open > 0 {
			fmt.Fprintf(out.stderr, "brief start: %s: feature is complete, %d of %d steps done; run 'brief new step %s' to add the next one\n",
				featureDir, brief.Done, brief.Done+brief.Open, feature)
		} else {
			fmt.Fprintf(out.stderr, "brief start: %s: no step files yet; run 'brief new step %s' to scaffold the first one\n",
				featureDir, feature)
		}
	}

	if err := assemble.RenderText(out.stdout, brief); err != nil {
		return fmt.Errorf("brief start: %w", err)
	}

	return nil
}
