// Package cli is brief's command dispatcher. Run parses argv, resolves
// configuration from a working directory, and drives a feature package;
// it renders every user-facing line itself and returns the error only so
// the caller can classify it into an exit code with ExitCode.
//
// Dispatch and flag parsing are delegated to github.com/spf13/cobra, but
// exit codes, usage text and error copy stay owned here: cobra never
// prints its own output or calls os.Exit. Flags and positionals may be
// given in any order on every command.
//
// JSON mode is on iff an exact "--json" token appears anywhere in argv
// before pflag's "--" terminator; run's scanJSONFlag strips every such
// token ahead of cobra, so no command ever sees "--json" in its own args,
// and a flag error is itself rendered in the mode that was asked for. See
// reporter (json.go) for the shared output contract every command renders
// through.
//
// run (cli.go), unlike the exported Run, takes a trailing ...runSeam: a
// test builds one to substitute an in-memory rwfs.FS for doctor's,
// setup's, scaffold's and assemble's real-filesystem adapters, so a
// command-level test can exercise this package's logic without touching
// real disk. Run always calls run with no seams, so production is
// unaffected. A few checks stay OS-subject regardless of any seam — they
// probe real permission bits or resolve a caller-given path outside any
// configuration this package controls; a command-level test covering one
// of those lives in a *_disk_test.go file.
package cli
