package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
)

// startUsage is "brief start"'s help text.
const startUsage = `Usage:
  brief start [--json] <feature>

Prints the next open step's id, title, acceptance criteria and checklist,
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

  --json  print the brief as a single JSON document instead of markdown.
          Every exit-0 run writes one, even when there is no open step:
          "step" is null rather than the document being omitted, so a
          structured caller detects completion the same way a human
          reads the stderr notice. --json may be given before or after
          <feature>.
`

// runStart implements "brief start [--json] <feature>".
func runStart(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	jsonOut := fs.Bool("json", false, "print the brief as JSON")

	// --json may precede or follow <feature>, so the feature must be
	// peeled off before Parse ever sees it, the same way finish peels
	// off its own leading positionals — otherwise flag.FlagSet.Parse
	// stops at the first non-flag argument and a trailing --json is
	// left as an unconsumed "too many arguments" positional.
	leading, flagArgs := splitLeadingPositionals(args)

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, startUsage)
			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief start: %s; run 'brief start <feature>'", err))
	}

	rest := slices.Concat(leading, fs.Args())

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
	// gives a human. RenderText's own nil-Step guard already writes
	// nothing, so the non-JSON path stays exactly as before.
	if *jsonOut {
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
