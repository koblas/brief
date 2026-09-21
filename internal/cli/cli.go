package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// invocationAnnotation is the cobra.Command.Annotations key holding the
// invocation string the root FlagErrorFunc names in "run '<invocation>'"
// when that command's flag parsing fails.
const invocationAnnotation = "invocation"

// Run parses args, dispatches to the named command, and renders every
// user-facing line to stdout or stderr itself. wd is the working directory
// used to resolve configuration and to relativize any printed path — Run
// never calls os.Getwd. stdin backs "-" arguments on commands that read one
// (finish's --handoff/--state); commands that take no such argument never
// read it.
func Run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// cobra's RunE has no context.Context parameter; every closure below
	// reads it via cmd.Context(), which ExecuteContext(ctx) sets on the
	// resolved command before RunE runs. contextcheck cannot see that
	// guarantee through cobra's own dispatch and instead flags the
	// context.Background() fallback inside Command.Context()'s body.
	root := newRootCommand(wd, stdin, stdout, stderr) //nolint:contextcheck

	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	root.SetArgs(argsCopy)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	return root.ExecuteContext(ctx)
}

// newRootCommand builds brief's command tree for one Run. It is rebuilt on
// every call rather than held in a package variable: cobra records parse
// state and flag values on the *Command itself, so a shared tree would
// leak one invocation's flags into the next.
//
// The root and "new" disable cobra's flag parsing and resolve their first
// argument themselves, via the unchanged runRoot and runNew, so a missing
// or unknown command or type — "-x" included — keeps its own one-line
// usage error instead of cobra's default dispatch. The leaves let cobra
// (via pflag) parse flags and report an undefined one in pflag's own
// words; the root FlagErrorFunc rewrites that into brief's one-line usage
// error, naming the invocation carried in the leaf's Annotations.
//
// Every command in the tree is Runnable with Args: cobra.ArbitraryArgs, so
// cobra never rejects an argument count itself — every run* function does
// its own counting and reports brief's own usage error.
func newRootCommand(wd string, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:                "brief",
		Long:               usage,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runRoot(args, stdout, stderr)
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true

	newCmd := &cobra.Command{
		Use:                "new",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return runNew(args, stderr)
		},
	}
	newCmd.AddCommand(
		leafCommand("feature", "brief new feature <name>", newFeatureUsage, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewFeature(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("step", "brief new step <feature>", newStepUsage, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewStep(cmd.Context(), wd, args, stdout, stderr)
			}),
	)

	root.AddCommand(
		newCmd,
		leafCommand("start", "brief start <feature>", startUsage,
			func(fs *pflag.FlagSet) {
				fs.Bool("json", false, "print the brief as a single JSON document instead of markdown")
			},
			func(cmd *cobra.Command, args []string) error {
				jsonOut, _ := cmd.Flags().GetBool("json")

				return runStart(cmd.Context(), wd, args, jsonOut, stdout, stderr)
			}),
		leafCommand("status", "brief status", statusUsage, nil,
			func(cmd *cobra.Command, args []string) error {
				return runStatus(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("finish", finishInvocation, finishUsage,
			func(fs *pflag.FlagSet) {
				fs.String("handoff", "", "the step's handoff body, written to its own file")
				fs.String("state", "", "the complete replacement body for the state file")
			},
			func(cmd *cobra.Command, args []string) error {
				handoffPath, _ := cmd.Flags().GetString("handoff")
				statePath, _ := cmd.Flags().GetString("state")

				return runFinish(cmd.Context(), wd, args, handoffPath, statePath, stdin, stderr)
			}),
		leafCommand("check", "brief check [feature]", checkUsage, nil,
			func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd.Context(), wd, args, stdout, stderr)
			}),
	)

	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		path := strings.TrimPrefix(cmd.CommandPath(), "brief ")
		invocation := cmd.Annotations[invocationAnnotation]

		return usageError(stderr, fmt.Sprintf("brief %s: %s; run '%s'", path, err, invocation))
	})

	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		fmt.Fprint(stdout, cmd.Long)
	})

	// A hidden "help" stub replaces cobra's default help command, which on
	// an unknown topic calls cobra.CheckErr and os.Exit(1) directly — the
	// only exit this package allows is ExitCode, called from main. The
	// stub ignores its topic and prints the root usage, matching runRoot's
	// own "help" handling for a bare "brief help".
	root.SetHelpCommand(&cobra.Command{
		Use:                "help",
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprint(stdout, usage)

			return nil
		},
	})

	return root
}

// leafCommand builds a command that takes flags and positionals but no
// subcommands. invocation is the usage line the root FlagErrorFunc names
// as how to fix a flag error on this command, and help is cmd.Long,
// printed verbatim to stdout by the root HelpFunc on -h, -help or --help.
// addFlags registers this command's own flags; nil for a command with none
// beyond cobra's automatic -h/--help.
func leafCommand(name, invocation, help string, addFlags func(*pflag.FlagSet), run func(*cobra.Command, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:         name,
		Long:        help,
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{invocationAnnotation: invocation},
		RunE:        run,
	}

	if addFlags != nil {
		addFlags(cmd.Flags())
	}

	return cmd
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
