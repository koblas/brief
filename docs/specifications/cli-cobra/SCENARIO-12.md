---
id: SCENARIO-12
status: done
depends-on: []
---

# SCENARIO-12: Shell completion script

## Scenario

Scenario: SCENARIO-12 Shell completion script
  When I run "brief completion zsh"
  Then stdout is a zsh completion script and the exit code is 0
  And "completion" does not appear in any "expected one of:" list
  And root help has one line for "brief completion <bash|zsh|fish|powershell>"

Rules: R9 (enabled, hidden from command lists, one root-help line, errors follow R2),
Product Verdict item 1. Inherited context: `STATE.md` only (no prior SCENARIO file opened).

## User-visible contract

- `brief completion <bash|zsh|fish|powershell>` → the generated script on stdout, stderr
  empty, exit 0.
- `brief completion` (no shell) → stderr exactly
  `brief completion: no shell given; run 'brief completion <bash|zsh|fish|powershell>'`,
  stdout empty, exit 2.
- `brief completion zsh bash` → stderr exactly
  `brief completion: too many arguments; run 'brief completion <bash|zsh|fish|powershell>'`,
  stdout empty, exit 2. (Same R2 shapes as `start`'s arg-count errors.)
- `brief completion --bogus` → R14 frame via the root `FlagErrorFunc`:
  `brief completion: unknown flag: --bogus; run 'brief completion <bash|zsh|fish|powershell>'`,
  exit 2.
- `brief completion --help` and `brief help completion` → the same leaf help body (Usage
  line, `completionLong` prose, Flags table), stdout, stderr empty, exit 0.
- `brief`, `brief bogus`, `brief help bogus` → the "expected one of:" list stays
  `new, start, finish, status, check` — no `completion`.
- Root help gains, after `check`'s row, the wrapped row (UseLine is 43 chars > the 33-column
  `cmdRow` threshold, so it takes finish's wrapped shape):
  `  brief completion <bash|zsh|fish|powershell>` / 35 spaces + `print a shell completion script`.
- Unknown shell (`completion tcsh`) is SCENARIO-13's to pin — see Handoff.

## Design decisions (this plan)

- **brief's own `completion` command, not cobra's default.** `CompletionOptions.DisableDefaultCmd`
  stays `true` — this contradicts STATE.md's "flips back on" note, deliberately. Cobra's default
  registers the four shells as *subcommands* of a non-runnable `completion`, so `completion tcsh`
  never reaches a brief-owned error path (non-root unknown args → `flag.ErrHelp`/help, exit 0),
  and S13's exact line would need surgery on a subtree cobra builds inside `Execute`. brief's
  own leaf calls `GenBashCompletionV2` / `GenZshCompletion` / `GenFishCompletion` /
  `GenPowerShellCompletionWithDesc` on `cmd.Root()` writing to the passed stdout. Outcome the
  verdict binds (enabled, hidden, one root-help line, R2/R14 errors) is preserved.
- **Built with `leafCommand`**, then `Hidden = true` plus a new "listed in help" annotation.
  Inherits `ArbitraryArgs`, `DisableFlagsInUseLine`, and the `invocation` annotation, so the
  R14 flag-error frame is free. Registered **last** in `root.AddCommand`.
- **Hidden but listed**: `IsAvailableCommand()` stays the sole filter for
  `expectedCommandList` (no change there). The help template's outer row loop and the help
  stub's topic-acceptance predicate both widen to "available OR carries the listed-in-help
  annotation". One concept governs both, so a command shown in root help is always a valid
  `brief help` topic — R8 (`help <path…>` ≡ `<path…> --help`) holds for `completion`, while
  the hidden `help` stub (no annotation) stays rejected as S09 pinned.
- **No feature package.** The command renders cobra's own tree into a script — delivery-only,
  no domain logic — so it lives in `internal/cli/completion.go`. No `cmd/brief` change:
  `ErrUsage` → exit 2 already maps.
- The four shell names live in one ordered table in `completion.go` (name → generator); the
  S13 "expected one of:" shell list is derived from it, tests pin the literal.

## Implementation Plan

- [x] Step 1: `internal/cli/completion_test.go` `Test_prints_a_completion_script_for_each_supported_shell` — table over bash/zsh/fish/powershell through `cli.Run`: no error, stderr empty, stdout non-empty and contains a shell-identifying marker taken from the real generated output at green time (no golden of script bytes) (red)
- [x] Step 2: `internal/cli/completion_test.go` `Test_completion_without_exactly_one_shell_is_a_one_line_usage_error` — rows for no argument and two arguments; exact stderr literal, stdout empty, `errors.Is(err, cli.ErrUsage)` (red)
- [x] Step 3: `internal/cli/completion.go` — `completionLong` prose, the ordered shell→generator table, `runCompletion` (arg-count guards, lookup, generate to stdout); non-matching single arg returns S13's exact unknown-shell usage error (new)
- [x] Step 4: `internal/cli/cli.go` `newRootCommand` — register the completion leaf last via `leafCommand` (`Use: "completion <bash|zsh|fish|powershell>"`, `Short: "print a shell completion script"`, invocation `brief completion <bash|zsh|fish|powershell>`), set `Hidden`, add the listed-in-help annotation constant; keep `DisableDefaultCmd = true` (green for Steps 1-2)
- [x] Step 5: `internal/cli/help_test.go` `rootHelp` — add the wrapped completion row after `check`'s row; `Test_prints_the_root_help_with_one_line_per_command` goes red (red)
- [x] Step 6: `internal/cli/cli.go` `helpTemplate` — widen only the outer `cmdList` row loop's predicate to admit a command carrying the annotation; first confirm `index` on an absent `Annotations` key reads falsy in cobra's template context (nil map included). Fallback if it does not: a root-only literal row, plus a test asserting that literal equals the completion command's UseLine/Short (green)
- [x] Step 7: prove `newHelp` and `startHelp` goldens byte-identical after the template edit (they must pass unedited) — `go test ./internal/cli/ -run 'Help'` (verify)
- [x] Step 8: `internal/cli/help_test.go` `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag` — add a `completion` row (`help completion` ≡ `completion --help`, exit 0, stderr empty); red against S09's hidden-target rule (red)
- [x] Step 9: `internal/cli/cli.go` help stub `RunE` — accept a target that is root, available, or carries the listed-in-help annotation; the existing `help help` rejection row must stay green (green)
- [x] Step 10: `internal/cli/help_test.go` `Test_every_command_help_has_a_usage_line_and_a_flag_table` — add a `completion` row, path `brief completion` (green on arrival expected; say so)
- [x] Step 11: `internal/cli/completion_test.go` `Test_completion_is_absent_from_every_expected_command_list` — table: bare `brief`, `brief bogus`, `brief help bogus`; each row's full stderr literal ending `expected one of: new, start, finish, status, check`. Control arm: mutation-verify by temporarily removing `Hidden` and watching these rows go red (green on arrival; mutation is the evidence)
- [x] Step 12: `internal/cli/flag_error_test.go` — add a `completion --bogus` row with the full R14 literal (green on arrival via the inherited annotation; mutation-verify by blanking the invocation argument)
- [x] Step 13: `internal/cli/completion_test.go` `Test_the_hidden_complete_command_answers_for_generated_scripts` — `cli.Run` with `__complete`, an empty word after the root (e.g. `__complete ""`): no error, stdout lists at least one known command name and ends with a `:<directive>` line. Do NOT assert stderr empty — cobra writes `Completion ended with directive: …` there. Regression pin, not new wiring: `__complete` is added by cobra's `initCompleteCmd` on every Execute regardless of `DisableDefaultCmd` (green on arrival; say so)
- [x] Step 14: `internal/cli/doc.go` / `newRootCommand` doc comment — mention `completion` in registration order and the listed-in-help annotation rule; no history in comments (update)
- [x] Step 15: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-12 done in specification.md; update STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `completion` is brief's own leaf (`internal/cli/completion.go`), `CompletionOptions.DisableDefaultCmd`
  stays `true` — cobra's default command makes shells subcommands, so S13's unknown-shell line
  would be unreachable (`flag.ErrHelp` path, exit 0) without it.
- `completion` is `Hidden`; `expectedCommandList` still filters by `IsAvailableCommand` only —
  R7's lists must never grow a name check.
- "Listed in help" is one annotation read in two places: `helpTemplate`'s outer row loop and
  the help stub's topic predicate. Any command shown in root help is a valid `brief help` topic
  (R8). The hidden `help` stub carries no annotation and stays rejected (S09 row).
- `completion` registers last in `root.AddCommand`; its root-help row is the wrapped (finish)
  shape because its UseLine exceeds 33 columns.
- Shell order `bash, zsh, fish, powershell` comes from one ordered table in `completion.go`;
  any shell-list message derives from it.
- Arg-count errors use R2's `start` shapes: `no shell given` / `too many arguments`, each
  `; run 'brief completion <bash|zsh|fish|powershell>'`.

**Left unbuilt** — named so nobody assumes it exists:
- A test pinning `brief completion tcsh` → `brief completion: unknown shell "tcsh"; expected one of: bash, zsh, fish, powershell`, exit 2 — SCENARIO-13. The branch is **implemented** in
  `runCompletion` here (the dispatch cannot exist without a miss branch); S13 adds the exact
  stderr/stdout-empty/exit-2 row and reports green-on-arrival, mutation-verifying the branch.
- Feature-name completion (`ValidArgsFunction`) — named follow-up in ADR-002, out of scope.
- No golden of generated script bytes anywhere — deliberately; cobra owns them.

**Traps** — things that look right and are not:
- `IsAvailableCommand()` is false for any `Hidden` command, and the template's row loop used
  the same predicate as `expectedCommandList` — hiding alone drops the root-help row.
- `__complete` writes `Completion ended with directive: …` to stderr on every run; an
  "stderr empty" assertion on it is wrong.
- Cobra's `GenZshCompletion` etc. read the tree from `cmd.Root()` — generating from the leaf
  itself produces a script for the wrong program name.
- Any `helpTemplate` edit can shift whitespace silently (STATE trap) — `newHelp`/`startHelp`
  must pass unedited; `new`'s children carry no annotation, so `newHelp` must not change.
- `completion` is `ArbitraryArgs` (via `leafCommand`) — cobra never counts its args; the
  guards in `runCompletion` are the only ones.
