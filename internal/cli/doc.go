// Package cli is brief's command dispatcher. Run parses argv, resolves
// configuration from a working directory, and drives a feature package;
// it renders every user-facing line itself and returns the error only so
// the caller can classify it into an exit code with ExitCode.
//
// The dispatcher is hand-rolled over flag.FlagSet rather than a third-party
// framework: a framework would take over exit codes, usage text and error
// copy, which is exactly the contract this package owns.
package cli
