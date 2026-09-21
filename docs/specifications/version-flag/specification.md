# Specification: `brief --version`

## Intent & Goal

**Primary Goal**: A person with `brief` on PATH, or a script, can ask the installed binary
which build it is: `brief --version` prints one line on stdout and exits 0.

**Out of Scope**: `-v` shorthand; a `brief version` subcommand; `--version` on subcommands;
`--version --json`; `-ldflags -X` overrides; tags, goreleaser, release workflows (separate
backlog item); shell completion for `--version`.

**Business Rules**: Version comes from Go's build info (`debug.BuildInfo.Main.Version`),
printed verbatim — Go ≥ 1.24 already stamps the tag, the pseudo-version, and `+dirty`.

## Business Rules & Invariants

- R1: `brief --version` (sole argument) prints exactly `brief <Main.Version>\n` to stdout,
  nothing on stderr, exit 0. The second whitespace-separated field is a script contract.
- R2: `Main.Version` of `(devel)`, empty, or no build info at all prints `brief (devel)`,
  still exit 0. Asking for the version never fails.
- R3: The build-info source is replaceable in tests (no real binaries needed to cover
  R1/R2); the architect chooses the seam.
- R4: Argument rules mirror `--help`'s sole-argument contract. One line on stderr, exit 2:
  - trailing argument: `brief: '--version' takes no arguments; run 'brief --version'`
  - value: `brief: '--version' takes no value; run 'brief --version'`
- R5: `-v` (and clusters like `-v=x`, `-vh`, `-hv`) keep today's unknown-shorthand wording.
- R6: `brief new --version`, `brief help --version`, and leaf `--version` (e.g.
  `brief start --version demo`) stay byte-identical to today. `classifyDashArg` is shared by
  root, `new` and the help stub, so a new arg kind must map back to today's message at
  `new` and `help`.
- R7: Root help gains one trailer line after `Run 'brief <command> --help' for details.`:
  `Run 'brief --version' to print the installed version.`
- R8: First argument decides: `brief --help --version` keeps today's
  `'--help' takes no arguments` error.

---

## Triage Brief

**Affected surface.** `internal/cli/classify.go` (`classifyDashArg`, `argKind`) — today
`--version` resolves to `argUnknownFlag` and `-v` to an unknown shorthand;
`internal/cli/cli.go` `runRoot` (the switch on `classifyDashArg(args[0])`), the help stub
(`newHelpCommand`) and `internal/cli/new.go` `runNew` (the other two callers);
`helpTemplate` root trailer; `cmd/brief/main.go` (thin wiring only).

**Cobra's own version support is inert.** Root uses `DisableFlagParsing: true`, so
`ParseFlags` no-ops and `Command.Version`/`InitDefaultVersionFlag` never fire; when
reachable it prints via cobra's own template, bypassing brief's output contract. Hand-roll
it in `classifyDashArg` + `runRoot`, like `-h`/`--help`.

**Build info facts (go1.27.1, verified).** `go build` in a checkout stamps
`Main.Version = v0.0.0-20260921145925-5709f33424c3` (pseudo-version, no tags exist),
`vcs.revision`, `vcs.time`, `vcs.modified=false`. No Makefile, goreleaser, CI, ldflags,
or tags exist.

**Tests pinning current behavior.** `internal/cli/run_test.go:397-406`
(`Test_returns_a_usage_error_for_an_unknown_double_dash_flag_at_the_root`) uses
`--version` as its example bogus flag — repoint to `--bogus` so it keeps testing what it
was written to test. No test pins `-v` by name.

**Already exists — do not re-plan.** `classifyDashArg` and the three callers' switch;
`usageError`/`ExitCode` (exit 2 for usage, 0 for nil); the root help golden in
`internal/cli/help_test.go`; `--help`'s sole-argument copy family
(`'<flag>' takes no arguments` / `takes no value`).

## Product Verdict

**SHIP WITH CHANGES** (`product-vision`, pre-design), all folded into rules above:
1. Root `--version` only — no `-v` (reserved for a future `--verbose`), no `version`
   subcommand, not on subcommands.
2. Print `Main.Version` verbatim, `brief ` prefix, `brief (devel)` fallback, exit 0 always.
3. No `--json` (scalar; spec Decisions taken #3).
4. Argument rules and exact copy per R4/R5/R8; `new`/`help` byte-identical (R6).
5. Release tooling out of scope; `ReadBuildInfo` only.
6. One root-help trailer line (R7).

---

## Scenarios (Gherkin)

```gherkin
Scenario Outline: SCENARIO-01 --version prints the build's stored version
  Given brief was built with stored module version "<stored>"
  When I run "brief --version"
  Then stdout is exactly "brief <stored>" followed by a newline
  And stderr is empty and the exit code is 0
  Examples:
    | stored                                          |
    | v0.3.0                                          |
    | v0.0.0-20260921145925-5709f33424c3              |
    | v0.0.0-20260921145925-5709f33424c3+dirty        |

Scenario: SCENARIO-02 A build with no stored version reports (devel)
  Given brief's build info has version "(devel)", an empty version, or no build info at all
  When I run "brief --version"
  Then stdout is exactly "brief (devel)" followed by a newline, and the exit code is 0

Scenario: SCENARIO-03 --version takes no arguments
  When I run "brief --version extra" or "brief --version --json"
  Then stderr is exactly: brief: '--version' takes no arguments; run 'brief --version'
  And stdout is empty and the exit code is 2

Scenario: SCENARIO-04 --version takes no value
  When I run "brief --version=x" or "brief --version="
  Then stderr is exactly: brief: '--version' takes no value; run 'brief --version'
  And stdout is empty and the exit code is 2

Scenario: SCENARIO-05 -v stays an unknown shorthand
  When I run "brief -v"
  Then stderr is exactly: brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'
  And the exit code is 2

Scenario: SCENARIO-06 --version outside the root is unchanged
  When I run "brief new --version", "brief help --version" or "brief start --version demo"
  Then each prints exactly what it prints today (an unknown-flag usage error, exit 2)

Scenario: SCENARIO-07 Root help points to --version
  When I run "brief --help"
  Then its last line is "Run 'brief --version' to print the installed version."
  And it comes right after "Run 'brief <command> --help' for details."
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: --version prints the build's stored version
- [x] SCENARIO-02: A build with no stored version reports (devel)
- [ ] SCENARIO-03: --version takes no arguments
- [ ] SCENARIO-04: --version takes no value
- [ ] SCENARIO-05: -v stays an unknown shorthand
- [ ] SCENARIO-06: --version outside the root is unchanged
- [ ] SCENARIO-07: Root help points to --version
