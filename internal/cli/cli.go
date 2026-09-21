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

func init() {
	// Root help lists commands in workflow order — new, start, status,
	// check, finish, as registered by newRootCommand — rather than
	// cobra's default alphabetical sort. EnableCommandSorting is a cobra
	// package global: set once here, never per Run or per call, since a
	// per-call write would race parallel tests' reads.
	cobra.EnableCommandSorting = false
}

// rootShort is root's one-sentence description: the first line of every
// root help render.
const rootShort = "brief manages feature specifications as files in your repository."

// expectedCommands is the "expected one of:" list named by runRoot's usage
// errors and by the hidden help stub's unknown-topic error. It is today's
// approved literal, registration order minus "new" (see S07's "new, start,
// status, check, finish" root listing) — not tree-derived; S11 replaces
// this with a derivation from the command tree and moves all three
// messages together.
const expectedCommands = "new, start, finish, status, check"

// helpTemplate renders R6's contract for every command in the tree, set
// once on root via SetHelpTemplate and inherited by every child through
// HelpTemplate()'s parent walk.
//
// A leaf (HasParent) renders its generated Usage line, its Long prose
// trimmed, and pflag's own flag table — nothing else, so no "Global
// Flags:", "Additional help topics:" or cobra trailer ever appears.
//
// Root renders its one-sentence Long, then one row per available command
// under "Usage:" — a command with its own available subcommands (only
// "new" today) contributes its children's rows instead of its own, so
// "new feature" and "new step" list in "new"'s place — each row is that
// command's UseLine padded to a fixed column, wrapped to its own line
// first when UseLine would overrun that column, followed by its Short;
// then the "Run '...' for details." trailer. Only cobra's built-in
// template funcs (rpad, trim, trimTrailingWhitespaces) and text/template
// builtins are used — no package-global AddTemplateFunc.
const helpTemplate = `{{- define "cmdRow" -}}
{{if gt (len .UseLine) 33}}  {{.UseLine}}
{{rpad "" 35}}{{else}}  {{rpad .UseLine 33}}{{end}}{{.Short}}
{{end -}}
{{- if .HasParent}}Usage:
  {{.UseLine}}

{{.Long | trimTrailingWhitespaces}}

Flags:
{{.LocalFlags.FlagUsages}}{{- else}}{{.Long}}

Usage:
{{range .Commands}}{{if .IsAvailableCommand}}
{{- if .HasAvailableSubCommands}}
{{- range .Commands}}{{if .IsAvailableCommand}}{{template "cmdRow" .}}{{end}}{{end}}
{{- else}}{{template "cmdRow" .}}
{{- end}}
{{- end}}
{{- end}}
Run 'brief <command> --help' for details.
{{end -}}
`

// invocationAnnotation is the cobra.Command.Annotations key holding the
// invocation string the root FlagErrorFunc names in "run '<invocation>'"
// when that command's flag parsing fails.
const invocationAnnotation = "invocation"

// jsonFlagUsage is start's --json flag's usage string, shown in its Flags
// table. Its embedded newlines are pflag's own wrapping cue: FlagUsages
// re-indents them to the table's description column.
const jsonFlagUsage = `print the brief as a single JSON document instead of markdown.
Every exit-0 run writes one, even when there is no open step:
"step" is null rather than the document being omitted, so a
structured caller detects completion the same way a human
reads the stderr notice. --json may be given before or after
<feature>.`

// handoffFlagUsage is finish's --handoff flag's usage string. The
// backquoted "path" is pflag's own convention (UnquoteUsage): it names the
// flag's value in its Flags table row ("--handoff path") instead of
// pflag's default type name ("string").
const handoffFlagUsage = "the `path` to the step's handoff body, " +
	"written to its own file"

// stateFlagUsage is finish's --state flag's usage string; see
// handoffFlagUsage for the backquoted "path" convention.
const stateFlagUsage = "the `path` to the COMPLETE replacement body for " +
	"the state file; it replaces the file, it is never appended to; " +
	"it must carry the configured state headings, though a section " +
	"may be empty"

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
//
// Commands are added in the order they should list in root help —
// new, start, status, check, finish — not alphabetically: see this
// package's init, which turns cobra's default sort off.
func newRootCommand(wd string, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:                "brief",
		Long:               rootShort,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args, stderr)
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
		leafCommand("feature <name>", "scaffold a new feature's specification and state file", "brief new feature <name>", newFeatureLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewFeature(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("step <feature>", "scaffold the next step file and its progress entry", "brief new step <feature>", newStepLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewStep(cmd.Context(), wd, args, stdout, stderr)
			}),
	)

	root.AddCommand(
		newCmd,
		leafCommand("start [--json] <feature>", "print the next open step's context", "brief start <feature>", startLong,
			func(fs *pflag.FlagSet) {
				fs.Bool("json", false, jsonFlagUsage)
			},
			func(cmd *cobra.Command, args []string) error {
				jsonOut, _ := cmd.Flags().GetBool("json")

				return runStart(cmd.Context(), wd, args, jsonOut, stdout, stderr)
			}),
		leafCommand("status", "print one done/total/next/blocked line per feature", "brief status", statusLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runStatus(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("check [feature]", "report faults finish would now refuse to write over", "brief check [feature]", checkLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("finish <feature> <step> --handoff <path> --state <path>", "close a step: handoff, state, then done", finishInvocation, finishLong,
			func(fs *pflag.FlagSet) {
				fs.String("handoff", "", handoffFlagUsage)
				fs.String("state", "", stateFlagUsage)
			},
			func(cmd *cobra.Command, args []string) error {
				handoffPath, _ := cmd.Flags().GetString("handoff")
				statePath, _ := cmd.Flags().GetString("state")

				return runFinish(cmd.Context(), wd, args, handoffPath, statePath, stdin, stderr)
			}),
	)

	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		path := strings.TrimPrefix(cmd.CommandPath(), "brief ")
		invocation := cmd.Annotations[invocationAnnotation]

		return usageError(stderr, fmt.Sprintf("brief %s: %s; run '%s'", path, err, invocation))
	})

	root.SetHelpTemplate(helpTemplate)

	// A hidden "help" stub replaces cobra's default help command, which on
	// an unknown topic calls cobra.CheckErr and os.Exit(1) directly — the
	// only exit this package allows is ExitCode, called from main. The
	// stub resolves its topic against the tree with Find, which never
	// errors on this ArbitraryArgs-everywhere tree and returns the
	// unstripped residual as its second value. A topic is accepted only
	// when that residual is empty and the resolved target is root itself
	// (a bare "brief help") or IsAvailableCommand — so a leftover
	// positional, a flag left after the topic, a flag ahead of it (Find
	// stops at root, treating the topic as that flag's value) and a
	// hidden command such as "help" itself are all rejected, not
	// silently routed to some leaf's help. A rejected topic is brief's
	// own usage error naming the whole topic as typed — every argument
	// joined by a space, not just the unresolved residual — so
	// "help new bogus" names "new bogus", not a false top-level command
	// "bogus". An accepted topic renders byte-identical to
	// "<path…> --help": InitDefaultHelpFlag backfills the -h/--help row
	// that Execute() would otherwise add during ordinary dispatch, which
	// Find alone skips.
	root.SetHelpCommand(&cobra.Command{
		Use:                "help",
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, residual, _ := cmd.Root().Find(args)
			if len(residual) > 0 || (target != cmd.Root() && !target.IsAvailableCommand()) {
				return usageError(stderr, fmt.Sprintf("brief help: unknown command %q; expected one of: %s", strings.Join(args, " "), expectedCommands))
			}

			target.InitDefaultHelpFlag()

			return target.Help()
		},
	})

	return root
}

// leafCommand builds a command that takes flags and positionals but no
// subcommands. use carries the command's argument syntax (its name plus
// today's flag/positional shape, e.g. "start [--json] <feature>"); short
// is its one-line root-listing description; invocation is the usage line
// the root FlagErrorFunc names as how to fix a flag error on this command
// — a hand-written literal, independent of use, so it never changes shape
// when use grows a flag; help is the command's Long prose. addFlags
// registers this command's own flags; nil for a command with none beyond
// cobra's automatic -h/--help.
func leafCommand(use, short, invocation, help string, addFlags func(*pflag.FlagSet), run func(*cobra.Command, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   use,
		Short:                 short,
		Long:                  help,
		Args:                  cobra.ArbitraryArgs,
		DisableFlagsInUseLine: true,
		Annotations:           map[string]string{invocationAnnotation: invocation},
		RunE:                  run,
	}

	if addFlags != nil {
		addFlags(cmd.Flags())
	}

	return cmd
}

// runRoot handles a top-level invocation that named no known command:
// help, nothing at all, or something unknown.
func runRoot(cmd *cobra.Command, args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr, "brief: no command given; expected one of: "+expectedCommands)
	}

	switch args[0] {
	case "-h", "--help", "help":
		return cmd.Help()
	default:
		return usageError(stderr, fmt.Sprintf("brief: unknown command %q; expected one of: %s", args[0], expectedCommands))
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
