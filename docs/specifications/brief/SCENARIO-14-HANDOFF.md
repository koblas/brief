# SCENARIO-14 Handoff

## What shipped

- `internal/assemble/brief.go`: `Shortfall{Path, Detail, Fix string}` and
  `Brief.Shortfalls []Shortfall`, nil when none.
- `internal/assemble/assemble.go` `(*Server).Start`: appends one `Shortfall` when the
  briefed step's `step.Acceptance.Found` is false, then one per `brief.Inherited` section
  whose `Found` is false, in `cfg.StateHeadings.Ordered()` order — acceptance first, state
  headings after, never on `Body == ""`.
- `internal/cli/start.go` `runStart`: writes one `brief start: <path>: <detail>; <fix>`
  line per `brief.Shortfalls` entry to stderr (through `flattenOneLine`), before the stdout
  brief and before the nil-Step notice. Never via `renderRefusal`. `startUsage` documents
  the behavior.
- `internal/assemble/doc.go` and `Start`'s own doc comment restate the rule as it now is,
  not as a deferred item.

## Decisions this scenario made — binding

- `cfg.OptionalConventions` stays unconsumed — no vocabulary defines an entry, and the
  Gherkin is fully satisfied by `cfg.AcceptanceHeading` and the four `cfg.StateHeadings`.
- A shortfall fires on `Section.Found == false`, never on an empty body —
  `stateSkeleton` writes all four headings bare, so `Body == ""` would over-fire.
- `Brief.Shortfalls` is nil when empty, not `[]` — matches `Status`'s existing rule, and
  15's `--json` needs `null`.
- Shortfalls originate in `assemble.Start`, not `cli` — `cli.runStart` cannot name the
  step file (`Brief.Step` carries no path; `Step.ID` can diverge from `pattern.ID(n)`).
  15 must marshal `Shortfalls` directly, never re-derive from `Found` flags.
- Order is pinned: acceptance, then `cfg.StateHeadings.Ordered()` — the same order
  `RenderText` renders sections in. One stderr line per shortfall, never combined.
- Exit stays 0, stdout stays byte-identical (`render.go`/`render_test.go` untouched). The
  notice is absolute-path, no `(no files changed)` tail (a read notice, not a write refusal).
- `Start` recomputes `filepath.Join(featurePath, s.cfg.StateFile)` for the state
  shortfall's `Path` rather than widening `readStateFile`'s signature, which sits on
  SCENARIO-13's refusal path.

## Verification

- Full sweep before the cli-level test went green (Step 5): grepped every fixture in
  `internal/cli/start_test.go`, `internal/cli/run_test.go` and
  `internal/assemble/assemble_test.go` that reaches a printed brief and asserts
  `stderr` empty. Only `start_test.go` drives `start`; its three such tests
  (`Test_prints_the_brief_and_writes_nothing_to_stderr`,
  `Test_prints_the_full_checklist_when_it_contains_a_nested_fence`,
  `Test_prints_the_brief_from_a_CRLF_step_file`) all carry every convention already —
  none needed updating.
- Mutation-verified individually, each reverted to a byte-identical file before the next:
  (a) dropped the acceptance-shortfall append → reddened Steps 1, 3, 9, 11; Step 7 stayed
  green. (b) dropped the state-heading-shortfall append → reddened Steps 7, 9; Steps 1, 3,
  11 stayed green. (c) changed the firing condition from `!Found` to
  `strings.TrimSpace(Body) == ""` → reddened Steps 10, 11; Steps 1, 7, 9 stayed green.
- `go build ./...` clean. `go test ./...`: 320 passing (delta +7 from the 313 baseline —
  the 7 new tests this scenario adds; every prior test still passes). 0 skips.
  `go test -race ./internal/assemble/... ./internal/cli/...` clean.
  `golangci-lint run ./...`: 0 issues.
- "The brief is still produced, exit 0" was green on arrival per the architect's plan —
  no red was manufactured for it. Only "says so on stderr" was genuinely red across
  Steps 1, 7 and 9.

## Left unbuilt / owned elsewhere

- `--json` and any JSON shape for `Shortfall` — SCENARIO-15. No struct tags, no marshal
  test written here.
- `cfg.OptionalConventions` consumption, and any vocabulary for what an entry means —
  unowned.
- An empty configured heading as an off-switch for its convention — not built;
  `acceptance-heading: ""` still matches the first blank line, so `Found` stays true.
- Shortfalls for `cfg.ProgressHeading`, the handoff file, or an id/filename mismatch —
  still non-events; unrelated to this scenario.

## Traps for the next reader

- 13's checklist/id refusal and 14's acceptance shortfall produce a byte-identical
  `Detail` string (`no "## X" heading found"`). A test asserting only `Contains` on that
  string cannot tell a refusal from a shortfall — always also assert exit code and
  stdout non-emptiness/emptiness.
- `RenderText` omits a section on `strings.TrimSpace(Body) == ""`, so stdout cannot
  distinguish an absent heading from a present-but-empty one — use `Section.Found`.
- A freshly scaffolded feature (`new feature` + `new step`) is not shortfall-free:
  `stepSkeleton` writes no acceptance heading, so `start` on it now prints exactly one
  stderr line. Intended — do not "fix" by changing `stepSkeleton`.
