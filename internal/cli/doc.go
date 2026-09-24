// Package cli is brief's command dispatcher. Run parses argv, resolves
// configuration from a working directory, and drives a feature package;
// it renders every user-facing line itself and returns the error only so
// the caller can classify it into an exit code with ExitCode.
//
// Dispatch and flag parsing are delegated to github.com/spf13/cobra, but
// exit codes, usage text and error copy stay owned here: every command's
// behavioral prose is its own constant, rendered by one help template into
// cobra's generated Usage line and flag table, every pflag error is
// rewritten into this package's one-line usage error, and cobra never
// prints its own output or calls os.Exit — ExitCode, called from main,
// remains the only thing that decides how the process exits. The root
// command and "new" disable cobra's flag parsing and resolve their first
// argument themselves, so an unknown command or type gets the same
// one-line error as any other usage mistake.
//
// Flags and positionals may be given in any order on every command.
//
// JSON mode (R5) is on iff an exact "--json" token appears anywhere in
// argv before the first "--" (pflag's own flag-parsing terminator, never
// a flag itself): run's own scanJSONFlag detects and strips every such
// token ahead of cobra entirely, so no command ever sees "--json" in its
// own args, and a flag error is itself rendered as JSON. A "--json"
// token at or after "--" is an ordinary positional. "--json=<v>", any
// value including an empty one, is always a text usage error — "'--json'
// takes no value" — checked before dispatch, so it wins over every other
// usage error on the line. reporter (json.go) is the one per-Run output
// seam every command renders a usage error through: usageError writes one
// compact JSON document to stdout in JSON mode, or msg to stderr
// otherwise, both returning the same errors.Is(err, ErrUsage) error.
//
// "brief completion <bash|zsh|fish|powershell>" is enabled but Hidden: it
// carries the listedInHelpAnnotation instead, which keeps it out of every
// "expected one of:" list while giving it a root-help row and a
// "brief help completion" topic.
//
// "brief init [--host <name>] [--no-hook] [--with-agents] [--edit-agents]
// [--dry-run | --print] [--force]" installs internal/setup's own config file and
// feature root, converging on a second run; its refusal — a
// *setup.RefusalError, checked ahead of a bare *config.InvalidConfigError
// so its own "run 'brief init --force'" fix is never lost to the generic
// one — carries writesFilesAnnotation the same way "new" and "finish" do.
// --json and --force are both registered but left out of the Use line
// itself (leaf help's own 80-column budget), the same as "check"'s and
// "new"'s own Use lines. A bare --host resolves by detection
// (setup.detectHost) rather than defaulting to "none": runInit passes ""
// straight through to setup.InitRequest.Host, and renders
// Result.NoHostDetected's own stderr line in place of the ordinary next
// action when detection ran and found nothing. --print computes the same
// plan as --dry-run and writes nothing either, but renders each pending
// artifact's own path and bytes instead of the ordinary rows — the two
// flags are mutually exclusive, checked in runInit before setup.Init ever
// runs. setup.ErrUnwritable (R10) is the one Init error path that returns
// a populated Result alongside the error, so runInit's own
// renderUnwritable can still print the --print output after the refusal
// line.
//
// "brief help [command] --json", "brief --help --json" and every leaf's
// own "<command> --help --json" render a help document instead of text:
// commands[] full index (root's own help) or filtered to the one command
// asked about, built by newRootCommand's single root.SetHelpFunc wrapper
// (help_json.go). Every help document's own "command" field is the
// literal "help", not the described command's path. "brief completion
// <shell> --json" is a usage error (R11), never the script.
//
// "brief new feature <name>" and "brief new step <feature>" share one
// refusal template: a *scaffold.RefusalError (or a
// *config.InvalidConfigError, for a bad ".brief.yaml") renders as one line
// ending "(no files changed)", naming the offending path and how to fix
// it, so a script can rely on that suffix to know a refusal changed
// nothing on disk. Every other error flattens to one line without that
// guarantee.
//
// run (cli.go), unlike the exported Run, takes a trailing ...runSeam: a
// test builds one with withDoctorOpts, withSetupOpts or withRootFS to
// substitute doctor's, setup's, scaffold's and assemble's own rwfs.FS
// seams — doctor.WithRootFS, doctor.WithHomeTree, setup.WithFSRoot,
// setup.WithResolveRoot, setup.WithWritableCheck, scaffold.WithFS and
// assemble.WithFS — for one shared rwfs.Mem, so a command-level test can
// exercise doctor's, init's, uninstall's, new's, finish's, start's,
// check's and status's own logic without touching real disk. withRootFS
// also drives resolveRoot (in place of config.Resolve), runDoctor's own
// config-location pre-check (locateInRepoFS, in place of
// config.LocateInRepo) and finish's own readSource (in place of
// os.ReadFile, for a non-"-" --handoff/--state argument, mapped through
// fsName) — every run* function except runCheckHook passes it straight
// through to resolveRoot and its own scaffold.WithFS/assemble.WithFS/
// readSource calls. Run always calls run with no seams, so production is
// unaffected: every seam's default reproduces exactly the OS adapter it
// replaces. A handful of checks stay OS-subject regardless of any seam —
// doctor's own root-dir and env-path rows, setup's R10 writability
// pre-check unless a test also overrides WithWritableCheck, and check
// --hook's own FeatureContaining resolution (real os.Lstat/
// filepath.EvalSymlinks, no seam) — since they probe real permission bits,
// compare real binaries, or resolve a caller-given path outside any
// configuration this package controls; a command-level test covering one
// of those stays on real disk (a *_disk_test.go file, per this package's
// own naming convention for a test that cannot move off it).
package cli
