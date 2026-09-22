---
id: SCENARIO-14
status: done
depends-on: []
---

# SCENARIO-14: Start degrades on a missing optional convention and says so

## Scenario

```gherkin
Scenario: SCENARIO-14 Start degrades on a missing optional convention and says so  [orig: 03d]
  Given a feature missing only an optional convention
  When I start that feature
  Then the brief is assembled on stdout and the command succeeds
  And the shortfall is named on stderr, so the payload stays clean
```

## Measured behaviour on arrival (binary built from 0feb3f1, go1.27.1)

Fixture = `newStartFixture`'s shape (step with `## Scenario` + `## Implementation Plan`,
STATE.md with all four headings, conforming specification), one variable changed per run:

| fixture | exit | stdout | stderr |
| --- | --- | --- | --- |
| baseline | 0 | 246, full brief | empty |
| `## Scenario` section deleted from the step file | 0 | 208, section omitted entirely | **empty** |
| `## Scenario` present, body deleted | 0 | **208 — identical to the row above** | empty |
| `## Traps` section deleted from STATE.md | 0 | 228, section omitted entirely | **empty** |
| STATE.md replaced by prose (all four headings gone) | 0 | 135, step sections only | **empty** |
| specification with no progress heading (13's refusal) | 1 | 0 | one refusal line |
| freshly scaffolded `new feature` + `new step`, then `start` | 0 | 43 | **empty** |

**The stdout counts above are advisory, not assertion targets.** They were measured through
shell command substitution, which strips trailing newlines, and with `${#out}`, which counts
characters — the `—` in `<id> — <done> done` is multibyte. Every one of them is short of what
`stdout.String()` returns in a Go test. Do not paste them into an `assert.Equal`: a number
copied out of the binary's own output pins the binary, not the contract. The plan below
asserts stdout **differentially** instead — degraded stdout equals the conforming fixture's
stdout with the absent section's block removed — which proves "omitted entirely, no
placeholder" with no magic number.

So: **"the brief is assembled, exit 0" is green on arrival — do not manufacture a red for
it.** "The shortfall is named on stderr" is genuinely red: stderr is empty in every
degrade case. This scenario adds only the second half plus the assertions that pin the
first half against regression.

Two measurements drive the whole design:

1. Rows 2 and 3 are **byte-identical on stdout** (208 chars both). `writeSection` omits on
   `strings.TrimSpace(Body) == ""`, so stdout cannot distinguish absent from
   present-but-empty. `Section.Found` is the only thing that can. **The notice fires on
   `!Found`, never on an empty body** — 13 ruled present-but-empty conforming.
2. `stepSkeleton` writes no acceptance heading, and `stateSkeleton` writes all four state
   headings **present with empty bodies**. So a freshly scaffolded feature must emit
   **exactly one** notice (the absent `## Scenario`) and **zero** for the four empty state
   sections. An implementation keyed on `Body == ""` emits five. That fixture is the
   sharpest available discriminator and gets its own step.

## Decisions this scenario makes

- **`cfg.OptionalConventions` is not consumed.** It is `[]string`, defaults to `nil`, has
  zero production readers, and no vocabulary defines what an entry means. Defining one is
  new contract surface no Gherkin covers, and the scenario's "missing only an optional
  convention" is fully satisfied by the acceptance heading and the state headings. The
  conventions in effect are the ones that already have configured heading text:
  `cfg.AcceptanceHeading` and the four `cfg.StateHeadings`.
- **The descriptor originates in `assemble`, not `cli`.** `cli.runStart` cannot name the
  briefed step file: `Brief.Step` carries no path, and `Step.ID` is the frontmatter id,
  which STATE.md records as diverging from `pattern.ID(n)` after a hand edit. `Start`
  already computes the correct `stepPath` from the pattern for its 13-era refusals.
  Originating it in `assemble` also lets 15 marshal it instead of re-deriving from `Found`
  flags in a second place.
- **One stderr line per shortfall, not one combined line.** R14's "one line" bounds each
  message; `runStatus` already emits N lines for N malformed features and 11 ratified that.
  A combined line cannot carry N paths and N fixes through R14a's `<path>: <problem>;
  <fix>` template. Order is pinned: acceptance first, then `cfg.StateHeadings.Ordered()` —
  the same order `RenderText` uses — so tests assert exact stderr. No cap; R13 truncation
  stays unbuilt.
- **Path is absolute, not a third case.** SCENARIO-12's config-relative form exists because
  `status`'s "no features found in docs/specifications" names no file the user can open.
  Both of 14's notices name a real file (the briefed step file; the state file) and the fix
  is "add a heading to it", actionable only with a path that opens.
- **stdout is unchanged, byte for byte.** "So the payload stays clean" means no placeholder
  and no marker. **`render.go` and `render_test.go` are not touched by this scenario.**
- **Exit stays 0.** Degradation, not refusal. No `RefusalError`, no `ErrMalformedFeature`,
  no `renderRefusal`.

## Implementation Plan

- [x] Step 1: `internal/cli/start_test.go` `Test_start_names_an_absent_acceptance_heading_and_still_prints_the_brief` — call `newStartFixture(t, "open")` then **overwrite** `SCENARIO-01.md` with the same content minus its `## Scenario` section (do not edit `newStartFixture` itself — every other test shares it). Assert exit 0; stderr is exactly one line naming the absolute `SCENARIO-01.md` path; stdout equals the conforming run's stdout **with the `"\n## Scenario\n\nthe acceptance criteria\n"` block removed** — derived from the fixture, never a pasted byte count, and it proves the section is omitted with no placeholder (red — stderr is empty today)
- [x] Step 2: `internal/assemble/brief.go` — add `Shortfall{Path, Detail, Fix string}` and `Brief.Shortfalls []Shortfall`, nil when none. A distinct type from `Problem`, whose doc comment already binds it to "could not read" / "refused to assemble"; a shortfall is neither, and 15 needs its own JSON key (new)
- [x] Step 3: `internal/assemble/assemble_test.go` `Test_start_reports_an_absent_acceptance_heading_as_a_shortfall` — `Server.Start` against a fixture whose briefed step has no acceptance heading; assert `err` is nil, `brief.Step` is non-nil, and `Shortfalls` has one entry whose `Path` is the step file's absolute path (red)
- [x] Step 4: `internal/assemble/assemble.go` `(*Server).Start` — append the acceptance shortfall for the briefed step when `step.Acceptance.Found` is false, after the 13-era id and checklist refusals and using the same `stepPath` they already compute (green)
- [x] Step 5: fixture sweep, **before any cli-level change goes green** — run and act on `grep -n "stderr" internal/cli/start_test.go internal/cli/run_test.go`, `grep -n "StateFile\|STATE.md\|## Scenario" internal/cli/start_test.go internal/cli/run_test.go internal/assemble/assemble_test.go`, `grep -n "Brief{" internal/assemble/*_test.go`. The moment Step 6 lands, every existing fixture that reaches a brief while missing one of the five conventions starts writing stderr, and a `assert.Empty(t, stderr.String())` on it goes red. 13's plan under-counted its fixture list by four; enumerate, do not estimate. The greps are read-only, so running them here costs nothing (update)
- [x] Step 6: `internal/cli/start.go` `runStart` — write one line per `brief.Shortfalls` entry to stderr, `brief start: <path>: <detail>; <fix>`, through `flattenOneLine`, before the stdout brief and before the nil-Step notice; never via `renderRefusal`, which returns the error for `ExitCode` to classify. Fix up whatever Step 5's sweep named, in the same commit (green)
- [x] Step 7: `internal/cli/start_test.go` `Test_start_names_an_absent_state_heading_and_still_prints_the_brief` — `newStartFixture`, overwrite `STATE.md` without its `## Traps` section; assert exit 0, one stderr line naming the absolute `STATE.md` path, and stdout equal to the conforming run's minus the `"\n## Traps\n\na trap\n"` block (red)
- [x] Step 8: `internal/assemble/assemble.go` `(*Server).Start` — append one shortfall per state heading whose `Found` is false, in `cfg.StateHeadings.Ordered()` order, after the acceptance shortfall. `Start` recomputes `filepath.Join(featurePath, s.cfg.StateFile)` for the `Path`; **do not widen `readStateFile`'s signature** to return it — that function is on 13's refusal path (green)
- [x] Step 9: `internal/cli/start_test.go` `Test_start_names_every_absent_convention_on_its_own_line` — overwrite `STATE.md` with prose carrying none of the four headings AND drop the step's `## Scenario`; assert exit 0, stdout equal to the conforming run's minus **all five** blocks, and **exactly five** stderr lines in the pinned order: acceptance, Binding decisions, Left unbuilt, Traps, Open debts (new — the one-line-per-shortfall and ordering decision)
- [x] Step 10: `internal/cli/start_test.go` `Test_start_says_nothing_about_a_present_but_empty_convention` — `newStartFixture`, overwrite `STATE.md` so all four headings are present with empty bodies and the step's `## Scenario` heading is present with nothing under it; assert exit 0 and **stderr empty**. Differs from Step 9's fixture in exactly one variable: the headings are there. This is the `!Found` vs `Body == ""` discriminator (new)
- [x] Step 11: `internal/cli/start_test.go` `Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading` — drive `new feature demo`, `new step demo`, then `start demo` through `cli.Run`; assert exit 0, non-empty stdout, and **exactly one** stderr line, for the acceptance heading. `stateSkeleton` writes the four headings bare, so a `Body == ""` implementation emits five lines here (new — end-to-end proof of Step 10's rule on the shape the tool itself produces)
- [x] Step 12: `internal/cli/start_test.go` `Test_start_still_refuses_a_state_file_whose_fence_is_unterminated` — the control arm keeping 13 and 14 apart: same state file, present and readable, one variable different (an unclosed ``` fence instead of an absent heading); assert exit **1**, stdout **empty**, one refusal line. **Green on arrival** — 13 already refuses this and `assemble_test.go:526` covers it at the package level; what is new is only the cli-level arm, which is what 14's tests share a probe with. Do not manufacture a red for it; without it, deleting 13's fence check leaves 14's tests green (new)
- [x] Step 13: mutation-verify, **one at a time**, stashing each per the standing brief, and name which test each reddened: (a) drop the acceptance shortfall append (Step 4) → Steps 1, 3, 9 and 11's tests red, Step 7's still green; (b) drop the state-heading shortfall append (Step 8) → Steps 7 and 9's tests red, Steps 1, 3 and 11's still green; (c) change the firing condition from `!Found` to `Body == ""` → Steps 10 and 11 red, Steps 1, 7 and 9 still green
- [x] Step 14: `internal/assemble/doc.go` + `(*Server).Start` doc comment — replace "that degradation belongs to a later scenario" with the rule as it now is: an absent acceptance heading or state heading is reported as a `Shortfall` and does not refuse. State the rule, never the history (update)
- [x] Step 15: `internal/cli/start.go` `startUsage` — say that a missing optional convention is named on stderr and the brief still prints, still exiting 0. `Test_prints_the_start_usage_for_help` asserts on this text (update)
- [x] Step 16: `go build ./...`, `go test ./...` unpiped from the repo root, `go test -race ./internal/assemble/... ./internal/cli/...`, `golangci-lint run ./...`; report the exact test count and the delta → mark SCENARIO-14 done in `specification.md` (line 830)

## Out of scope — do not build

- `--json` and any JSON representation of a shortfall — SCENARIO-15.
- Consuming `cfg.OptionalConventions` — no owner (see Handoff).
- Any change to `stepSkeleton` to make it write the acceptance heading. Doing so would make
  the heading present-but-empty on every scaffolded step, so the shortfall would never fire
  for the case it exists to catch (a planner who never wrote the scenario).
- `finish` refusals (16-21), `check` (22), R13 truncation, heading/cap validation.
- Any `Store`, `WithX` option or `cmd/brief` change: `assemble` has no Store and this
  scenario injects no new dependency.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- `cfg.OptionalConventions` stays unconsumed — no vocabulary defines an entry, and defining
  one is contract surface 14's Gherkin does not cover; the conventions in effect are
  exactly `cfg.AcceptanceHeading` and the four `cfg.StateHeadings`.
- **A shortfall fires on `Section.Found == false`, never on an empty body.** 13 ruled
  present-but-empty conforming, `stateSkeleton` writes all four headings bare, and stdout
  is byte-identical (208 chars) for absent vs present-but-empty — so `Found` is the only
  discriminator that exists.
- `Brief.Shortfalls []Shortfall`, nil when none (not `[]`) — 15's `--json` needs `null`,
  matching `Status`'s existing nil-never-empty-slice rule.
- Shortfalls originate in `assemble.Start`, not `cli` — `Brief.Step` carries no path and
  `Step.ID` diverges from `pattern.ID(n)` after a hand edit, so `cli` cannot name the step
  file. 15 marshals `Shortfalls`; it must not re-derive them from `Found` flags.
- Order is acceptance, then `cfg.StateHeadings.Ordered()` — the same order `RenderText`
  emits sections in. Tests assert exact stderr against it.
- Exit stays **0** and stdout stays byte-identical; `render.go` was not touched. 15's JSON
  must keep the text payload unchanged.
- The notice is absolute-path (`brief start: <abs>: <detail>; <fix>`), no `(no files
  changed)` tail. 12's config-relative form is only for a notice naming no openable file.

**Left unbuilt** — named so nobody assumes it exists:

- `cfg.OptionalConventions` consumption, and any convention-name vocabulary — **no owner**.
- An empty configured heading as an off-switch for its convention — not built. Setting
  `acceptance-heading: ""` makes `markdown.Section` match the first blank line, so `Found`
  is true and no notice fires anyway. Unowned.
- Shortfalls for `cfg.ProgressHeading`, the handoff file, an id/filename mismatch — all
  still non-events; `check` (22) owns the first two, 11 ruled on the third.
- Anything JSON: `Shortfall` has no struct tags and no marshal test — 15 owns that.

**Traps** — things that look right and are not:

- **13's checklist refusal and 14's acceptance notice produce a byte-identical `Detail`**
  (`no "## X" heading found`, from the same `fmt.Sprintf` shape). A test asserting only
  `Contains(stderr, "no \"## Scenario\" heading found")` cannot tell a refusal from a
  notice. Every shortfall test must also assert exit 0 **and** non-empty stdout.
- `RenderText` omits a section on `strings.TrimSpace(Body) == ""`, so **stdout is the wrong
  probe for "was the heading there"** — absent and present-but-empty both render 208 chars.
  Use `Section.Found`.
- A freshly scaffolded feature is **not** shortfall-free: `stepSkeleton` writes no
  acceptance heading, so `new feature` + `new step` + `start` now prints one stderr line.
  That is intended; do not "fix" it by changing `stepSkeleton` (see *Out of scope*).
- Do not edit `newStartFixture` to build a degraded tree — it is shared by six tests. The
  local idiom is call it, then overwrite the single file, as
  `Test_start_refuses_a_specification_with_no_progress_heading` does.
- Adding `Shortfalls` to `Brief` is safe for `render_test.go`'s `assemble.Brief{...}`
  literals only because they are keyed. Confirm with `grep -n "Brief{" internal/assemble/*_test.go`
  before assuming it.
- 13's plan under-counted its fixture-update list by four files. Step 5's greps are the
  enumeration; run them **before** the cli change goes green, rather than reasoning about
  which fixtures are affected.
- **The stdout char counts in this plan's measurement table are not assertion targets.**
  They came from shell command substitution (trailing newlines stripped) and `${#out}`
  (characters, and `—` is multibyte), so each is short of what `stdout.String()` returns.
  Assert stdout differentially against the conforming fixture instead.
- `readStateFile` returns only `[]byte`. Resist widening it to hand back the state file's
  path for a shortfall — it sits on 13's refusal path; recompute the join in `Start`.
