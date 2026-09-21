package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	ucli "github.com/urfave/cli/v3"
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
  brief status                 print one done/total/next/blocked line per feature
  brief check [feature]        report faults finish would now refuse to write over

  brief finish <feature> <step> --handoff <path> --state <path>
                                close a step: handoff, state, then done

Run 'brief new feature --help', 'brief new step --help', 'brief start
--help', 'brief status --help', 'brief check --help' or 'brief finish
--help' for details on those commands.
`

// Run parses args, dispatches to the named command, and renders every
// user-facing line to stdout or stderr itself. wd is the working directory
// used to resolve configuration and to relativize any printed path — Run
// never calls os.Getwd. stdin backs "-" arguments on commands that read one
// (finish's --handoff/--state); commands that take no such argument never
// read it.
func Run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	root := newRootCommand(wd, stdin, stdout, stderr)

	return root.Run(ctx, append([]string{"brief"}, args...))
}

// newRootCommand builds brief's command tree for one Run. It is rebuilt on
// every call rather than held in a package variable: urfave/cli records
// parse state on the *Command itself, so a shared tree would leak one
// invocation's flags into the next.
//
// The root and "new" skip urfave's flag parsing and handle their first
// argument themselves, so a missing or unknown command or type — "-x"
// included — keeps its own one-line usage error instead of urfave's help
// dump. The leaves let urfave parse flags, which it reports in the stdlib
// flag package's words ("flag provided but not defined: -x"), and print
// their own usage constant as help.
func newRootCommand(wd string, stdin io.Reader, stdout, stderr io.Writer) *ucli.Command {
	return &ucli.Command{
		Name:            "brief",
		Writer:          stdout,
		ErrWriter:       stderr,
		HideHelpCommand: true,
		SkipFlagParsing: true,
		// A no-op handler keeps urfave from calling os.Exit on an
		// ExitCoder error: cmd/brief owns the only exit, via ExitCode.
		ExitErrHandler: func(context.Context, *ucli.Command, error) {},
		Action: func(_ context.Context, cmd *ucli.Command) error {
			return runRoot(cmd.Args().Slice(), stdout, stderr)
		},
		Commands: []*ucli.Command{
			{
				Name:            "new",
				HideHelpCommand: true,
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *ucli.Command) error {
					return runNew(cmd.Args().Slice(), stderr)
				},
				Commands: []*ucli.Command{
					leafCommand("feature", "new feature", "brief new feature <name>", newFeatureUsage, stdout, stderr, nil,
						func(ctx context.Context, cmd *ucli.Command) error {
							return runNewFeature(ctx, wd, cmd.Args().Slice(), stdout, stderr)
						}),
					leafCommand("step", "new step", "brief new step <feature>", newStepUsage, stdout, stderr, nil,
						func(ctx context.Context, cmd *ucli.Command) error {
							return runNewStep(ctx, wd, cmd.Args().Slice(), stdout, stderr)
						}),
				},
			},
			leafCommand("start", "start", "brief start <feature>", startUsage, stdout, stderr,
				[]ucli.Flag{&ucli.BoolFlag{Name: "json"}},
				func(ctx context.Context, cmd *ucli.Command) error {
					return runStart(ctx, wd, cmd.Args().Slice(), cmd.Bool("json"), stdout, stderr)
				}),
			leafCommand("status", "status", "brief status", statusUsage, stdout, stderr, nil,
				func(ctx context.Context, cmd *ucli.Command) error {
					return runStatus(ctx, wd, cmd.Args().Slice(), stdout, stderr)
				}),
			leafCommand("finish", "finish", finishInvocation, finishUsage, stdout, stderr,
				[]ucli.Flag{&ucli.StringFlag{Name: "handoff"}, &ucli.StringFlag{Name: "state"}},
				func(ctx context.Context, cmd *ucli.Command) error {
					return runFinish(ctx, wd, cmd.Args().Slice(), cmd.String("handoff"), cmd.String("state"), stdin, stderr)
				}),
			leafCommand("check", "check", "brief check [feature]", checkUsage, stdout, stderr, nil,
				func(ctx context.Context, cmd *ucli.Command) error {
					return runCheck(ctx, wd, cmd.Args().Slice(), stdout, stderr)
				}),
		},
	}
}

// leafCommand builds a command that takes flags and positionals but no
// subcommands. path is its full name after "brief", invocation is the
// usage line every flag error names as how to fix it, and help is printed
// verbatim to stdout on -h, -help or --help.
//
// urfave's own help flag is hidden, not reused: with it defined, urfave
// prints its generated help — ignoring any template — whenever --help
// arrives alongside a flag error. Without it, -h and --help are undefined
// flags, and isHelpRequest spots them in OnUsageError. Because urfave
// parses left to right and stops at the first undefined flag, this keeps
// the stdlib flag package's order semantics: "--help --bogus" prints help,
// "--bogus --help" is a usage error.
func leafCommand(name, path, invocation, help string, stdout, stderr io.Writer, flags []ucli.Flag, action ucli.ActionFunc) *ucli.Command {
	return &ucli.Command{
		Name:            name,
		Flags:           flags,
		HideHelp:        true,
		HideHelpCommand: true,
		OnUsageError: func(_ context.Context, _ *ucli.Command, err error, _ bool) error {
			if isHelpRequest(err) {
				fmt.Fprint(stdout, help)
				return nil
			}

			return usageError(stderr, fmt.Sprintf("brief %s: %s; run '%s'", path, err, invocation))
		},
		Action: action,
	}
}

// isHelpRequest reports whether err is urfave's undefined-flag error for
// -h or -help. urfave exports no type for it, only this message, which it
// words exactly as the stdlib flag package does.
func isHelpRequest(err error) bool {
	switch err.Error() {
	case "flag provided but not defined: -h", "flag provided but not defined: -help":
		return true
	default:
		return false
	}
}

// runRoot handles a top-level invocation that named no known command:
// help, nothing at all, or something unknown.
func runRoot(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief: no command given; expected one of: new, start, finish, status, check")
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return usageError(stderr, fmt.Sprintf("brief: unknown command %q; expected one of: new, start, finish, status, check", args[0]))
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
