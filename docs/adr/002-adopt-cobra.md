# ADR-002: Adopt spf13/cobra over urfave/cli v3 for CLI dispatch

## Status

Accepted

## Context

ADR-001 adopted urfave/cli v3 for the same reason this decision revisits it: `internal/cli`
owns a contract several hundred tests pin exactly, and a dispatch framework is only
acceptable if it can carry that contract. urfave did, but it capped what brief could grow
into. Its help system could not be restructured without abandoning `CustomHelpTemplate` the
moment a flag error and `--help` appeared together (ADR-001's own workaround,
`isHelpRequest`, existed because of this). It has no shell completion, and no generated help
structure — every command's help was already a hand-written constant with no `Usage:` line or
flag table, which is what `brief start --help` still needs.

cobra v1.10.2 (via pflag) gives brief three things urfave does not: a completion command per
shell, help generation from `Long`/flag definitions instead of only a literal constant, and
command groups for when the surface grows past six commands. The trade-off, accepted here, is
GNU-style flag syntax: pflag does not accept a single-dash long flag (`-json`, `-help`); only
`--json`, `--help` and the short `-h`/`-x` forms remain. No existing test used the
single-dash form, so this became a business rule (R4 in the specification) rather than a
compatibility shim.

## Decision

`internal/cli` uses cobra v1.10.2. The command tree is rebuilt on every `Run` call, exactly as
urfave's was. The root and `new` set `DisableFlagParsing` and resolve their own first
argument, unchanged from ADR-001's shape. Leaf commands let cobra (via pflag) parse flags
normally; a single root-level `SetFlagErrorFunc` rewrites pflag's error into brief's one-line
usage error, naming the invocation carried in each leaf's `Annotations`. A single root-level
`SetHelpFunc` prints `cmd.Long` verbatim — the same `*Usage` constants ADR-001 already had,
unmoved. cobra's own `help` command is replaced via `SetHelpCommand`: its default calls
`cobra.CheckErr` on an unknown topic, which calls `os.Exit(1)` directly, violating R1
(exit codes decided only by `cli.ExitCode` in `cmd/brief`). `ExitCode` remains the only thing
that decides how the process exits.

Feature-name completion (`ValidArgsFunction` naming known features so `brief start <TAB>`
completes) is deliberately out of scope here. It needs a `Store` read at completion time,
which is feature-package work, not a dispatch change; it is a named follow-up, not a debt
this decision leaves silently open.

## Consequences

- **Positive:** `brief` gets `completion <bash|zsh|fish|powershell>` for free, generated help
  structure (a `Usage:` line and a flag table cobra builds from the same flag definitions
  `RunE` already reads), and a place to add command groups without another rewrite.
- **Positive:** `isHelpRequest` is gone. cobra's own `-h`/`--help` flag, present on every
  leaf, drives help directly — no more parsing an undefined-flag message to detect it.
- **Trade-off:** pflag's GNU syntax rejects the single-dash long flag. `-json`, `-help` and
  their kin no longer parse; only `--json`/`--help` and `-h`/`-x` do. Approved as R4; no
  existing test exercised the old form.
- **Trade-off:** flag-error wording changed again, this time to pflag's: `unknown flag:
  --bogus`, `unknown shorthand flag: 'x' in -x`, `flag needs an argument: --handoff`. Every
  test pinning the old `flag provided but not defined: …` wording was rewritten in
  SCENARIO-01, in the same pass as the port, so the suite never carried both wordings at
  once.
- **Negative:** cobra always registers hidden `__complete`/`__completeNoDescriptions`
  commands during `ExecuteC`, regardless of `CompletionOptions.DisableDefaultCmd`. Anything
  deriving a command list from the tree (the `expected one of:` list, eventually) must filter
  `Hidden`, not assume the tree only contains what was explicitly registered.
- **Negative:** This is a second dependency swap in two ADRs. The contract that forced both —
  one-line usage errors, help on stdout at exit 0, exit codes decided only in `main` — held
  through this one too, and is what any future framework swap must hold as well.
