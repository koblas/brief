package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ErrUsage marks an error caused by the invocation itself — a missing or
// unknown command or type, an undefined flag, the wrong number of
// arguments — rather than by anything the command tried to do. ExitCode
// maps an error satisfying errors.Is(err, ErrUsage) to exit code 2.
var ErrUsage = errors.New("usage error")

func init() {
	// Root help and every "expected one of:" list share one order:
	// registration order, as newRootCommand's root.AddCommand calls lay it
	// out — new, start, finish, status, check — rather than cobra's default
	// alphabetical sort. EnableCommandSorting is a cobra package global:
	// set once here, never per Run or per call, since a per-call write
	// would race parallel tests' reads.
	cobra.EnableCommandSorting = false

	// Cobra's mousetrap check (Windows only: was brief launched by
	// double-clicking it in Explorer, rather than from a shell) prints its
	// own message and calls os.Exit(1) directly, bypassing every exit this
	// package returns through ExitCode. Clearing MousetrapHelpText disables
	// that check so Run's caller stays the only place that calls os.Exit.
	cobra.MousetrapHelpText = ""
}

// rootShort is root's one-sentence description: the first line of every
// root help render.
const rootShort = "brief manages feature specifications as files in your repository."

// resolveRoot resolves wd's configuration and the directory every path in
// that configuration is relative to: source's directory when a config file
// was found, wd itself otherwise. Every command that touches configuration
// or the repository tree shares this pattern; the caller still renders its
// own renderRefusal(stderr, "<command>", err) on a non-nil error, since the
// command name in that refusal differs per caller.
func resolveRoot(wd string) (config.Config, string, error) {
	cfg, source, err := config.Resolve(wd)
	if err != nil {
		return config.Config{}, "", err
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	return cfg, root, nil
}

// expectedCommandList names cmd's root's available top-level commands, in
// registration order, for an "expected one of:" usage message. Cobra adds
// "help" as a hidden child of root during Execute; IsAvailableCommand
// excludes it, and any other hidden or deprecated command, without a
// name-based filter.
func expectedCommandList(cmd *cobra.Command) string {
	root := cmd.Root()
	names := make([]string, 0, len(root.Commands()))

	for _, c := range root.Commands() {
		if c.IsAvailableCommand() {
			names = append(names, c.Name())
		}
	}

	return strings.Join(names, ", ")
}

// helpTemplate renders every command's help text in the tree, set once on
// root via SetHelpTemplate and inherited by every child through
// HelpTemplate()'s parent walk.
//
// A command with no available subcommands (every leaf) renders its
// generated Usage line, its Long prose trimmed, and pflag's own flag
// table — nothing else, so no "Global Flags:", "Additional help topics:"
// or cobra trailer ever appears.
//
// A command with available subcommands (root, and "new") renders the
// "cmdList" group body instead: its one-sentence Long, then one row per
// available command — or per command carrying listedInHelpAnnotation, so a
// Hidden-but-listed command such as "completion" still gets a row even
// though IsAvailableCommand is false for it — under "Usage:". A child with
// its own available subcommands contributes its children's rows instead of
// its own, so "new feature" and "new step" list in "new"'s place under
// root, and under "new" itself the same two rows are its entire listing —
// each row is that command's UseLine padded to cmdRowUseWidth columns,
// wrapped to its own line first (continuation indented to
// cmdRowContinuationWidth) when UseLine would overrun that column, followed
// by its Short; then a "Run '<command path> <noun> --help' for details."
// trailer scoped to that command's own path, where <noun> is that
// command's own commandNounAnnotation ("type" for "new") or the literal
// "command" when the command carries none (root). Only cobra's built-in
// template funcs (rpad, trim, trimTrailingWhitespaces, index) and
// text/template builtins (or) are used — no package-global AddTemplateFunc.
var helpTemplate = fmt.Sprintf(`{{- define "cmdRow" -}}
{{if gt (len .UseLine) %[3]d}}  {{.UseLine}}
{{rpad "" %[4]d}}{{else}}  {{rpad .UseLine %[3]d}}{{end}}{{.Short}}
{{end -}}
{{- define "cmdList" -}}
{{.Long}}

Usage:
{{range .Commands}}{{if or .IsAvailableCommand (index .Annotations %[1]q)}}
{{- if .HasAvailableSubCommands}}
{{- range .Commands}}{{if .IsAvailableCommand}}{{template "cmdRow" .}}{{end}}{{end}}
{{- else}}{{template "cmdRow" .}}
{{- end}}
{{- end}}
{{- end}}
Run '{{.CommandPath}} <{{or (index .Annotations %[2]q) "command"}}> --help' for details.
{{end -}}
{{- if .HasAvailableSubCommands}}{{template "cmdList" .}}{{- else}}Usage:
  {{.UseLine}}

{{.Long | trimTrailingWhitespaces}}

Flags:
{{.LocalFlags.FlagUsages}}{{- end -}}
`, listedInHelpAnnotation, commandNounAnnotation, cmdRowUseWidth, cmdRowContinuationWidth)

// cmdRowUseWidth is the column helpTemplate's "cmdRow" block pads a short
// UseLine to before its Short description; a UseLine longer than this
// wraps onto its own line instead.
const cmdRowUseWidth = 33

// cmdRowContinuationWidth is the column a wrapped "cmdRow" row's Short
// starts at on its continuation line: cmdRowUseWidth plus the 2-column
// indent every row's UseLine carries, so a wrapped row's Short lines up
// with an unwrapped one's.
const cmdRowContinuationWidth = cmdRowUseWidth + 2

// invocationAnnotation is the cobra.Command.Annotations key holding the
// invocation string the root FlagErrorFunc names in "run '<invocation>'"
// when that command's flag parsing fails.
const invocationAnnotation = "invocation"

// listedInHelpAnnotation is the cobra.Command.Annotations key marking a
// Hidden command that still belongs in root help and as a "brief help"
// topic — currently only "completion": enabled and dispatchable, but
// excluded from every "expected one of:" list. helpTemplate's outer
// cmdList row loop and the help stub's topic-acceptance check both widen
// on this one annotation, so any command shown in root help is always a
// valid "brief help" topic.
const listedInHelpAnnotation = "listedInHelp"

// commandNounAnnotation is the cobra.Command.Annotations key naming the
// word a "cmdList" group's own trailer uses in place of "command" —
// "brief new"'s own trailer reads "Run 'brief new <type> --help' for
// details." because "new" carries this annotation with value "type";
// root carries none, so helpTemplate falls back to the literal "command".
const commandNounAnnotation = "commandNoun"

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
// pflag's default type name ("string"). Its embedded newline is pflag's
// own wrapping cue, the same convention jsonFlagUsage uses: FlagUsages
// re-indents it to the table's description column, so every rendered line
// stays within 80 columns.
const handoffFlagUsage = `the ` + "`path`" + ` to the step's handoff body,
written to its own file`

// stateFlagUsage is finish's --state flag's usage string; see
// handoffFlagUsage for the backquoted "path" convention and its embedded
// newlines.
const stateFlagUsage = `the ` + "`path`" + ` to the COMPLETE replacement body for the state
file; it replaces the file, it is never appended to; it
must carry the configured state headings, though a
section may be empty`

// Run parses args, dispatches to the named command, and renders every
// user-facing line to stdout or stderr itself. wd is the working directory
// used to resolve configuration and to relativize any printed path — Run
// never calls os.Getwd. stdin backs "-" arguments on commands that read one
// (finish's --handoff/--state); commands that take no such argument never
// read it. Run delegates to run, passing debug.ReadBuildInfo as the source
// "--version" reads (R3).
func Run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return run(ctx, wd, args, stdin, stdout, stderr, debug.ReadBuildInfo)
}

// run is Run's implementation, taking readBuildInfo as an explicit
// dependency so a test can pin "--version"'s output against a fake build
// info without a real binary (R3, R2's guard against a false ok). It has
// the same signature as debug.ReadBuildInfo: production passes that
// function itself.
func run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer, readBuildInfo func() (*debug.BuildInfo, bool)) error {
	// cobra's RunE has no context.Context parameter; every closure below
	// reads it via cmd.Context(), which ExecuteContext(ctx) sets on the
	// resolved command before RunE runs. contextcheck cannot see that
	// guarantee through cobra's own dispatch and instead flags the
	// context.Background() fallback inside Command.Context()'s body.
	root := newRootCommand(wd, stdin, stdout, stderr, readBuildInfo) //nolint:contextcheck

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
// leak one invocation's flags into the next. readBuildInfo threads through
// to runRoot's "--version" arm unchanged; nothing else in the tree reads
// it.
//
// The root and "new" disable cobra's flag parsing and resolve their first
// argument themselves, via runRoot and runNew, so a missing or unknown
// command or type — "-x" included — keeps its own one-line usage error
// instead of cobra's default dispatch. runNew routes a sole "-h"/"--help"
// argument to cmd.Help() itself, since cobra's own help check never runs
// under disabled flag parsing; alongside any other argument it falls
// through to the unknown-type error. The leaves let cobra (via pflag)
// parse flags and report an undefined one in pflag's own words; the root
// FlagErrorFunc rewrites that into brief's one-line usage error, naming
// the invocation carried in the leaf's Annotations.
//
// Every command in the tree is Runnable with Args: cobra.ArbitraryArgs, so
// cobra never rejects an argument count itself — every run* function does
// its own counting and reports brief's own usage error.
//
// Commands are added in the order they should list in root help and in
// every "expected one of:" message — new, start, finish, status, check —
// not alphabetically: see this package's init, which turns cobra's default
// sort off, and expectedCommandList, which reads root.Commands() in that
// same order. "completion" registers last: it is Hidden (enabled and
// dispatchable, but excluded from expectedCommandList, which filters on
// IsAvailableCommand alone) and carries listedInHelpAnnotation instead, so
// it still gets a root-help row and remains a valid "brief help" topic.
func newRootCommand(wd string, stdin io.Reader, stdout, stderr io.Writer, readBuildInfo func() (*debug.BuildInfo, bool)) *cobra.Command {
	root := &cobra.Command{
		Use:                "brief",
		Long:               rootShort,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args, stdout, stderr, readBuildInfo)
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true

	newCmd := &cobra.Command{
		Use:                "new",
		Short:              newShort,
		Long:               newLong,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		Annotations:        map[string]string{commandNounAnnotation: "type"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, stderr)
		},
	}
	newCmd.AddCommand(
		leafCommand("feature <name>", "scaffold a new feature's specification and state file", newFeatureInvocation, newFeatureLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewFeature(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("step <feature>", "scaffold the next step file and its progress entry", newStepInvocation, newStepLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runNewStep(cmd.Context(), wd, args, stdout, stderr)
			}),
	)

	root.AddCommand(
		newCmd,
		leafCommand("start [--json] <feature>", "print the next open step's context", startInvocation, startLong,
			func(fs *pflag.FlagSet) {
				fs.Bool("json", false, jsonFlagUsage)
			},
			func(cmd *cobra.Command, args []string) error {
				jsonOut, _ := cmd.Flags().GetBool("json")

				return runStart(cmd.Context(), wd, args, jsonOut, stdout, stderr)
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
		leafCommand("status", "print one done/total/next/blocked line per feature", statusInvocation, statusLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runStatus(cmd.Context(), wd, args, stdout, stderr)
			}),
		leafCommand("check [feature]", "report faults finish would now refuse to write over", checkInvocation, checkLong, nil,
			func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd.Context(), wd, args, stdout, stderr)
			}),
	)

	completionCmd := leafCommand(
		"completion <bash|zsh|fish|powershell>",
		"print a shell completion script",
		completionInvocation,
		completionLong,
		nil,
		func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, args, stdout, stderr)
		},
	)
	completionCmd.Hidden = true
	completionCmd.Annotations[listedInHelpAnnotation] = "true"
	root.AddCommand(completionCmd)

	root.SetFlagErrorFunc(newFlagErrorFunc(stderr))

	root.SetHelpTemplate(helpTemplate)
	root.SetHelpCommand(newHelpCommand(stderr))

	return root
}

// helpShort is the help stub's own one-line description, used only for
// go doc: the stub is Hidden and carries no listedInHelpAnnotation, so it
// never gets a root-help row and this Short never renders.
const helpShort = "print help for a command"

// helpLong is the help stub's own prose, rendered by helpTemplate's leaf
// branch when "brief help" is asked for its own help — see
// newHelpCommand's sole-argument case.
const helpLong = `Prints help for a command. 'brief help <command>' prints the same text as
'brief <command> --help'; with no command it prints the overview.`

// newHelpCommand builds the hidden "help" stub that replaces cobra's
// default help command, which on an unknown topic calls cobra.CheckErr and
// os.Exit(1) directly — the only exit this package allows is ExitCode,
// called from main. The stub resolves its topic against the tree with
// Find, which never errors on this ArbitraryArgs-everywhere tree and
// returns the unstripped residual as its second value. A topic is accepted
// only when that residual is empty and the resolved target is root itself
// (a bare "brief help"), IsAvailableCommand, or carries
// listedInHelpAnnotation — so a leftover positional, a flag left after the
// topic, a flag ahead of it (Find stops at root, treating the topic as
// that flag's value) and a hidden-and-unlisted command such as "help"
// itself are all rejected, not silently routed to some leaf's help. A
// rejected topic is brief's own usage error naming the whole topic as
// typed — every argument joined by a space, not just the unresolved
// residual — so "help new bogus" names "new bogus", not a false top-level
// command "bogus". An accepted topic renders byte-identical to "<path…>
// --help": InitDefaultHelpFlag backfills the -h/--help row that Execute()
// would otherwise add during ordinary dispatch, which Find alone skips.
//
// A leading "-h"/"--help" (with or without an attached value) or any other
// dash-prefixed topic is never resolved against the tree at all: args[0]
// is classified the same way runRoot and runNew classify theirs, before
// Find ever runs. Unlike runRoot and runNew, a sole "-h"/"--help" (or an
// all-'h' cluster, "-hh" and so on) prints the help stub's own usage —
// its Use, Long and Flags table, set via helpShort/helpLong below — rather
// than routing to some topic's help; asking "brief help" for help on
// itself is exactly the sole-argument case a bare "brief help" already
// answers, so "help -h" is that same answer, not an error. Alongside any
// other argument the help flag still takes no arguments, the same as
// runRoot and runNew report for that shape. "--" classifies as argNotFlag,
// so "brief help --" falls through to Find like any other topic and is
// rejected as an unresolved one.
func newHelpCommand(stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                   "help [command]",
		Short:                 helpShort,
		Long:                  helpLong,
		Hidden:                true,
		DisableFlagsInUseLine: true,
		DisableFlagParsing:    true,
		Args:                  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				switch kind, msg := classifyDashArg(args[0]); kind {
				case argHelpFlagWithValue:
					return usageError(stderr, fmt.Sprintf("brief help: '%s' takes no value; run 'brief help <command>'", msg))
				case argHelpFlag:
					if len(args) == 1 {
						cmd.InitDefaultHelpFlag()

						return cmd.Help()
					}

					return usageError(stderr, fmt.Sprintf("brief help: '%s' takes no arguments; run 'brief help <command>'", args[0]))
				case argUnknownFlag, argVersionFlag:
					return usageError(stderr, fmt.Sprintf("brief help: %s; run 'brief help <command>'", msg))
				case argNotFlag:
				}
			}

			target, residual, _ := cmd.Root().Find(args)
			listed := target.Annotations[listedInHelpAnnotation] != ""
			if len(residual) > 0 || (target != cmd.Root() && !target.IsAvailableCommand() && !listed) {
				return usageError(stderr, fmt.Sprintf("brief help: unknown command %q; expected one of: %s", strings.Join(args, " "), expectedCommandList(cmd)))
			}

			target.InitDefaultHelpFlag()

			return target.Help()
		},
	}
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
// nothing at all, "--help"/"-h" given a value, a sole "-h"/"--help", a
// "-h"/"--help" alongside another argument, a sole "--version", some other
// dash-prefixed token, "--" (pflag's flag-parsing terminator, never a flag
// itself), or an unknown command name. Cobra intercepts "help" as a
// dispatch to the tree's own help command (see newRootCommand's
// SetHelpCommand) before this ever runs, so this function never sees
// "help" as args[0].
func runRoot(cmd *cobra.Command, args []string, stdout, stderr io.Writer, readBuildInfo func() (*debug.BuildInfo, bool)) error {
	if len(args) == 0 {
		return usageError(stderr, "brief: no command given; expected one of: "+expectedCommandList(cmd))
	}

	switch kind, msg := classifyDashArg(args[0]); kind {
	case argHelpFlagWithValue:
		return usageError(stderr, fmt.Sprintf("brief: '%s' takes no value; run 'brief --help'", msg))
	case argHelpFlag:
		if len(args) == 1 {
			return cmd.Help()
		}

		return usageError(stderr, fmt.Sprintf("brief: '%s' takes no arguments; run 'brief help <command>'", args[0]))
	case argVersionFlag:
		if len(args) == 1 {
			fmt.Fprintln(stdout, versionLine(readBuildInfo))

			return nil
		}

		return usageError(stderr, fmt.Sprintf("brief: %s; run 'brief <command> --help'", msg))
	case argUnknownFlag:
		return usageError(stderr, fmt.Sprintf("brief: %s; run 'brief <command> --help'", msg))
	case argNotFlag:
	}

	return usageError(stderr, fmt.Sprintf("brief: unknown command %q; expected one of: %s", args[0], expectedCommandList(cmd)))
}

// versionLine renders "--version"'s stdout line, prefix included but the
// trailing newline excluded: "brief " followed by readBuildInfo's
// Main.Version verbatim (R1).
func versionLine(readBuildInfo func() (*debug.BuildInfo, bool)) string {
	info, _ := readBuildInfo()

	return "brief " + info.Main.Version
}

// usageError writes msg, followed by a single newline, to stderr and
// returns an error satisfying errors.Is(err, ErrUsage).
func usageError(stderr io.Writer, msg string) error {
	fmt.Fprintln(stderr, msg)

	return fmt.Errorf("%s: %w", msg, ErrUsage)
}

// newFlagErrorFunc builds the one root SetFlagErrorFunc frame every leaf's
// pflag.Parse error passes through: boolFlagParseMessage's rewrite when
// err qualifies, else err's own text, always flattened to one line, named
// alongside the failing command's path and its invocation.
func newFlagErrorFunc(stderr io.Writer) func(*cobra.Command, error) error {
	return func(cmd *cobra.Command, err error) error {
		path := strings.TrimPrefix(cmd.CommandPath(), "brief ")
		invocation := cmd.Annotations[invocationAnnotation]

		msg, ok := boolFlagParseMessage(err)
		if !ok {
			msg = err.Error()
		}

		return usageError(stderr, fmt.Sprintf("brief %s: %s; run '%s'", path, flattenOneLine(msg), invocation))
	}
}

// boolFlagParseMessage reports whether err is pflag's *InvalidValueError
// for a bool-typed flag — "--json=maybe", or any other value
// strconv.ParseBool rejects — and, when it is, brief's own replacement for
// pflag's raw strconv wording, naming the value exactly as given (including
// "" for "--json=") and the flag alone, generalized to any bool flag rather
// than hard-coded to one.
func boolFlagParseMessage(err error) (string, bool) {
	var invalid *pflag.InvalidValueError
	if !errors.As(err, &invalid) || invalid.GetFlag().Value.Type() != "bool" {
		return "", false
	}

	return fmt.Sprintf("invalid value %q for --%s (want true or false, or no value)", invalid.GetValue(), invalid.GetFlag().Name), true
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
