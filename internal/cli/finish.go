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
const finishLong = `Closes step in feature: writes the body at --handoff to the step's own
handoff file, replaces the feature's state file with the body at --state,
and marks the step done in the progress list. "-" reads a flag's body
from stdin; it may be given for at most one of --handoff and --state.`

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

	if err := srv.Finish(ctx, feature, step, handoff, state); err != nil {
		if refusal, ok := errors.AsType[*scaffold.RefusalError](err); ok {
			switch refusal.Path {
			case scaffold.StateSource:
				refusal.Path = sourceLocator(statePath)
			case scaffold.HandoffSource:
				refusal.Path = sourceLocator(handoffPath)
			}
		}

		return out.refusal(err)
	}

	fmt.Fprintf(out.stderr, "brief finish: %s is done\n", step)

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
