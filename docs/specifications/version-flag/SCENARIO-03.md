---
id: SCENARIO-03
status: done
---

# SCENARIO-03: --version takes no arguments

## Scenario

```gherkin
Scenario: SCENARIO-03 --version takes no arguments
  When I run "brief --version extra" or "brief --version --json"
  Then stderr is exactly: brief: '--version' takes no arguments; run 'brief --version'
  And stdout is empty and the exit code is 2
```

Rules: R4 (trailing-argument copy), R8 (first argument decides — `brief --help --version`
keeps `--help`'s copy).

User-visible contract:
- `brief --version <anything>` (e.g. `extra`, `--json`, `--help`, `--version`) → stderr is
  exactly one line `brief: '--version' takes no arguments; run 'brief --version'`, stdout
  empty, `cli.ErrUsage`, exit 2.
- Control: `brief --help --version` → stderr exactly
  `brief: '--help' takes no arguments; run 'brief help <command>'`, stdout empty, exit 2.
- Unchanged: sole `brief --version` → `brief (devel)\n` on stdout, exit 0 (existing test).

What exists (via grep, no Glob needed): `runRoot` (`internal/cli/cli.go` ~468) already has an
`argVersionFlag` arm with a `len(args) == 1` guard; its trailing-arg branch currently emits
`brief: unknown flag: --version; run 'brief <command> --help'`. The `argHelpFlag` arm builds
the sibling `takes no arguments` copy inline with `fmt.Sprintf`. The help trailing-arg test
`Test_reports_a_help_flag_with_trailing_arguments_as_taking_no_arguments`
(`internal/cli/run_test.go` ~297) is a `cli.Run` table test with exactly the assertion shape
needed.

## Implementation Plan

- [x] Step 1: `internal/cli/run_test.go` `Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments` — `cli.Run` table test, rows `--version extra`, `--version --json`, `--version --help`, `--version --version`; each asserts `ErrorIs cli.ErrUsage`, `ExitCode == 2`, empty stdout, exact one-line stderr via `oneLine` (red — today's unknown-flag wording)
- [x] Step 2: `internal/cli/run_test.go` `Test_reports_a_help_flag_with_trailing_arguments_as_taking_no_arguments` — add row `--help --version` pinning `--help`'s copy (R8 control arm; green on arrival — say so)
- [x] Step 3: `internal/cli/cli.go` `runRoot` `argVersionFlag` arm — trailing-arg branch returns the `'--version' takes no arguments; run 'brief --version'` usage error (green)
- [x] Step 4: `internal/cli/cli.go` `takesNoArgumentsMessage(flag, runHint string) string` — unexported helper rendering `brief: '<flag>' takes no arguments; run '<runHint>'`, used by both root `argHelpFlag` and `argVersionFlag` arms; root only, `new`/help-stub copy untouched (refactor, suite stays green)
- [x] Step 5: `internal/cli/cli.go` `runRoot` doc comment + `internal/cli/classify.go` `argVersionFlag` doc — state that root reports a trailing argument after `--version` as "takes no arguments" and does not use `msg` for it; `msg` remains for `runNew`/help stub (update)
- [x] Step 6: mutation-verify the length guard — stash `cli.go`, change `len(args) == 1` in the `argVersionFlag` arm to `len(args) >= 1`; every Step 1 row must red (nil error, version on stdout); restore and `diff` byte-identical. Separately mutate the hint argument (`'brief --version'` → `'brief help <command>'`) and confirm Step 1 reds while the Step 2 control row stays green; record both mutations and the tests they reddened in the test's doc comment
- [x] Step 7: `go build ./...`, `go test ./...` (unpiped; report count + delta: +5 subtests), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-03 done in specification.md and rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Root's `argVersionFlag` arm: `len(args) == 1` prints the version; any `len(args) > 1` returns
  `brief: '--version' takes no arguments; run 'brief --version'` — the run hint is
  `brief --version`, NOT `--help`'s `brief help <command>` (R4 copy is a user contract).
- The trailing-arg copy names `--version` literally (only exact `--version` classifies as
  `argVersionFlag`), not the unknown-flag `msg`. `msg` for `argVersionFlag` is now consumed
  only by `runNew` and the help stub (R6 byte-identity — SCENARIO-06 depends on it).
- First argument decides (R8): `--help --version` stays `--help`'s error, `--version --help`
  is `--version`'s error. No scanning past `args[0]`.
- `takesNoArgumentsMessage(flag, runHint)` is root-scoped (`brief: ` prefix baked in).
  SCENARIO-04 should add a sibling `takesNoValueMessage(flag, runHint)` for
  `'--version' takes no value; run 'brief --version'` rather than re-inlining `Sprintf`.

**Left unbuilt** — named so nobody assumes it exists:
- `--version=x` / `--version=` "takes no value" copy — SCENARIO-04 (still `argUnknownFlag`).
- `takesNoValueMessage` — SCENARIO-04 (only if natural; `--help`'s value arm is its second user).
- `-v` pin — SCENARIO-05; `new`/`help`/leaf `--version` byte-identity table — SCENARIO-06;
  root-help trailer — SCENARIO-07.

**Traps** — things that look right and are not:
- The existing trailing-arg branch already returns `ErrUsage`/exit 2, so a test asserting only
  the error and exit code is green today — Step 1 must assert exact stderr to be red.
- The `--help --version` control row is a behavior pin, green on arrival; no plausible guard
  mutation in the `argVersionFlag` arm can redden it. Do not claim it as guard evidence.
- Don't refactor `new.go`/the help stub onto the root helper: their prefixes (`brief new: `,
  `brief help: `) and hints differ, and R6 requires their bytes to be unchanged.
- `exhaustive` lint: no new `argKind` in this scenario, so no new arms at the three switch sites.
