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
// "brief completion <bash|zsh|fish|powershell>" is enabled but Hidden: it
// carries the listedInHelpAnnotation instead, which keeps it out of every
// "expected one of:" list while giving it a root-help row and a
// "brief help completion" topic.
//
// "brief new feature <name>" and "brief new step <feature>" share one
// refusal template: a *scaffold.RefusalError (or a
// *config.InvalidConfigError, for a bad ".brief.yaml") renders as one line
// ending "(no files changed)", naming the offending path and how to fix
// it, so a script can rely on that suffix to know a refusal changed
// nothing on disk. Every other error flattens to one line without that
// guarantee.
package cli
