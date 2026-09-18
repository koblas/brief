// Package cli is brief's command dispatcher. Run parses argv, resolves
// configuration from a working directory, and drives a feature package;
// it renders every user-facing line itself and returns the error only so
// the caller can classify it into an exit code with ExitCode.
//
// The dispatcher is hand-rolled over flag.FlagSet rather than a third-party
// framework: a framework would take over exit codes, usage text and error
// copy, which is exactly the contract this package owns.
//
// "brief new feature <name>" and "brief new step <feature>" share one
// refusal template: a *scaffold.RefusalError (or a
// *config.InvalidConfigError, for a bad ".brief.yaml") renders as one line
// ending "(no files changed)", naming the offending path and how to fix
// it, so a script can rely on that suffix to know a refusal changed
// nothing on disk. Every other error flattens to one line without that
// guarantee.
package cli
