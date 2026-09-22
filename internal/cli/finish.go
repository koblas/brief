package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/koblas/brief/internal/scaffold"
)

// finishLong is "brief finish"'s help prose.
var finishLong = `Closes step in feature: writes the body at --handoff to the step's own
handoff file, replaces the feature's state file with the body at --state,
and marks the step done in the progress list. "-" reads a flag's body
from stdin; it may be given for at most one of --handoff and --state.

` + jsonFieldsParagraph("feature", "step", "changed", "handoff_path", "state_path", "next")

// finishDocument is finish's --json success document: the common header
// first, then scaffold.FinishResult's own fields, both paths absolute and
// passed through verbatim. Next is statusNextJSON, the identical
// id/title/path object status's own "next" renders (MAJOR 2) — nil (JSON
// null) when nothing is open — and Changed is false only on R11's no-op.
type finishDocument struct {
	jsonHeader

	Feature     string          `json:"feature"`
	Step        string          `json:"step"`
	Changed     bool            `json:"changed"`
	HandoffPath string          `json:"handoff_path"`
	StatePath   string          `json:"state_path"`
	Next        *statusNextJSON `json:"next"`
}

// runFinish implements "brief finish <feature> <step> --handoff <path>
// --state <path>"; rest is its positional arguments and handoffPath and
// statePath its flag values, "" when the flag was not given.
func runFinish(ctx context.Context, wd string, rest []string, handoffPath, statePath string, stdin io.Reader, out reporter) error {
	switch {
	case len(rest) == 0:
		return out.usageError(fmt.Sprintf("brief finish: no feature given; run '%s'", finishInvocation))
	case len(rest) == 1:
		return out.usageError(fmt.Sprintf("brief finish: no step given; run '%s'", finishInvocation))
	case len(rest) > 2:
		return out.usageError(fmt.Sprintf("brief finish: too many arguments; run '%s'", finishInvocation))
	}

	feature, step := rest[0], rest[1]

	switch {
	case handoffPath == "":
		return out.usageError(fmt.Sprintf("brief finish: --handoff is required; run '%s'", finishInvocation))
	case statePath == "":
		return out.usageError(fmt.Sprintf("brief finish: --state is required; run '%s'", finishInvocation))
	case handoffPath == "-" && statePath == "-":
		return out.usageError("brief finish: - may be given for at most one of --handoff and --state")
	}

	handoff, err := readSource(handoffPath, stdin)
	if err != nil {
		return out.refusal(err)
	}

	state, err := readSource(statePath, stdin)
	if err != nil {
		return out.refusal(err)
	}

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.Finish(ctx, feature, step, handoff, state)
	if err != nil {
		if refusal, ok := errors.AsType[*scaffold.RefusalError](err); ok {
			switch refusal.Path {
			case scaffold.StateSource:
				refusal.Path = sourceLocator(statePath)
			case scaffold.HandoffSource:
				refusal.Path = sourceLocator(handoffPath)
			}
		}

		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err))
	}

	if out.json {
		var next *statusNextJSON
		if res.Next.ID != "" {
			next = &statusNextJSON{ID: res.Next.ID, Title: res.Next.Title, Path: res.Next.Path}
		}

		doc := finishDocument{
			jsonHeader:  out.successHeader(),
			Feature:     res.Feature,
			Step:        res.Step,
			Changed:     res.Changed,
			HandoffPath: res.HandoffPath,
			StatePath:   res.StatePath,
			Next:        next,
		}

		return out.document(doc)
	}

	if !res.Changed {
		fmt.Fprintf(out.stderr, "brief finish: %s %s already done with identical inputs; nothing written\n", feature, step)

		return nil
	}

	handoffRel, stateRel := displayPath(wd, res.HandoffPath), displayPath(wd, res.StatePath)

	if res.Next.ID != "" {
		fmt.Fprintf(out.stderr, "brief finish: %s %s done; wrote %s, replaced %s; next: %s — run 'brief start %s'\n",
			feature, step, handoffRel, stateRel, res.Next.ID, feature)
	} else {
		fmt.Fprintf(out.stderr, "brief finish: %s %s done; wrote %s, replaced %s; %s is complete\n",
			feature, step, handoffRel, stateRel, feature)
	}

	return nil
}

// finishInvocation is the invocation string every "brief finish" usage
// error names as how to fix it.
const finishInvocation = "brief finish <feature> <step> --handoff <path> --state <path>"

// sourceLocator returns the refusal locator for one of finish's --handoff
// or --state arguments: path unchanged, or "<stdin>" when path is "-", so
// a refusal about piped input never names an empty or misleading path.
func sourceLocator(path string) string {
	if path == "-" {
		return "<stdin>"
	}

	return path
}

// readSource returns the bytes at path, or stdin's contents when path is
// "-".
func readSource(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}

		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return data, nil
}
