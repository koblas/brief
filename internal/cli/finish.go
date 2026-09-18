package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
)

// finishUsage is "brief finish"'s help text.
const finishUsage = `Usage:
  brief finish <feature> <step> --handoff <path> --state <path>

Closes step in feature: replaces its handoff block with the body at
--handoff, replaces the feature's state file with the body at --state,
and marks the step done in the progress list. "-" reads a flag's body
from stdin; it may be given for at most one of --handoff and --state.

  --handoff <path>  the step's handoff block, replacing what is there now
  --state <path>    the COMPLETE replacement body for the state file; it
                     replaces the file, it is never appended to
`

// runFinish implements "brief finish <feature> <step> --handoff <path>
// --state <path>".
func runFinish(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("finish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	handoffPath := fs.String("handoff", "", "path to the handoff body, or - for stdin")
	statePath := fs.String("state", "", "path to the replacement state body, or - for stdin")

	// flag.FlagSet.Parse stops at the first argument that does not start
	// with "-", so <feature> and <step> — which precede every flag in
	// this command's contract — must be peeled off before Parse ever
	// sees them, or they would swallow --handoff and --state as
	// leftover positional arguments.
	rest, flagArgs := splitLeadingPositionals(args)

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, finishUsage)
			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief finish: %s; run '%s'", err, finishInvocation))
	}

	switch {
	case len(rest) == 0:
		return usageError(stderr, fmt.Sprintf("brief finish: no feature given; run '%s'", finishInvocation))
	case len(rest) == 1:
		return usageError(stderr, fmt.Sprintf("brief finish: no step given; run '%s'", finishInvocation))
	case len(rest) > 2:
		return usageError(stderr, fmt.Sprintf("brief finish: too many arguments; run '%s'", finishInvocation))
	}

	feature, step := rest[0], rest[1]

	switch {
	case *handoffPath == "":
		return usageError(stderr, fmt.Sprintf("brief finish: --handoff is required; run '%s'", finishInvocation))
	case *statePath == "":
		return usageError(stderr, fmt.Sprintf("brief finish: --state is required; run '%s'", finishInvocation))
	case *handoffPath == "-" && *statePath == "-":
		return usageError(stderr, "brief finish: - may be given for at most one of --handoff and --state")
	}

	handoff, err := readSource(*handoffPath, stdin)
	if err != nil {
		return renderRefusal(stderr, "finish", err)
	}

	state, err := readSource(*statePath, stdin)
	if err != nil {
		return renderRefusal(stderr, "finish", err)
	}

	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return renderRefusal(stderr, "finish", err)
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	srv := scaffold.NewServer(cfg, root)

	if err := srv.Finish(ctx, feature, step, handoff, state); err != nil {
		return renderRefusal(stderr, "finish", err)
	}

	fmt.Fprintf(stderr, "brief finish: %s is done\n", step)

	return nil
}

// finishInvocation is the invocation string every "brief finish" usage
// error names as how to fix it.
const finishInvocation = "brief finish <feature> <step> --handoff <path> --state <path>"

// splitLeadingPositionals splits args into the leading run of arguments
// that do not start with "-" and everything from the first "-"-prefixed
// argument onward, so a flag.FlagSet — which stops parsing at the first
// non-flag argument — only ever sees flags.
func splitLeadingPositionals(args []string) ([]string, []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			return args[:i], args[i:]
		}
	}

	return args, nil
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
