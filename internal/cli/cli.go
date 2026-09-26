package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
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
	// out — new, start, finish, status, check, init, doctor, uninstall —
	// rather than cobra's default alphabetical sort. EnableCommandSorting is a cobra package global:
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

// rootLifecycleParagraph is root's second Long paragraph, naming the
// feature lifecycle from opening a feature through its last step.
const rootLifecycleParagraph = `A feature is a specification, ordered step files and one state file. Open one
with 'brief new feature', write its specification, add steps with
'brief new step', then take each step from 'brief start' to 'brief finish'.`

// resolveRoot resolves wd's configuration and the directory every path in
// that configuration is relative to: source's directory when a config file
// was found, wd itself otherwise. Every command that touches configuration
// or the repository tree shares this pattern; the caller still renders its
// own out.refusal(err) on a non-nil error, since the command that refusal
// renders against differs per caller. rootFS is nil in production (every
// run* function reads through config.Resolve, real disk); a test's
// withRootFS runSeam substitutes an rwfs.Mem, read through resolveRootFS
// instead — the same nil-means-real-disk contract runDoctor's own
// rootFS parameter already carries.
func resolveRoot(rootFS fs.FS, wd string) (config.Config, string, error) {
	var (
		cfg    config.Config
		source string
		err    error
	)

	if rootFS != nil {
		cfg, source, err = resolveRootFS(rootFS, wd)
	} else {
		cfg, source, err = config.Resolve(wd)
	}

	if err != nil {
		return config.Config{}, "", err
	}

	root := wd
	if source != "" {
		root = filepath.Dir(source)
	}

	return cfg, root, nil
}

// resolveRootFS is resolveRoot's own fsys-backed twin, mirroring
// config.Resolve exactly — including its unbounded walk. Resolve never
// applies repo's own git-repository boundary the way config.LocateInRepo
// does for doctor, init and uninstall: new, finish, start, check and
// status all resolve configuration the unbounded way, so this never calls
// repo.RootFS either; doing so would silently stop finding a config above
// the enclosing git repository that Resolve itself would still find.
func resolveRootFS(fsys fs.FS, wd string) (config.Config, string, error) {
	abs, err := filepath.Abs(wd)
	if err != nil {
		return config.Config{}, "", fmt.Errorf("resolve config: %w", err)
	}

	nearest, _, err := config.LocateWithinFS(fsys, abs, "")
	if err != nil {
		return config.Config{}, "", fmt.Errorf("resolve config: %w", err)
	}

	if nearest == "" {
		return config.Default(), "", nil
	}

	cfg, violations, err := config.InspectFS(fsys, nearest)
	if err != nil {
		if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
			return config.Config{}, "", fmt.Errorf("resolve config: %w", invalidCfg)
		}

		return config.Config{}, "", fmt.Errorf("resolve config: %s: %w", nearest, err)
	}

	if len(violations) > 0 {
		return config.Config{}, "", fmt.Errorf("resolve config: %w", &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return cfg, nearest, nil
}

// fsName maps abs, an absolute OS path, onto the name a withRootFS fixture
// expects: the leading path separator stripped, forward-slash separated,
// "." for the root itself. Duplicated from internal/scaffold's,
// internal/assemble's, internal/setup's, internal/platform/config's and
// internal/doctor's own identical helper, following this codebase's own
// precedent of copying an eight-line mapping rather than sharing it across
// packages with no other reason to depend on one another. finish.go's own
// readSource is the one call site in this package.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
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
// "command" when the command carries none (root). Root's group body — and
// only root's, gated on HasParent being false — ends with one further
// line, "Run '<command path> --version' to print the installed version.":
// "new"'s group body and every leaf's Flags-table body end at the trailer
// above instead. Only cobra's built-in template funcs (rpad, trim,
// trimTrailingWhitespaces, index) and text/template builtins (or, if) are
// used — no package-global AddTemplateFunc.
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
{{if not .HasParent}}Run '{{.CommandPath}} --version' to print the installed version.
{{end -}}
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

// writesFilesAnnotation is the cobra.Command.Annotations key marking a
// command whose successful run can modify the tree — "new", "new
// feature", "new step", "finish" and "init" — so filesChangedFor knows
// files_changed is false (not null) on a usage error or a refusal that
// changed nothing for one of these, true when at least one write landed
// before the failure, and null for every other command.
const writesFilesAnnotation = "writesFiles"

// jsonFlagUsage is every JSON-capable command's own --json flag's usage
// string, shown in its Flags table: one ruled line, the same wording on
// every command so the help index's own "usage" field can never drift
// from what a command's rendered text carries.
const jsonFlagUsage = "print one JSON document on stdout"

// addJSONFlag registers --json, with jsonFlagUsage's ruled wording, on fs
// — the one call every JSON-capable leaf's addFlags makes, so the flag
// renders identically in every Flags table and every help-index entry.
// leafCommand itself does not call this: completion is built through it,
// and never advertises --json.
func addJSONFlag(fs *pflag.FlagSet) {
	fs.Bool("json", false, jsonFlagUsage)
}

// jsonParagraphWidth is the column every JSON-capable command's trailing
// JSON paragraph is hand-wrapped to, the same 80-column budget every other
// hand-wrapped usage string in this package already keeps.
const jsonParagraphWidth = 80

// jsonParagraphHeaderClause is every JSON-capable command's own opening
// clause, wrapped once and reused verbatim: it names the common header —
// schema, command, ok, exit_code; on a usage error or refusal an error
// object carries the failure — the same way in every command, so only the
// field-key sentence jsonFieldsParagraph appends ever varies per command.
// Test_every_command_help_names_its_json_documents_top_level_fields slices
// from this clause's own first line to isolate a command's field-key
// sentence from the rest of that command's prose.
var jsonParagraphHeaderClause = wrapWords(
	"With --json, this command writes one JSON document on stdout: the common header "+
		"(`schema`, `command`, `ok`, `exit_code`; on a usage error or refusal an `error` "+
		"object carries the failure), then its own top-level fields, in document order:",
	jsonParagraphWidth,
)

// jsonFieldsParagraph builds one command's trailing JSON paragraph:
// jsonParagraphHeaderClause, then keys — that command's own top-level
// fields, in document order — backquoted, comma-joined and wrapped
// separately from the header clause, so the header clause's own line
// breaks never depend on what follows it.
func jsonFieldsParagraph(keys ...string) string {
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = "`" + k + "`"
	}

	return jsonParagraphHeaderClause + "\n" + wrapWords(strings.Join(quoted, ", ")+".", jsonParagraphWidth)
}

// jsonScriptHint is the sentence status and check's own Long end with,
// right after their JSON paragraph — no other command carries it, since
// only their text-mode output's own layout is unstable release to
// release.
const jsonScriptHint = "For scripts, use --json; the text layout may change."

// wrapWords greedily wraps text's whitespace-separated words onto lines no
// longer than width, never splitting a word itself — the one mechanical
// wrap this package uses instead of guessing break points by hand or by
// regex.
func wrapWords(text string, width int) string {
	words := strings.Fields(text)
	lines := make([]string, 0, len(words))

	var cur string

	for _, w := range words {
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}

	if cur != "" {
		lines = append(lines, cur)
	}

	return strings.Join(lines, "\n")
}

// handoffFlagUsage is finish's --handoff flag's usage string. The
// backquoted "path" is pflag's own convention (UnquoteUsage): it names the
// flag's value in its Flags table row ("--handoff path") instead of
// pflag's default type name ("string"). Its embedded newline is pflag's
// own wrapping cue: FlagUsages re-indents it to the table's description
// column, so every rendered line stays within 80 columns.
const handoffFlagUsage = `the ` + "`path`" + ` to the step's handoff body,
written to its own file`

// stateFlagUsage is finish's --state flag's usage string; see
// handoffFlagUsage for the backquoted "path" convention and its embedded
// newlines.
const stateFlagUsage = `the ` + "`path`" + ` to the COMPLETE replacement body for the state
file; it replaces the file, it is never appended to; it
must carry the configured state headings, though a
section may be empty`

// hostFlagUsage is init's --host flag's usage string. The backquoted
// "name" is pflag's own placeholder convention (see handoffFlagUsage) —
// unlike an earlier draft that backquoted "none", one of the two accepted
// values, which pflag then rendered as the flag's own table placeholder
// ("--host none") instead of a generic one.
const hostFlagUsage = "the agent host `name` to install for: claude-code or none\n(default: detected)"

// printFlagUsage is init's --print flag's usage string.
const printFlagUsage = "print each pending file to stdout instead of\nwriting it (cannot be combined with --dry-run)"

// noHookFlagUsage is init's --no-hook flag's usage string. Unlike
// --with-agents, --no-hook is never refused under --host none — with no
// plugin to omit a hook from, it is a documented no-op there, since an
// empty InitRequest.Host may still resolve to claude-code by detection,
// and a value the caller cannot predict in advance is a poor thing to
// make a usage error turn on.
const noHookFlagUsage = "install the plugin without its PostToolUse hook\n(no effect with --host none)"

// withAgentsFlagUsage is init's --with-agents flag's usage string. Its
// continuation line, like handoffFlagUsage's, wraps via an embedded
// newline — but unlike handoffFlagUsage's, it never starts that line with
// "--": a flag usage's own rendered continuation is otherwise
// indistinguishable from a second flag definition row to a text-table
// scraper.
const withAgentsFlagUsage = "install the three role agents (the resolved\nhost must be claude-code)"

// editAgentsFlagUsage is init's --edit-agents flag's usage string. Its
// value is double-quoted, never backticked — pflag turns a backticked
// word into a placeholder, which "skills:" and "brief-workflow" must
// never become. Wrapped across three lines: two alone pushes the second
// past the 80-column budget once pflag indents it under
// "--edit-agents"'s own column.
const editAgentsFlagUsage = "add \"brief-workflow\" to the \"skills:\" list of the planner\n" +
	"and implementer agents bound in .brief.yaml\n(repository files only)"

// hookFlagUsage is check's --hook flag's usage string. Its embedded newline
// is pflag's own wrapping cue — see handoffFlagUsage. The backquoted "host"
// is the generic placeholder (see hostFlagUsage); host.HookHosts() names
// only "claude-code" today, so that is what the parenthetical states.
const hookFlagUsage = "read a `host` hook payload from stdin and check only the\nedited feature (claude-code only)"

// dryRunFlagUsage is init's --dry-run flag's usage string.
const dryRunFlagUsage = "print the plan without writing anything"

// forceFlagUsage is init's --force flag's usage string.
const forceFlagUsage = "rewrite an existing .brief.yaml from defaults"

// uninstallDryRunFlagUsage is uninstall's --dry-run flag's usage string.
const uninstallDryRunFlagUsage = "print the plan without removing anything"

// runSeams collects every environment seam a test can override on a call to
// run or newRootCommand, gathered from a trailing ...runSeam so neither
// signature grows a dedicated parameter per package that needs one.
type runSeams struct {
	doctorOpts []doctor.Option
	setupOpts  []setup.Option
	rootFS     rwfs.FS
}

// runSeam configures one field of a runSeams collector. withDoctorOpts and
// withSetupOpts are the two constructors; resolveRunSeams folds a
// ...runSeam argument list into one runSeams value.
type runSeam func(*runSeams)

// withDoctorOpts appends opts to a runSeams' own doctorOpts, passed to
// runDoctor after its own doctor.WithVersion — a test overriding
// WithVersion this way still wins.
func withDoctorOpts(opts ...doctor.Option) runSeam {
	return func(s *runSeams) { s.doctorOpts = append(s.doctorOpts, opts...) }
}

// withSetupOpts appends opts to a runSeams' own setupOpts, passed to
// setup.NewServer inside runInit — a test injects setup.WithHomeDir this
// way so host detection never depends on the developer's own
// os.UserHomeDir.
func withSetupOpts(opts ...setup.Option) runSeam {
	return func(s *runSeams) { s.setupOpts = append(s.setupOpts, opts...) }
}

// withRootFS sets a runSeams' own rootFS, read by resolveRoot (runNewFeature,
// runNewStep, runFinish, runStart, runCheck and runStatus — the six that
// call it) in place of config.Resolve(wd), by runDoctor's own
// config-location pre-check (locateInRepoFS) in place of
// config.LocateInRepo(wd), and by finish's own readSource in place of
// os.ReadFile for a non-"-" --handoff/--state argument — a test injects
// the same rwfs.Mem it also passed to withDoctorOpts(doctor.WithRootFS(...)),
// scaffold.WithFS and assemble.WithFS, so every seamed package reads one
// fixture. nil (the zero value, production's own default) means "read
// real disk", identical to before this seam existed. rwfs.FS's read side
// is exactly fs.FS plus four more interfaces (rwfs/fs.go), so the same
// value satisfies every fs.FS-typed parameter this seam feeds (resolveRoot,
// runDoctor) as well as scaffold's and assemble's own rwfs.FS-typed WithFS.
func withRootFS(fsys rwfs.FS) runSeam {
	return func(s *runSeams) { s.rootFS = fsys }
}

// resolveRunSeams folds seams into one runSeams value, applied in order.
func resolveRunSeams(seams []runSeam) runSeams {
	var rs runSeams

	for _, s := range seams {
		s(&rs)
	}

	return rs
}

// Run parses args, dispatches to the named command, and renders every
// user-facing line to stdout or stderr itself. wd is the working directory
// used to resolve configuration and to relativize any printed path — Run
// never calls os.Getwd. stdin backs "-" arguments on commands that read one
// (finish's --handoff/--state) and check --hook's own payload read;
// commands that read neither never read it. Run delegates to run, passing
// debug.ReadBuildInfo as the source "--version" reads.
func Run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return run(ctx, wd, args, stdin, stdout, stderr, debug.ReadBuildInfo)
}

// run is Run's implementation, taking readBuildInfo as an explicit
// dependency so a test can pin "--version"'s output against a fake build
// info without a real binary, including a build info reported with ok=false. It has
// the same signature as debug.ReadBuildInfo: production passes that
// function itself. seams is a trailing seam letting a test override
// doctor's own environment seams (WithLookPath, WithExecutable,
// WithBinaryVersion, WithVersion, WithRootFS, WithHomeTree, via
// withDoctorOpts), setup's own (WithHomeDir, WithFSRoot, WithResolveRoot,
// WithWritableCheck, via withSetupOpts), or the root FS resolveRoot,
// runDoctor's own config-location pre-check and finish's own readSource
// each read in place of real disk (withRootFS; see its own doc comment
// for the full list of what it feeds) — without a new run overload —
// every existing call site compiles unchanged, since a trailing variadic
// is optional.
//
// --json detection runs here, ahead of cobra entirely: scanJSONFlag scans
// args for an exact "--json" token before the first "--", strips every
// one it finds, and reports whether a "--json=<v>" token was seen. The
// stripped args are all cobra, and every command below it, ever sees —
// "--json" never reaches pflag, so a leaf that also registers it (only
// "start" does, for its help table) never has to read its value. A
// "--json=<v>" token is always a text usage error, reported here before
// ExecuteContext ever runs so it wins over every other usage error on the
// line; root.InitDefaultHelpCmd registers the help stub as a real child
// so root.Find can resolve "help" the same way ExecuteContext's own
// dispatch would.
func run(ctx context.Context, wd string, args []string, stdin io.Reader, stdout, stderr io.Writer, readBuildInfo func() (*debug.BuildInfo, bool), seams ...runSeam) error {
	strippedArgs, jsonMode, hasJSONValue := scanJSONFlag(args)

	var helpFailure error

	out := reporter{stdout: stdout, stderr: stderr, json: jsonMode, wd: wd, helpFailure: &helpFailure}

	// cobra's RunE has no context.Context parameter; every closure below
	// reads it via cmd.Context(), which ExecuteContext(ctx) sets on the
	// resolved command before RunE runs. contextcheck cannot see that
	// guarantee through cobra's own dispatch and instead flags the
	// context.Background() fallback inside Command.Context()'s body.
	root := newRootCommand(wd, stdin, out, readBuildInfo, seams...) //nolint:contextcheck
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.InitDefaultHelpCmd()

	if hasJSONValue {
		target, _, _ := root.Find(strippedArgs)

		return usageError(stderr, jsonTakesNoValueMessage(target))
	}

	argsCopy := make([]string, len(strippedArgs))
	copy(argsCopy, strippedArgs)
	root.SetArgs(argsCopy)

	if err := root.ExecuteContext(ctx); err != nil {
		return err
	}

	return helpFailure
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
// every "expected one of:" message — new, start, finish, status, check,
// init, doctor, uninstall — not alphabetically: see this package's init,
// which turns cobra's default sort off, and expectedCommandList, which
// reads root.Commands() in that same order. "completion" registers last: it is
// Hidden (enabled and dispatchable, but excluded from
// expectedCommandList, which filters on IsAvailableCommand alone) and
// carries listedInHelpAnnotation instead, so it still gets a root-help
// row and remains a valid "brief help" topic.
//
// seams is resolved once (resolveRunSeams) into doctorOpts, setupOpts and
// rootFS: doctorOpts threads through unchanged to the "doctor" leaf's own
// RunE, appended after runDoctor's own doctor.WithVersion; setupOpts
// threads through to "init"'s and "uninstall"'s own RunE, passed to
// runInit's and runUninstall's own extraSetupOpts; rootFS threads to seven
// of the eleven leaves' own RunE — "doctor" alongside doctorOpts, read by
// runDoctor's own config-location pre-check (locateInRepoFS) in place of
// real disk, and "new feature"/"new step"/"finish"/"start"/"status"/"check"
// (never "check --hook", which stays real-disk-only regardless — see
// runCheckHook's own doc comment; "init", "uninstall" and "completion"
// never receive it either, reading their own seams or none at all), each
// passing it to resolveRoot in place of config.Resolve and to its own
// scaffold.WithFS/assemble.WithFS when constructing their own Server —
// "finish" alone also passes it to readSource in place of os.ReadFile —
// see run's own doc comment.
//
// One root.SetHelpFunc wrapper backs every help document: root --help, the
// help stub, runNew's sole-help arm and every leaf's own --help all reach
// cmd.Help(), which walks up an unset per-command helpFunc to whichever
// ancestor last called SetHelpFunc — root, here — so no other call site
// ever renders help on its own. In text mode the wrapper defers to
// defaultHelpFunc, captured from root.HelpFunc() before SetHelpFunc
// replaces it (capturing after would recurse into the wrapper itself); in
// JSON mode it writes helpIndex(root) when cmd is root itself, else
// helpEntry(cmd) alone — the index-membership predicate (listedForHelp) and
// "the command asked about" are two different questions, so a target that
// fails the former (the help stub itself, under "help -h --json") still
// gets its own one-entry document. Every help document's own "command"
// field is the literal "help", never the described command's path: see
// newHelpDocument.
func newRootCommand(wd string, stdin io.Reader, out reporter, readBuildInfo func() (*debug.BuildInfo, bool), seams ...runSeam) *cobra.Command {
	rs := resolveRunSeams(seams)

	root := &cobra.Command{
		Use:                "brief",
		Long:               rootShort + "\n\n" + rootLifecycleParagraph,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args, out.forCommand(cmd), readBuildInfo)
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true

	newCmd := &cobra.Command{
		Use:                   "new",
		Short:                 newShort,
		Long:                  newLong,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		Args:                  cobra.ArbitraryArgs,
		Annotations:           map[string]string{commandNounAnnotation: "type", writesFilesAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, out.forCommand(cmd))
		},
	}

	newFeatureCmd := leafCommand("feature <name>", "scaffold a new feature's specification and state file", newFeatureInvocation, newFeatureLong, addJSONFlag,
		func(cmd *cobra.Command, args []string) error {
			return runNewFeature(cmd.Context(), wd, args, out.forCommand(cmd), rs.rootFS)
		})
	newFeatureCmd.Annotations[writesFilesAnnotation] = "true"

	newStepCmd := leafCommand("step <feature>", "scaffold the next step file and its progress entry", newStepInvocation, newStepLong, addJSONFlag,
		func(cmd *cobra.Command, args []string) error {
			return runNewStep(cmd.Context(), wd, args, out.forCommand(cmd), rs.rootFS)
		})
	newStepCmd.Annotations[writesFilesAnnotation] = "true"

	newCmd.AddCommand(newFeatureCmd, newStepCmd)

	finishCmd := leafCommand("finish <feature> <step> --handoff <path> --state <path>", "close a step: handoff, state, then done", finishInvocation, finishLong,
		func(fs *pflag.FlagSet) {
			fs.String("handoff", "", handoffFlagUsage)
			fs.String("state", "", stateFlagUsage)
			addJSONFlag(fs)
		},
		func(cmd *cobra.Command, args []string) error {
			handoffPath, _ := cmd.Flags().GetString("handoff")
			statePath, _ := cmd.Flags().GetString("state")

			return runFinish(cmd.Context(), wd, args, handoffPath, statePath, stdin, out.forCommand(cmd), rs.rootFS)
		})
	finishCmd.Annotations[writesFilesAnnotation] = "true"

	const initUse = "init [--host <name>] [--no-hook] [--with-agents] [--edit-agents] [--dry-run | --print] [--force] [--json]"

	initCmd := leafCommand(initUse, "install brief's config and agent-host integration", initInvocation, initLong,
		func(fs *pflag.FlagSet) {
			fs.String("host", "", hostFlagUsage)
			fs.Bool("no-hook", false, noHookFlagUsage)
			fs.Bool("with-agents", false, withAgentsFlagUsage)
			fs.Bool("edit-agents", false, editAgentsFlagUsage)
			fs.Bool("dry-run", false, dryRunFlagUsage)
			fs.Bool("print", false, printFlagUsage)
			fs.Bool("force", false, forceFlagUsage)
			addJSONFlag(fs)
		},
		func(cmd *cobra.Command, args []string) error {
			host, _ := cmd.Flags().GetString("host")
			noHook, _ := cmd.Flags().GetBool("no-hook")
			withAgents, _ := cmd.Flags().GetBool("with-agents")
			editAgents, _ := cmd.Flags().GetBool("edit-agents")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			printFlag, _ := cmd.Flags().GetBool("print")
			force, _ := cmd.Flags().GetBool("force")

			return runInit(cmd.Context(), wd, args, host, noHook, withAgents, editAgents, dryRun, printFlag, force, out.forCommand(cmd), rs.setupOpts...)
		})
	initCmd.Annotations[writesFilesAnnotation] = "true"

	uninstallCmd := leafCommand("uninstall [--host <name>] [--dry-run] [--force] [--json]", "remove what init installed", uninstallInvocation, uninstallLong,
		func(fs *pflag.FlagSet) {
			fs.String("host", "", uninstallHostFlagUsage)
			fs.Bool("dry-run", false, uninstallDryRunFlagUsage)
			fs.Bool("force", false, uninstallForceFlagUsage)
			addJSONFlag(fs)
		},
		func(cmd *cobra.Command, args []string) error {
			host, _ := cmd.Flags().GetString("host")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")

			return runUninstall(cmd.Context(), wd, args, host, dryRun, force, out.forCommand(cmd), rs.setupOpts...)
		})
	uninstallCmd.Annotations[writesFilesAnnotation] = "true"

	root.AddCommand(
		newCmd,
		leafCommand("start [--json] <feature>", "print the next open step's context", startInvocation, startLong,
			addJSONFlag,
			func(cmd *cobra.Command, args []string) error {
				return runStart(cmd.Context(), wd, args, out.forCommand(cmd), rs.rootFS)
			}),
		finishCmd,
		leafCommand("status", "print a FEATURE/DONE/BLOCKED/NEXT table of every feature", statusInvocation, statusLong, addJSONFlag,
			func(cmd *cobra.Command, args []string) error {
				return runStatus(cmd.Context(), wd, args, out.forCommand(cmd), rs.rootFS)
			}),
		leafCommand("check [feature] [--hook <host>]", "report faults finish would now refuse to write over", checkInvocation, checkLong,
			func(fs *pflag.FlagSet) {
				fs.String("hook", "", hookFlagUsage)
				addJSONFlag(fs)
			},
			func(cmd *cobra.Command, args []string) error {
				hook, _ := cmd.Flags().GetString("hook")

				return runCheck(cmd.Context(), wd, args, hook, stdin, out.forCommand(cmd), rs.rootFS)
			}),
		initCmd,
		leafCommand("doctor [--json]", "check brief's setup: config, feature root, host integration", doctorInvocation, doctorLong, addJSONFlag,
			func(cmd *cobra.Command, args []string) error {
				return runDoctor(cmd.Context(), wd, args, readBuildInfo, out.forCommand(cmd), rs.rootFS, rs.doctorOpts...)
			}),
		uninstallCmd,
	)

	completionCmd := leafCommand(
		"completion <bash|zsh|fish|powershell>",
		"print a shell completion script",
		completionInvocation,
		completionLong,
		nil,
		func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, args, out.forCommand(cmd))
		},
	)
	completionCmd.Hidden = true
	completionCmd.Annotations[listedInHelpAnnotation] = "true"
	root.AddCommand(completionCmd)

	root.SetFlagErrorFunc(newFlagErrorFunc(out))

	root.SetHelpTemplate(helpTemplate)

	// Captured before SetHelpFunc: root.HelpFunc(), with no helpFunc set on
	// any command yet, returns cobra's own default closure, which renders
	// whichever *Command it is handed rather than being bound to root —
	// calling root.HelpFunc() again from inside the wrapper below would
	// instead return the wrapper itself and recurse forever.
	defaultHelpFunc := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !out.json {
			defaultHelpFunc(cmd, args)

			return
		}

		doc := newHelpDocument()
		if cmd == root {
			doc.Commands = helpIndex(root)
		} else {
			doc.Commands = []helpCommandJSON{helpEntry(cmd)}
		}

		// HelpFunc's own signature returns nothing, and cmd.Help() always
		// returns nil, so a write failure is reported here and parked in
		// out.helpFailure for run to return.
		if err := writeJSONDocument(out.stdout, doc); err != nil {
			*out.helpFailure = helpStdoutFailure(out, cmd, err)
		}
	})

	root.SetHelpCommand(newHelpCommand(out))

	return root
}

// helpShort is the help stub's own one-line description, used only for
// go doc: the stub is Hidden and carries no listedInHelpAnnotation, so it
// never gets a root-help row and this Short never renders.
const helpShort = "print help for a command"

// helpLong is the help stub's own prose, rendered by helpTemplate's leaf
// branch when "brief help" is asked for its own help — see
// newHelpCommand's sole-argument case.
var helpLong = `Prints help for a command. 'brief help <command>' prints the same text as
'brief <command> --help'; with no command it prints the overview.

` + jsonFieldsParagraph("commands")

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
//
// Every accepted topic's target.Help() call, and the sole-argument "-h"
// case's own cmd.Help(), reach newRootCommand's one root.SetHelpFunc
// wrapper — under --json this renders the resolved command's own help
// document instead of its text help, target unchanged either way.
func newHelpCommand(out reporter) *cobra.Command {
	helpCmd := &cobra.Command{
		Use:                   "help [command]",
		Short:                 helpShort,
		Long:                  helpLong,
		Hidden:                true,
		DisableFlagsInUseLine: true,
		DisableFlagParsing:    true,
		Args:                  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			reported := out.forCommand(cmd)

			if len(args) > 0 {
				switch kind, msg := classifyDashArg(args[0]); kind {
				case argHelpFlagWithValue:
					return reported.usageError(fmt.Sprintf("brief help: '%s' takes no value; run 'brief help <command>'", msg))
				case argHelpFlag:
					if len(args) == 1 {
						cmd.InitDefaultHelpFlag()

						return cmd.Help()
					}

					return reported.usageError(fmt.Sprintf("brief help: '%s' takes no arguments; run 'brief help <command>'", args[0]))
				case argUnknownFlag, argVersionFlag, argVersionFlagWithValue:
					return reported.usageError(fmt.Sprintf("brief help: %s; run 'brief help <command>'", msg))
				case argNotFlag:
				}
			}

			target, residual, _ := cmd.Root().Find(args)
			if len(residual) > 0 || (target != cmd.Root() && !listedForHelp(target)) {
				return reported.usageError(fmt.Sprintf("brief help: unknown command %q; expected one of: %s", strings.Join(args, " "), expectedCommandList(cmd)))
			}

			target.InitDefaultHelpFlag()

			return target.Help()
		},
	}

	// Registered only for the stub's own help table row — DisableFlagParsing
	// means the stub's dispatch never reads this flag's value; run's own
	// scanJSONFlag strips every "--json" token before RunE ever runs.
	addJSONFlag(helpCmd.Flags())

	return helpCmd
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
// "-h"/"--help" alongside another argument, a sole "--version", "--version"
// alongside another argument, some other dash-prefixed token, "--" (pflag's
// flag-parsing terminator, never a flag itself), or an unknown command
// name. Cobra intercepts "help" as a dispatch to the tree's own help
// command (see newRootCommand's SetHelpCommand) before this ever runs, so
// this function never sees "help" as args[0].
//
// A trailing argument alongside "--version" is reported as that flag taking
// no arguments, naming it exactly as typed and pointing at "brief
// --version" — argVersionFlag's own msg (the unknown-flag wording runNew
// and the help stub fold it into) is not used for this branch. Only
// args[0] decides which flag's "takes no arguments" error fires: a
// "--version" trailing after "--help" reports "--help"'s error, never
// this one. A value on "--version" ("--version=<v>") is reported before any
// trailing argument is even looked at: "--version=x extra" reports the
// value error, not the trailing-argument one.
//
// A sole "--version" under out.json (scanJSONFlag's stripping already
// puts "--version" and "--json" in either order into this same
// len(args)==1 arm) writes versionDocument instead of versionLine's text
// line before returning; the value and trailing-argument arms above stay
// text-only errors in both modes (out.usageError renders those itself).
func runRoot(cmd *cobra.Command, args []string, out reporter, readBuildInfo func() (*debug.BuildInfo, bool)) error {
	if len(args) == 0 {
		return out.usageError("brief: no command given; expected one of: " + expectedCommandList(cmd))
	}

	switch kind, msg := classifyDashArg(args[0]); kind {
	case argHelpFlagWithValue:
		return out.usageError(takesNoValueMessage(msg, "brief --help"))
	case argHelpFlag:
		if len(args) == 1 {
			return cmd.Help()
		}

		return out.usageError(takesNoArgumentsMessage(args[0], "brief help <command>"))
	case argVersionFlag:
		if len(args) == 1 {
			if out.json {
				doc := versionDocument{
					jsonHeader: out.successHeader(),
					Version:    versionString(readBuildInfo),
				}

				if err := writeJSONDocument(out.stdout, doc); err != nil {
					err = stdoutWriteError(err, "brief --version --json", false)
					fmt.Fprintf(out.stderr, "brief --version: %s\n", err)

					return err
				}

				return nil
			}

			fmt.Fprintln(out.stdout, versionLine(readBuildInfo))

			return nil
		}

		return out.usageError(takesNoArgumentsMessage(args[0], "brief --version"))
	case argVersionFlagWithValue:
		return out.usageError(takesNoValueMessage("--version", "brief --version"))
	case argUnknownFlag:
		return out.usageError(fmt.Sprintf("brief: %s; run 'brief <command> --help'", msg))
	case argNotFlag:
	}

	return out.usageError(fmt.Sprintf("brief: unknown command %q; expected one of: %s", args[0], expectedCommandList(cmd)))
}

// versionDocument is "--version --json"'s success document: the common
// header first, then version, versionString's own value — never
// versionLine's "brief "-prefixed one. Asking for the version never
// fails, so this document's exit_code is always 0.
type versionDocument struct {
	jsonHeader

	Version string `json:"version"`
}

// versionString reports readBuildInfo's Main.Version verbatim, or
// "(devel)" when readBuildInfo reports ok=false or an empty Main.Version
// — asking for the version never fails. It is the one version rule both
// versionLine's text line and the JSON "--version" document's "version"
// field share; neither ever computes its own.
func versionString(readBuildInfo func() (*debug.BuildInfo, bool)) string {
	info, ok := readBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}

	return info.Main.Version
}

// versionLine renders "--version"'s stdout line, prefix included but the
// trailing newline excluded: "brief " followed by versionString's value.
func versionLine(readBuildInfo func() (*debug.BuildInfo, bool)) string {
	return "brief " + versionString(readBuildInfo)
}

// takesNoArgumentsMessage renders root's "takes no arguments" usage copy for
// flag — named exactly as typed — pointing the caller at runHint. It backs
// root's argHelpFlag and argVersionFlag arms only: runNew and the help stub
// build their own "takes no arguments" copy inline, with their own prefixes
// and hints.
func takesNoArgumentsMessage(flag, runHint string) string {
	return fmt.Sprintf("brief: '%s' takes no arguments; run '%s'", flag, runHint)
}

// takesNoValueMessage renders root's "takes no value" usage copy for flag
// — named exactly as typed, never the flag's own unknown-flag wording —
// pointing the caller at runHint. It backs root's argHelpFlagWithValue and
// argVersionFlagWithValue arms only: runNew and the help stub build their
// own "takes no value" copy inline, with their own prefixes and hints.
func takesNoValueMessage(flag, runHint string) string {
	return fmt.Sprintf("brief: '%s' takes no value; run '%s'", flag, runHint)
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
// alongside the failing command's path and its invocation, rendered
// through out narrowed to the failing command.
func newFlagErrorFunc(out reporter) func(*cobra.Command, error) error {
	return func(cmd *cobra.Command, err error) error {
		path := commandName(cmd)
		invocation := cmd.Annotations[invocationAnnotation]

		msg, ok := boolFlagParseMessage(err)
		if !ok {
			msg = err.Error()
		}

		return out.forCommand(cmd).usageError(fmt.Sprintf("brief %s: %s; run '%s'", path, flattenOneLine(msg), invocation))
	}
}

// boolFlagParseMessage reports whether err is pflag's *InvalidValueError
// for a bool-typed flag — a leaf's auto-registered "--help=maybe", or any
// other value strconv.ParseBool rejects on a bool flag — and, when it is,
// brief's own replacement for pflag's raw strconv wording, naming the
// value exactly as given (including "" for an explicit empty value) and
// the flag alone, generalized to any bool flag rather than hard-coded to
// one. "--json" itself never reaches pflag: run's own scanJSONFlag strips
// every exact "--json" token before ExecuteContext, and reports a
// "--json=<v>" token as its own, always-text usage error before dispatch
// even begins.
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

// helpStdoutFailure reports a --json help document's failed stdout write
// for cmd on out's stderr as stdoutWriteError's line under "brief help: ",
// re-running the help spelling that names cmd — "brief help --json" for
// root itself — and returns that error.
func helpStdoutFailure(out reporter, cmd *cobra.Command, cause error) error {
	hint := "brief help --json"
	if cmd != cmd.Root() {
		hint = fmt.Sprintf("brief help %s --json", commandName(cmd))
	}

	err := stdoutWriteError(cause, hint, false)
	fmt.Fprintf(out.stderr, "brief help: %s\n", err)

	return err
}
