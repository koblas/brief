---
id: SCENARIO-06
status: done
depends-on: []
---

# SCENARIO-06: --version outside the root is unchanged

## Scenario

```gherkin
Scenario: SCENARIO-06 --version outside the root is unchanged
  When I run "brief new --version", "brief help --version" or "brief start --version demo"
  Then each prints exactly what it prints today (an unknown-flag usage error, exit 2)
```

Rule: R6 — `new`, `help` and every leaf keep today's bytes for `--version`.

## Baseline (captured while planning — the "today" in R6)

Listed here as captured evidence of pre-feature bytes, so nobody has to rebuild `5709f33`
to know what R6 means; it is not an assertion design.

Built `./cmd/brief` from `5709f33` (the merge base with `origin/main`, pre-feature; extracted
with `git archive` into `$TMPDIR`, no worktree or stash touched) and from this branch's HEAD
(`af50392`), then diffed stdout, stderr and exit code per command line. **All 18 lines
byte-identical.** Every one: stdout empty, exit 2, one stderr line:

- `brief new --version` → `brief new: unknown flag: --version; run 'brief new <type> --help'`
- `brief help --version` → `brief help: unknown flag: --version; run 'brief help <command>'`
- `brief start --version demo` / `brief start --version` → `brief start: unknown flag: --version; run 'brief start <feature>'`
- `brief finish demo SCENARIO-01 --version` → `brief finish: unknown flag: --version; run 'brief finish <feature> <step> --handoff <path> --state <path>'`
- `brief status --version` → `brief status: unknown flag: --version; run 'brief status'`
- `brief check --version` → `brief check: unknown flag: --version; run 'brief check [feature]'`
- `brief new feature --version payments` → `brief new feature: unknown flag: --version; run 'brief new feature <name>'`
- `brief new step --version demo` → `brief new step: unknown flag: --version; run 'brief new step <feature>'`
- `brief completion --version` → `brief completion: unknown flag: --version; run 'brief completion <bash|zsh|fish|powershell>'`
- `brief start --version=x demo` → same line as `start --version` (pflag strips the value)
- `new`/`help` `--version=x` / `--version=` → same lines as their bare `--version`
- also checked, unchanged: `brief help start --version`, `brief --help --version`

The expected stderr strings in the new test are these captured literals, typed out per row
— never built from a production constant (e.g. `startInvocation`), which would pin nothing.

## Existing coverage — reuse, do not duplicate

- `new --version` — `flag_error_test.go` `Test_new_reports_the_version_flag_as_unknown`
  (with its own mutation note). Already pinned.
- `new`/`help` `--version=x` and `--version=` — the cross-site table
  `Test_classifies_dash_prefixed_tokens_consistently_across_disabled_parsing_sites`. Already
  pinned.
- `--help --version` (R8) — `run_test.go`. Already pinned.

**Gaps this scenario closes:** `help --version` (bare) and leaf `--version` on all seven
leaves. The cross-site table is the wrong home for `help --version`: its rows come in
root/new/help triples, and a bare root `--version` succeeds, so it has no usage-error row.

## Implementation Plan

Expected to be **green on arrival**: the behavior was never changed — the fold in
`runNew`/the help stub (`argUnknownFlag, argVersionFlag, argVersionFlagWithValue`) and pflag's
unknown-flag path at leaves already produce these bytes. Say so; do not manufacture a red.
The mutations in Step 2 are the evidence that the rows guard something.

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_reports_the_version_flag_as_unknown_outside_the_root` — black-box table through `cli.Run`, one assertion shape per row (`ErrUsage`, exit 2, empty stdout, exact stderr line); rows: `help --version`, `start --version demo`, `finish demo SCENARIO-01 --version`, `status --version`, `check --version`, `new feature --version payments`, `new step --version demo`, `completion --version`, `start --version=x demo` (new — green on arrival)
- [x] Step 2: mutation-verify each guard individually, stashing per `.claude/rules/agent-briefs.md` (use a `cp` to `$TMPDIR` or a uniquely tagged stash applied by SHA — the stash stack is shared) (verify):
  - help stub (`internal/cli/cli.go`, `newHelpCommand`'s switch): move `argVersionFlag` out of `case argUnknownFlag, argVersionFlag, argVersionFlagWithValue:` into the bodyless arm (`case argNotFlag, argVersionFlag:`) → predicted: only the `help --version` row red, now reporting `brief help: unknown command "--version"; expected one of: ...` (the bodyless arm falls through to `cmd.Root().Find(args)`); the cross-site `help --version=x`/`help --version=` rows stay green — the independence proof between the bare-flag and value arms. Any other failure shape means the mutation did not take as planned: stop and report
  - leaf control arm: register a `version` bool on `start`'s local flag set → predicted: exactly the two `start` rows red, every other row green. This proves the leaf rows are falsifiable. The other six leaf rows are **behavior pins, not guard evidence** — nothing brief-authored sits between pflag and those bytes (no `PersistentFlags` exist in `internal/cli`), so there is no guard to break; say so in the test comment, as STATE.md does for the `(devel)` row
  - record which mutation reddened which row in the test's doc comment (rule-shaped, no scenario IDs as narrative)
- [x] Step 3: `internal/cli/flag_error_test.go` `Test_new_reports_the_version_flag_as_unknown` — update its doc comment only: drop the "S06 owns the full ... table" pointer, which is now resolved (update)
- [x] Step 4: `go build ./...`, `go test ./...` (unpiped; report the test count and the delta — expect +9 subtests, no other change), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-06 done in `specification.md`

No production code changes. No new files. `cmd/brief` untouched.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- R6's baseline is the `5709f33` build's bytes, captured above; all 18 lines matched HEAD
  `af50392` — any later change to `runNew`/the help stub/leaf flag sets must keep these rows.
- Leaf `--version` stays a pflag unknown flag — no leaf registers a `version` flag, and
  `internal/cli` has no persistent flags; only the `start` rows are mutation-proven, the
  other leaf rows are behavior pins.
- `new --version` stays in its standalone test; `new`/`help` `--version=*` stay in the
  cross-site table. The new table owns only `help --version` and the leaves — one pin per
  line, no duplicates.

**Left unbuilt** — named so nobody assumes it exists:
- Root-help trailer `Run 'brief --version' to print the installed version.` — SCENARIO-07.
- Leaf `-v` (e.g. `brief start -v`) — still unpinned, out of scope (pflag path, not
  `classifyDashArg`).

**Traps** — things that look right and are not:
- The help-stub switch lists `argUnknownFlag, argVersionFlag, argVersionFlagWithValue` in one
  case. A mutation that removes the whole case breaks `exhaustive` lint/compile shape and
  reds `--bogus` too — not evidence. Move `argVersionFlag` alone.
- `brief help start --version` does not reach a leaf: the help stub treats the joined
  remainder as a command name (`unknown command "start --version"`). Do not use it as a
  leaf row.
- Leaf rows are green on arrival and stay green under every `classifyDashArg` mutation —
  leaves never call it. Only a flag-registration mutation reaches them.
- The shared git stash stack: never bare `git stash pop` during mutation runs.
