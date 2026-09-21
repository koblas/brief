# Specification: CLI dispatch on cobra

## Intent & Goal

**Primary Goal**: Move `internal/cli` dispatch from urfave/cli v3 to spf13/cobra (v1.10.2,
pflag) so a growing command surface gets generated help structure, shell completion and
command groups instead of hand-rolled dispatch.

**Out of Scope**: Feature-name completion via `ValidArgsFunction` (named follow-up); new
commands; man/markdown doc generation; persistent/global flags; any change to feature
packages (`internal/assemble`, `internal/scaffold`, `internal/platform/*`).

**Business Rules**: User-approved behavior changes — pflag GNU syntax (single-dash long flags
such as `-json`/`-help` stop working; `--json`, `--help`, `-h` remain); flag-error wording
becomes brief's R14 frame around pflag's message; `--help` next to an undefined flag, in
either order, is a usage error.

## Business Rules & Invariants

- R1: Exit codes 0 ok / 1 failure / 2 usage are decided only by `cli.ExitCode` in
  `cmd/brief`. Nothing below `main` calls `os.Exit` — including cobra's `CheckErr` paths.
- R2: Every usage error is exactly one line on stderr:
  `brief <command path>: <problem>; run '<invocation>'` (root/type errors keep their
  existing `expected one of:` copy). Nothing on stdout. Cobra never prints its own
  `Error:` line or a usage dump (`SilenceErrors`, `SilenceUsage`).
- R3: Help goes to stdout, stderr empty, exit 0, nil error.
- R4: `""` and `-` arguments, and flags in any order relative to positionals, keep working.
- R5: Refusal rendering (`internal/cli/refusal.go`) is unchanged.
- R6: A command's behavioral prose from today's `*Usage` constants appears verbatim in its
  help; cobra generates the `Usage:` line and flag table. No `Global Flags:`,
  `Additional help topics:`, or duplicate `Available Commands:` block.
- R7: The `expected one of:` command list is derived from registered, non-hidden commands —
  never a hard-coded literal. Cobra suggestions are disabled.
- R8: `brief help <path…>` is byte-identical to `brief <path…> --help`; brief owns the
  `help` command (cobra's is replaced).
- R9: `completion` is enabled, hidden from command lists, and has one line in root help;
  its errors follow R2.

---

## Triage Brief

**Affected surface.** `internal/cli/cli.go` (urfave tree: `newRootCommand`, `leafCommand`,
`isHelpRequest` — the last is dead under cobra and is deleted, not ported); the six
`*Usage` constants in `start.go`, `finish.go`, `new.go`, `status.go`, `check.go`;
`go.mod`/`go.sum`; `.golangci.yaml` wrapcheck ignore-sig for urfave `Command).Run(`
(replace with cobra's equivalent only if still needed).

**Already exists — do not re-plan.** `ErrUsage`, `usageError`, `ExitCode`
(`cli.go`); `renderRefusal` and `flattenOneLine` (`refusal.go`); the `run*` functions,
which already take parsed positionals and flag values; `Run(ctx, wd, args, stdin, stdout,
stderr)` signature — maps onto `SetArgs`/`SetIn`/`SetOut`/`SetErr`/`ExecuteContext`.

**Tests pinning old wording (update in place, per the approved change).**
`run_test.go:163`, `new_step_test.go:60`, `finish_test.go:454`, `start_test.go:462`,
`start_test.go:498` — `flag provided but not defined`. `start_test.go:479-498` — the
`--help --bogus` / `--bogus --help` pair; the first half flips to a usage error.
pflag has two shapes: `unknown flag: --bogus` (long) vs
`unknown shorthand flag: 'x' in -x` (short). Missing value: `flag needs an argument: --x`.

**Tests pinning help content (must pass unedited).** `check_test.go:212`,
`finish_test.go:503`, `status_test.go:340-341`, `start_test.go:473,487,731`. Root and
`new feature --help` assert only `NotEmpty`.

**Cobra hazards (verified in module source).**
- Auto `help` command: unknown topic → `CheckErr` → `os.Exit(1)` (`cobra.go:238`). Replace.
- Auto `completion` command (`completions.go:107-117`): on by default.
- Default `FlagErrorFunc` returns raw pflag error — needs `SetFlagErrorFunc`.
- `SilenceErrors`/`SilenceUsage` must be set; otherwise `Error:` + usage dump on stderr.
- `legacyArgs` checks unknown subcommands only at the true root; `new widget` on a
  non-runnable `new` returns `flag.ErrHelp` → help printed, nil error. `new` needs a RunE /
  Args handling to keep its unknown-type usage error.
- Root unknown command error appends multi-line suggestions — disable.
- `ParseFlags` runs before the help check: an undefined flag errors regardless of `--help`
  position (matches the approved change).
- pflag `interspersed` is default true; `""` and `-` pass through as positionals.

**Docs.** `docs/adr/001-adopt-urfave-cli-v3.md` → status `Superseded by ADR-002`, body
untouched; new ADR-002. Historical `docs/specifications/brief/SCENARIO-02/03/04/05.md` and
`SCENARIO-15-HANDOFF.md` quote old wording — leave as record.

## Product Verdict

**SHIP WITH CHANGES** (`product-vision`, pre-design). Accepted, folded into rules R6–R9 and
the scenarios:

1. `completion` enabled, `HiddenDefaultCmd`, one line in root help
   (`brief completion <bash|zsh|fish|powershell>  print a shell completion script`); its
   errors in R14 frame. Feature-name completion named as follow-up in ADR-002.
2. Replace cobra's `help`: `brief help` = root usage; `help <cmd…>` = `<cmd…> --help`;
   `help bogus` = unknown-command usage error, exit 2.
3. Disable suggestions; derive the `expected one of:` list from the tree.
4. Generate help structure with a custom template; move each constant's prose verbatim into
   `Long`, flag paragraphs into flag usage strings; root listing from each command's
   `Short`; the "Run 'brief new feature --help', …" paragraph becomes
   `Run 'brief <command> --help' for details.`
5. `new` gets its own help listing `new feature` and `new step`.
6. ADR-002; ADR-001 status line only.

Must-verify after port: bare `brief` still a usage error (exit 2); positionals under `new`
never produce cobra's `unknown command "x" for "brief new"`; `--handoff -` pinned;
`ExecuteContext` returns rather than exits.

---

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 Existing command contract holds on cobra
  Given the existing internal/cli suite, minus the 5 stdlib-wording assertions and the --help ordering pair
  When brief dispatches through cobra
  Then every one of those tests passes unedited, including "" names, "-" stdin values and flags in any order

Scenario: SCENARIO-02 Undefined long flag is a one-line usage error
  When I run "brief start --bogus demo"
  Then stderr is exactly "brief start: unknown flag: --bogus; run 'brief start <feature>'"
  And stdout is empty and the exit code is 2

Scenario: SCENARIO-03 Undefined short flag is a one-line usage error
  When I run "brief new feature -x payments"
  Then stderr is exactly "brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'"
  And the exit code is 2

Scenario: SCENARIO-04 Single-dash long flag is rejected
  When I run "brief start -json demo"
  Then brief reports a one-line usage error and exits 2, writing nothing to stdout

Scenario: SCENARIO-05 Flag missing its value
  When I run "brief finish demo SCENARIO-01 --handoff"
  Then stderr is exactly "brief finish: flag needs an argument: --handoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'"
  And the exit code is 2

Scenario: SCENARIO-06 --help next to an undefined flag is a usage error in either order
  When I run "brief start --help --bogus" or "brief start --bogus --help"
  Then each is a one-line usage error with exit 2 and nothing on stdout

Scenario: SCENARIO-07 Command help keeps its prose inside generated structure
  When I run "brief start --help"
  Then stdout has a Usage line, the existing start prose verbatim, and a flag table listing --json
  And there is no "Global Flags:" or "Additional help topics:" block, stderr is empty, and the exit code is 0

Scenario: SCENARIO-08 "brief help <command>" matches "<command> --help"
  When I run "brief help start" and "brief help new feature"
  Then each stdout is byte-identical to "brief start --help" and "brief new feature --help", with exit 0

Scenario: SCENARIO-09 "brief help" with an unknown topic
  When I run "brief help bogus"
  Then stderr is exactly: brief help: unknown command "bogus"; expected one of: new, start, finish, status, check
  And the exit code is 2 and the process is never exited from inside cli

Scenario: SCENARIO-10 "new" has its own help
  When I run "brief new --help" or "brief help new"
  Then stdout lists "new feature" and "new step", stderr is empty, and the exit code is 0

Scenario: SCENARIO-11 Unknown command stays one line, list built from the command tree
  Given a hidden command is registered
  When I run "brief bogus"
  Then stderr is one line naming only the visible commands, with no "Did you mean" block, and the exit code is 2

Scenario: SCENARIO-12 Shell completion script
  When I run "brief completion zsh"
  Then stdout is a zsh completion script and the exit code is 0
  And "completion" does not appear in any "expected one of:" list
  And root help has one line for "brief completion <bash|zsh|fish|powershell>"

Scenario: SCENARIO-13 Completion for an unsupported shell
  When I run "brief completion tcsh"
  Then stderr is exactly: brief completion: unknown shell "tcsh"; expected one of: bash, zsh, fish, powershell
  And the exit code is 2
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: Existing command contract holds on cobra
- [x] SCENARIO-02: Undefined long flag is a one-line usage error
- [x] SCENARIO-03: Undefined short flag is a one-line usage error
- [x] SCENARIO-04: Single-dash long flag is rejected
- [x] SCENARIO-05: Flag missing its value
- [x] SCENARIO-06: --help next to an undefined flag is a usage error in either order
- [x] SCENARIO-07: Command help keeps its prose inside generated structure
- [x] SCENARIO-08: "brief help <command>" matches "<command> --help"
- [x] SCENARIO-09: "brief help" with an unknown topic
- [x] SCENARIO-10: "new" has its own help
- [x] SCENARIO-11: Unknown command stays one line, list built from the command tree
- [x] SCENARIO-12: Shell completion script
- [x] SCENARIO-13: Completion for an unsupported shell
