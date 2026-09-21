# SCENARIO-05: -v stays an unknown shorthand

## Scenario

```gherkin
Scenario: SCENARIO-05 -v stays an unknown shorthand
  When I run "brief -v"
  Then stderr is exactly: brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'
  And the exit code is 2
```

Rule R5: `-v` and its clusters (`-v=x`, `-vh`, `-hv`) keep today's unknown-shorthand wording.

## Status: green on arrival (pinning-only scenario)

No production code changes. `classifyDashArg` already sends every `-v` shape to
`argUnknownFlag` via `unknownShortFlagMessage`; nothing in SCENARIO-01..04 touched the
single-dash branch. What is missing is a pin: no test names `-v` today (triage), so a future
`-v → --version` alias (explicitly rejected by the product verdict; `-v` is reserved for
`--verbose`) would ship silently. This scenario adds that pin and proves it is load-bearing by
mutation. The developer must report the new rows as **green on arrival** and not manufacture
a red.

## User-visible contract (observed on a built binary, go1.27.1, this branch at 46cebdc)

Every row: stdout empty, exit 2, stderr exactly one line:

- `brief -v` → `brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'`
- `brief -v=x` → `brief: unknown shorthand flag: 'v' in -v=x; run 'brief <command> --help'`
- `brief -vh` → `brief: unknown shorthand flag: 'v' in -vh; run 'brief <command> --help'`
- `brief -hv` → `brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'` (leading `h` consumed)
- `brief new -v` → `brief new: unknown shorthand flag: 'v' in -v; run 'brief new <type> --help'`
- `brief new -v=x` → `brief new: unknown shorthand flag: 'v' in -v=x; run 'brief new <type> --help'`
- `brief new -vh` → `brief new: unknown shorthand flag: 'v' in -vh; run 'brief new <type> --help'`
- `brief new -hv` → `brief new: unknown shorthand flag: 'v' in -v; run 'brief new <type> --help'`
- `brief help -v` → `brief help: unknown shorthand flag: 'v' in -v; run 'brief help <command>'`
- `brief help -v=x` → `brief help: unknown shorthand flag: 'v' in -v=x; run 'brief help <command>'`
- `brief help -vh` → `brief help: unknown shorthand flag: 'v' in -vh; run 'brief help <command>'`
- `brief help -hv` → `brief help: unknown shorthand flag: 'v' in -v; run 'brief help <command>'`

Cross-check done during planning: leaf pflag gives the same residual shape
(`brief start -v=x` → `... 'v' in -v=x ...`, `brief start -hv` → `... 'v' in -v ...`), so the
root/new/help strings above are pflag-consistent, not a divergence being frozen.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_classifies_dash_prefixed_tokens_consistently_across_disabled_parsing_sites` — add four clusters (`-v`, `-v=x`, `-vh`, `-hv`) x three sites (root, `new`, `help`) = 12 rows with the exact strings above, in the table's existing comment-headed group style (pin; green on arrival)
- [x] Step 2: mutation A, surgical and still-compiling, in `internal/cli/classify.go` `classifyDashArg` — just before the final `argUnknownFlag` return, route any arg with prefix `-v` to `argVersionFlag` **with msg `unknownLongFlagMessage(arg)`** (the msg form the real `--version` arm uses). Expected red: root `-v` (prints version, exit 0), root `-v=x`/`-vh` (root prints the version or the takes-no-arguments error — either way not the pinned string), and the `new`/`help` `-v`/`-v=x`/`-vh` rows (their msg becomes `bad flag syntax: -v...`). Do NOT use `unknownShortFlagMessage(cluster)` as the msg: `new`/help fold `argVersionFlag` into the `argUnknownFlag` case, so their rows would stay green and prove nothing. Record which rows went red; restore byte-identical (stash/copy per agent-briefs) (verify)
- [x] Step 3: mutation B in `internal/cli/classify.go` `isAllH` — accept `'v'` as well as `'h'`; expect the `-hv` rows (and `-v`/`-vh`) red because they classify as `argHelpFlag`; record rows; restore byte-identical. This is the only mutation that proves the `-hv` rows, since mutation A does not reach them (verify)
- [x] Step 4: `go build ./...`, `go test ./...` (unpiped, report count and delta: +12 subtests), `go test -race ./internal/cli/...`, `golangci-lint run ./...` — all green
- [x] Step 5: mark SCENARIO-05 done in `specification.md`; rewrite `STATE.md` (drop the `-v` item from Left unbuilt, add the trap below)

No step for `cmd/brief`, `classify.go` production code, `runRoot`, `runNew`, or the help stub:
none change. No row in `classify_internal_test.go`: its doc says it documents each `argKind`
without re-covering ground the black-box tables own, and `-v` is just another
`argUnknownFlag` (already represented by `-xy`).

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `-v` is not an alias for `--version` at any site — product verdict reserves `-v` for a
  future `--verbose`; the 12 rows added here are the guard.
- `-v` pins live in the cross-site table, not a new test func — that table already asserts
  `ErrUsage`, exit 2, empty stdout and the one-line stderr, and SCENARIO-06 extends the same
  table for `--version` outside root.

**Left unbuilt** — named so nobody assumes it exists:
- Leaf `-v` (e.g. `brief start -v`) is not pinned here: it goes through pflag, not
  `classifyDashArg`, and R5's risk is a `classifyDashArg` alias. Owner: none (out of R5 scope).
- `help --version`, leaf `--version` byte-identity rows: SCENARIO-06.
- Root-help trailer `Run 'brief --version' to print the installed version.`: SCENARIO-07.

**Traps** — things that look right and are not:
- `-hv` reports `'v' in -v`, not `'v' in -hv` — `unknownShortFlagMessage` skips leading `h`
  like pflag does. Writing `-hv` in the expected string is a wrong pin.
- `-v=x` keeps `=x` in the residual (`in -v=x`) — unlike `--version=x`, whose message trims
  the value. Matches leaf pflag; do not "fix" it.
- A `-v`-prefix mutation (A) never reaches `-hv` (its arg starts `-h`). Only mutation B
  (or equivalent) proves the `-hv` rows; don't claim them from A.
- `new`/`help` reuse `classifyDashArg`'s msg for `argVersionFlag`, so a `-v` alias only reddens
  their rows if the msg changes. A mutation returning `argVersionFlag` with the unchanged
  shorthand msg leaves them green — mutation A must use `unknownLongFlagMessage(arg)`.
