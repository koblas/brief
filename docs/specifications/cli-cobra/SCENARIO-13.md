# SCENARIO-13: Completion for an unsupported shell

## Scenario

```gherkin
Scenario: SCENARIO-13 Completion for an unsupported shell
  When I run "brief completion tcsh"
  Then stderr is exactly: brief completion: unknown shell "tcsh"; expected one of: bash, zsh, fish, powershell
  And the exit code is 2
```

## User-visible contract

- Command: `brief completion <name>` with exactly one positional that is not one of the four
  table names (exact, case-sensitive match).
- stdout: empty. stderr: exactly one line,
  `brief completion: unknown shell "<name>"; expected one of: bash, zsh, fish, powershell`
  (`<name>` Go-`%q`-quoted). Exit code 2 (`cli.ErrUsage`).
- Observed before planning (`go run ./cmd/brief completion …`, exit 2 for each):
  `tcsh` → `unknown shell "tcsh"`; `ZSH` → `unknown shell "ZSH"`; `""` → `unknown shell ""`.
  All three take the same miss branch in `runCompletion` (`internal/cli/completion.go:69`).
  `ZSH` and `""` are pinned as rows anyway because each pins a decision a plausible refactor
  would flip: case-folding (`strings.EqualFold`) would make `ZSH` exit 0 with a script, and
  folding an empty name into the `len(rest) == 0` guard would turn `""` into `no shell given`.
  Neither is a separate branch today; they are not separate test functions.
- Arg-count errors (no shell / too many) are SCENARIO-12's and stay in their own test.

## Implementation Plan

Green on arrival is expected: SCENARIO-12 already implemented the miss branch. The work is
the pinning test plus mutation proof that it is not vacuous.

- [x] Step 1: `internal/cli/completion_test.go` `Test_completion_of_an_unknown_shell_is_a_one_line_usage_error` — table test through `cli.Run` (same shape as `Test_completion_without_exactly_one_shell_is_a_one_line_usage_error`: `require.ErrorIs(err, cli.ErrUsage)`, `ExitCode == 2`, empty stdout, `oneLine(stderr)` equals a full hand-typed literal per row); rows `tcsh`, `ZSH` (case-sensitive), `""` (empty name) (green on arrival — report it as such, do not manufacture a red)
- [x] Step 2: mutation — reorder `completionShells` (swap the `bash` and `zsh` entries) → every Step 1 row red on the list order only; proves the list derives from the ordered table, not a literal. S12's marker test must stay green under it (still-valid mutation) (verify)
- [x] Step 3: mutation — miss-branch message: `%q` → `%s` in `runCompletion`'s unknown-shell `fmt.Sprintf` → every row red (verify)
- [x] Step 4: mutation — case-fold the match (`s.name != shell` → `!strings.EqualFold(s.name, shell)`) → only the `ZSH` row red (verify)
- [x] Step 5: mutation — treat an empty name as missing (`len(rest) == 0` → `len(rest) == 0 || rest[0] == ""`) → only the `""` row red (verify)
- [x] Step 6: doc comment on the new test names the rule under test and the four mutations with the rows each reddened (update)
- [x] Step 7: `go build ./...`, `go test ./...` (unpiped, report count + delta: +3 subtests), `go test -race ./internal/cli/...`, `golangci-lint run ./...` all green → mark SCENARIO-13 done in `specification.md`; rewrite `STATE.md` (remove the S13 "Left unbuilt" entry and the "SCENARIO-13's to pin" clause in the `completion` decision)

Mutation hygiene (steps 2–5): one mutation at a time. Save `internal/cli/completion.go` with
`cp` to `$TMPDIR` before each mutation, restore with `cp`, then `diff` against `git show
HEAD:internal/cli/completion.go` to prove byte-identical. Never bare `git stash`/`git stash
pop` — the stash stack is shared across worktrees and sessions.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Shell matching is exact and case-sensitive; `ZSH` is an unknown shell — the `ZSH` row pins it,
  and completion scripts are only generated for the four lowercase table names.
- An empty shell argument (`brief completion ""`) is an unknown shell, not `no shell given` —
  the `no shell given` wording is reserved for zero positionals (S12's arg-count row).
- The unknown-shell "expected one of:" list derives from the ordered `completionShells` table
  (bash, zsh, fish, powershell); adding a shell means one table row, and every literal in
  `completion_test.go` plus `completionInvocation` changes with it.

**Left unbuilt** — named so nobody assumes it exists:
- Any "did you mean" suggestion for a near-miss shell name — unowned.
- Trimming/normalising the shell argument (`"zsh "` is unknown, observed) — deliberately not built.

**Traps** — things that look right and are not:
- `go run ./cmd/brief …` reports `exit status 1` for its own process while printing
  `exit status 2` for the child; read the child's line, or test through `cli.Run`/`cli.ExitCode`.
- Expected stderr lines must be full hand-typed literals per row — never built from
  `completionShellList()` or `fmt.Sprintf` in the test, or the order mutation (Step 2) cannot
  go red.
- `completion` is `ArbitraryArgs`; `runCompletion`'s guards are the only arg checks, so an
  empty-string positional reaches the table loop rather than being dropped by cobra.
