package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrUsage marks an error caused by the invocation itself — a missing or
// unknown command or type, an undefined flag, the wrong number of
// arguments — rather than by anything the command tried to do. ExitCode
// maps an error satisfying errors.Is(err, ErrUsage) to exit code 2.
var ErrUsage = errors.New("usage error")

// usage is the top-level help text: what "brief --help", "brief -h" and
// "brief help" print to stdout before returning a nil error.
const usage = `brief manages feature specifications as files in your repository.

Usage:
  brief new feature <name>     scaffold a new feature's specification and state file
  brief new step <feature>     scaffold the next step file and its progress entry
  brief start <feature>        print the next open step's context

  brief finish <feature> <step> --handoff <path> --state <path>
                                close a step: handoff, state, then done

Run 'brief new feature --help', 'brief new step --help', 'brief start
--help' or 'brief finish --help' for details on those commands.
`

// Run parses args, dispatches to the named command, and renders every
// user-facing line to stdout or stderr itself. wd is the working directory
// used to resolve configuration and to relativize any printed path — Run
// never calls os.Getwd. stdin backs "-" arguments on commands that read one
// (finish's --handoff/--state); commands that take no such argument never
// read it.
func Run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief: no command given; expected one of: new, start, finish")
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	}

	switch args[0] {
	case "new":
		return runNew(ctx, wd, args[1:], stdout, stderr)
	case "start":
		return runStart(ctx, wd, args[1:], stdout, stderr)
	case "finish":
		return runFinish(ctx, wd, args[1:], stdin, stdout, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("brief: unknown command %q; expected one of: new, start, finish", args[0]))
	}
}

// usageError writes msg, followed by a single newline, to stderr and
// returns an error satisfying errors.Is(err, ErrUsage).
func usageError(stderr io.Writer, msg string) error {
	fmt.Fprintln(stderr, msg)

	return fmt.Errorf("%s: %w", msg, ErrUsage)
}

// ExitCode maps a Run error to the exit code main should return: 0 for a
// nil error, 2 for one satisfying errors.Is(err, ErrUsage), 1 for anything
// else.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrUsage):
		return 2
	default:
		return 1
	}
}
