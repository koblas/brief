// Package cli is brief's command dispatcher. Run parses argv, resolves
// configuration from a working directory, and drives a feature package;
// it renders every user-facing line itself and returns the error only so
// the caller can classify it into an exit code with ExitCode.
//
// Dispatch and flag parsing are delegated to github.com/urfave/cli/v3, but
// exit codes, usage text and error copy stay owned here: every command
// prints its own help constant, every flag error is rewritten into this
// package's one-line usage error, and urfave's exit handler is disabled so
// ExitCode, called from main, remains the only thing that decides how the
// process exits. The root command and "new" skip urfave's flag parsing and
// resolve their first argument themselves, so an unknown command or type
// gets the same one-line error as any other usage mistake.
//
// Flags and positionals may be given in any order on every command.
//
// "brief new feature <name>" and "brief new step <feature>" share one
// refusal template: a *scaffold.RefusalError (or a
// *config.InvalidConfigError, for a bad ".brief.yaml") renders as one line
// ending "(no files changed)", naming the offending path and how to fix
// it, so a script can rely on that suffix to know a refusal changed
// nothing on disk. Every other error flattens to one line without that
// guarantee.
package cli
