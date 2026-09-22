---
id: SCENARIO-09
status: done
---

# SCENARIO-09: "brief help" with an unknown topic

## Scenario

```gherkin
Scenario: SCENARIO-09 "brief help" with an unknown topic
  When I run "brief help bogus"
  Then stderr is exactly: brief help: unknown command "bogus"; expected one of: new, start, finish, status, check
  And the exit code is 2 and the process is never exited from inside cli
```

## User-visible contract

A help topic is accepted only when `Find` consumes every topic argument **and** lands on a
command that is available (`IsAvailableCommand`) or on root itself (bare `brief help`).
Anything else is a usage error: stdout empty, exit 2 (`ErrUsage` → `ExitCode`), and stderr
is exactly one line:

`brief help: unknown command "<topic>"; expected one of: new, start, finish, status, check`

where `<topic>` is **all the topic arguments as typed, joined by single spaces**.

- `brief help bogus` — Find: root, residual `[bogus]` → `unknown command "bogus"` (the pinned line)
- `brief help new bogus` — Find: new, residual `[bogus]` → `unknown command "new bogus"`
- `brief help start extra` — Find: start, residual `[extra]` → `unknown command "start extra"`
- `brief help start --json` — Find: start, residual `[--json]` → `unknown command "start --json"`
- `brief help --json start` — Find: root, residual `[--json start]` → `unknown command "--json start"`
- `brief help help` — Find: hidden stub, residual empty → `unknown command "help"`
- `brief help`, `help start`, `help new feature`, `help new` — residual empty, target root or available → unchanged: renders help to stdout, exit 0

Decisions (made here, not left to the developer):

- **Quote the whole topic, not just the residual.** For `help new bogus` the residual alone
  (`"bogus"`) would read as an unknown *top-level* command, which is false. The full topic
  names the path the user asked about, and one format string with one list covers every row,
  including `help help` where the residual is empty. The rejected alternative — a per-level
  message (`brief help new: unknown type "bogus"; expected one of: feature, step`) — needs a
  second list literal (`feature, step`) and a second format string; that is the copy S11
  exists to eliminate. The list is always the root list: it names the valid first words of a
  topic.
- **Extra positional or any flag after a resolved leaf is a usage error, not ignored.**
  `help` takes a command path and nothing else; silently ignoring `--json` or `extra` would
  let `help start --json` render start's help as if it meant something. Closes S08's
  deferral with no special case: the residual rule answers it.
- **A leading flag does not resolve the topic.** `Find(["--json","start"])` stops at root
  (root has no `--json`, so `stripFlags` treats `start` as its value) → residual is the whole
  topic → usage error. Deliberate; no pre-scan to strip flags.
- **Hidden commands are not help topics.** `help help` (and `help __complete`) resolve to a
  hidden command; the rule rejects `target != root && !target.IsAvailableCommand()`. This
  makes "accepted help topic" and "command in the `expected one of:` list / root listing" the
  same set, which is exactly S11's `IsAvailableCommand` filter.

Verified in cobra source (`command.go` `Find`): the second return is the **unstripped**
residual (flags included), and `Find` never errors on this tree (every command has `Args`
set). So the stub keys off `len(residual) > 0`, never `Find`'s error.

## Implementation Plan

- [x] Step 1: `internal/cli/cli.go` — extract the `"new, start, finish, status, check"` literal into one unexported const (e.g. `expectedCommands`) used by both `runRoot` messages; existing root no-command / unknown-command tests in `run_test.go` stay green unedited (refactor, green on arrival — say so)
- [x] Step 2: `internal/cli/run_test.go` `Test_prints_root_usage_and_a_nil_error_for_brief_help_with_an_unknown_topic` — delete it (lines ~264-277); it pins the behavior this scenario replaces and is superseded by Step 3's table, not kept alongside it (update)
- [x] Step 3: `internal/cli/help_test.go` `Test_help_with_an_unresolved_topic_is_a_one_line_usage_error` — one table through `cli.Run`, one row per contract row above that errors (`help bogus`, `help new bogus`, `help start extra`, `help start --json`, `help --json start`, `help help`); each row asserts `ErrorIs(err, cli.ErrUsage)`, `cli.ExitCode(err) == 2`, stdout empty, and stderr equal to a hand-written literal line (raw, trailing `\n` — never built from the const or a format string) (red)
- [x] Step 4: `internal/cli/cli.go` help stub `RunE` — keep `Find`'s residual; when residual is non-empty or target is a non-root unavailable command, return `usageError(stderr, …)` with the joined topic and the Step 1 const; otherwise the existing `InitDefaultHelpFlag` + `target.Help()` path, untouched (green)
- [x] Step 5: `internal/cli/cli.go` — rewrite the stub's block comment (currently "A topic Find cannot resolve … falls back to root help") to state the new contract: accepted topics, the usage-error rule, no `os.Exit` (update)
- [x] Step 6: control arms, no new test needed — confirm unchanged and green: `help_test.go:85` bare `help` row, `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag` (`help start`, `help new feature`), `Test_help_start_prints_the_literal_start_help`, `run_test.go` `Test_prints_root_usage_and_a_nil_error_for_brief_help`. `help start` vs `help help` is the one-variable control for the hidden filter; `help start` vs `help start extra` is the control for the residual rule (verify)
- [x] Step 7: mutation-verify each guard individually (file copied to `$TMPDIR` or `git stash push -m "mutation: s09 <what>" -- internal/cli/cli.go` + `git stash apply <sha>` by captured SHA + drop by tag — never bare `git stash`/`pop`; `diff` to prove byte-identical restore): (a) remove the residual check → the five residual rows red, `help help` row stays green; (b) remove the hidden-target check → only the `help help` row red; (c) quote the residual instead of the full topic → `help new bogus`, `help start extra`, `help start --json` rows red, `help bogus` stays green. Report each mutation and the rows it reddened (verify)
- [x] Step 8: `go build ./...`, `go test ./...` (unpiped, report count + delta: −1 deleted test, +1 table test with 6 subtests), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-09 done in specification.md, update STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Help stub rule: accept iff `Find` residual is empty AND (target is root OR `target.IsAvailableCommand()`); else `brief help: unknown command "<all topic args joined by space>"; expected one of: <list>`, exit 2, stdout empty — replaces STATE's "Unresolved topic → root help". S10's `help new` passes (new is available, residual empty) and must stay passing.
- Quoted token is the full topic as typed, never the residual alone — the `help new bogus` row pins `"new bogus"`; a per-level list (`feature, step`) here would be a second hard-coded list S11 would have to chase.
- Extra positional or any flag after the topic (`help start --json`, `help --json start`) is a usage error, never ignored.
- The command list lives in one const in `cli.go`, used by `runRoot` (×2) and the help stub. S11 replaces that single source with a tree derivation — all three messages move together.

**Left unbuilt**:
- Tree-derived `expected one of:` list — S11. S09 pins `new, start, finish, status, check` (the approved scenario's string), which is NOT registration order (`new, start, status, check, finish`). When S11 derives the list it must either reorder `root.AddCommand` (which also moves the S07 root-help golden) or rewrite S09's six stderr literals and runRoot's goldens in the same change.
- Per-level help errors for `new` (`brief help new: unknown type …`) — deliberately not built.

**Traps**:
- `Find`'s residual is unstripped (flags included) and `Find` never returns an error on this tree — keying off the error, or off `stripFlags(residual)`, silently accepts `help start --json`.
- S12 plans `completion` with `HiddenDefaultCmd` → `Hidden: true` → `help completion` becomes `unknown command "completion"` under this rule. If S12 wants `help completion` to render, it must carve that out explicitly and say so; `completion` must still stay out of the list.
- Test literals are the user contract: never build the expected stderr from the const or `fmt.Sprintf` — a pin derived from the code pins nothing.
