# ADR-001: Adopt urfave/cli v3 over kong for CLI dispatch

## Status

Accepted

## Context

`internal/cli` used to be a hand-written dispatcher built on per-command `flag.FlagSet`s. It
worked, but it needed workarounds: `splitLeadingPositionals` existed only because
`flag.FlagSet` stops at the first positional, so `start` and `finish` had to split their
arguments by hand. We wanted a framework to handle routing and flag parsing. There was one
hard constraint. `internal/cli` owns a contract that about 2,500 lines of tests pin exactly:
one-line usage errors with fixed wording, help on stdout with exit 0, exit codes 0/1/2
decided only in `main`, `""` accepted as an argument so it can be refused as an empty name,
and `-` accepted as a flag value. A framework was only acceptable if the existing test suite
passed with no test file edited.

We tried both candidates against those cases, and then ported the real code to one of them.

**kong v1.16.1.** It routes the same way, but it writes its own error messages:
`unknown flag -x, did you mean "-h"?`, `unexpected argument b`, `expected one of "new", …`.
Every tested usage line reads `flag provided but not defined: -x`. Keeping that wording would
mean matching on kong's error strings and rewriting them. On `--help`, kong calls its `Exit`
hook and then carries on parsing (`brief --help` returned both exit 0 and the error
`expected one of …`). To keep a nil error, the `Exit` hook would have to panic or set a
sentinel. kong's struct tags would pay off for a large option surface. brief has three
flags.

**urfave/cli v3.13.0.** Its flag parser reports errors in the same words as the standard
`flag` package, so the tested flag-error lines pass unchanged. It keeps `""`, allows flags
between positionals, and takes argv, writers and a no-op `ExitErrHandler` for each run, so
it never calls `os.Exit`.

## Decision

`internal/cli` uses urfave/cli v3. The command tree is rebuilt on every `Run` call. The root
and `new` set `SkipFlagParsing`, so their own Actions produce the "no/unknown command" and
"no/unknown type" errors. Leaf commands let urfave parse flags. They print their existing
usage constant through `CustomHelpTemplate`, and `OnUsageError` wraps urfave's message in
brief's usage-error line. `ExitCode` in `main` is still the only place that sets the exit
code.

The deciding evidence: all 483 existing tests passed with no test file changed. A
differential run of the old and new binaries over 33 invocations gave byte-identical
stdout, stderr and exit codes for 30 of them. The other 3 are listed below.

## Consequences

- **Positive:** `splitLeadingPositionals` and six copies of FlagSet/ErrHelp boilerplate are
  gone. Flags and positionals now work in any order on every command. Adding a command means
  adding one `leafCommand(...)` entry.
- **Positive:** The error wording, help text and exit codes the tests pin are unchanged.
- **Trade-off:** `finish` now accepts flags before or between its positionals. Before, that
  was a usage error ("no feature given"). This is looser, not stricter, and
  `Test_finishes_the_step_when_the_flags_precede_the_feature_and_step` now pins it.
- **Negative:** If `--help` comes before an undefined flag on the same command
  (`brief start --help --bogus`), urfave prints its own generated help instead of brief's,
  still exiting 0. That path ignores `CustomHelpTemplate`. The only way around it is to set
  urfave's package-level `ShowSubcommandHelp`, and we chose not to change global state. The
  reverse order (`--bogus --help`) still gives brief's usage error.
- **Negative:** A missing flag value is now reported as `flag needs an argument: --handoff`
  (with two dashes) instead of `-handoff`. No test pins this line.
- **Negative:** This adds a third-party dependency. Help constants now pass through
  `text/template`, so they must never contain `{{`.
