# SCENARIO-12: A completed feature has no next step

## Scenario

```gherkin
Scenario: SCENARIO-12 A completed feature has no next step  [orig: 02]
  Given a feature whose steps are all done
  When I start the feature
  Then stdout is empty and the command succeeds
  And one line on stderr says the feature is complete
```

## Measured current behaviour

Built `./cmd/brief` (go1.27.1) and ran it against a scaffolded fixture project
(`alpha` = 2 steps, both finished; `beta` = 1 open step):

- `brief start alpha` (complete) — exit 0, stdout 0 bytes, stderr **0 bytes**
- `brief start beta` (open step, control) — exit 0, stdout 46 bytes, stderr 0 bytes
- `brief status` — exit 0, stderr 0 bytes, stdout two lines: `alpha 2/2 - 0` and
  `beta 0/1 SCENARIO-01 0`
- `brief start nosuch` — exit 1, stdout 0 bytes, stderr `brief start: no such feature`
- `brief start beta` with `STATE.md` removed — exit 1, stdout 0 bytes, stderr
  `brief start: malformed feature`

So: the `status` half of R14 (`-` in the next-step field for a complete feature) is
**green on arrival** — SCENARIO-09 built it. The `start` half is **silent**: exit 0,
empty stdout, and *nothing on stderr*. That silence is this scenario's red.
`assemble.RenderText` already returns early for `b.Step == nil`, and its doc comment
defers the case here verbatim.

## Contract this scenario pins

`brief start <feature>` where `Brief.Step == nil` — exit **0**, **empty stdout**, exactly
**one** line on stderr. Two discriminated sub-cases. The first is measured silent above;
the second is **derived**, not measured — an empty feature directory gives `readSteps` no
entries, so `Done`, `Open` are 0, `Step` is nil, and `RenderText` no-ops exactly as in the
all-done case:

1. every step done (`Done+Open > 0`):
   `brief start: <abs feature dir>: feature is complete, <done> of <total> steps done; run 'brief new step <feature>' to add the next one`
2. no step files at all (`Done+Open == 0`) — the state a bare `brief new feature x`
   leaves behind:
   `brief start: <abs feature dir>: no step files yet; run 'brief new step <feature>' to scaffold the first one`

Neither carries the `(no files changed)` tail — R14a drops it for read commands. Both are
written inline with `fmt.Fprintf(stderr, …)`, **not** through `renderRefusal`, exactly as
`runStatus` does for its no-features notice: `renderRefusal` returns the error for
`ExitCode` to classify, and completion is not an error.

Unchanged and non-zero: `ErrNoSuchFeature` → exit 1; `ErrMalformedFeature` (missing /
unreadable state file, unterminated fence) → exit 1; a step file whose frontmatter does
not parse → exit 1.

## Implementation Plan

- [x] Step 1: `internal/cli/start_test.go` `newStartFixture` — parameterize the existing helper by the frontmatter `status:` value so the complete and open arms differ in exactly one variable; keep every current caller passing `"open"` (update)
- [x] Step 2: `internal/cli/start_test.go` `Test_start_says_the_feature_is_complete_when_every_step_is_done` — `cli.Run` slice pinning empty stdout, the exact one-line stderr notice, and no error (red — stderr is empty today)
- [x] Step 3: `internal/cli/start_test.go` `Test_prints_the_brief_and_writes_nothing_to_stderr` — add the missing positive-stdout assertion to the existing open-step test so it pairs against Step 2's empty-stdout claim on the same probe; that pairing is what makes "stdout is empty" falsifiable (update)
- [x] Step 4: `internal/cli/start_test.go` `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature` — feature directory with a state file and zero step files; pins the second notice line, empty stdout, no error (red)
- [x] Step 5: `internal/cli/start.go` `runStart` — after a successful `srv.Start`, branch on `brief.Step == nil`: write the matching notice to stderr and return before `RenderText` (green)
- [x] Step 6: mutation-verify Step 5 — delete only the `brief.Step == nil` early return (still compiles, stdout still empty); Steps 2 and 4 must redden on the **stderr** assertion alone, Step 3 must stay green. Stash per the standing brief, restore, `diff` byte-identical (verify)
- [x] Step 7: `internal/cli/start_test.go` `Test_start_still_refuses_a_feature_with_no_state_file` — control arm proving the new nil-`Step` branch did not swallow the refusal path: exit 1, empty stdout, a message on stderr (new; no cli-level test covers this today — only `assemble_test.go`'s `Test_returns_an_error_when_the_state_file_is_missing`)
- [x] Step 8: `internal/cli/start_test.go` `Test_returns_an_error_for_an_unknown_feature_on_start` — leave as-is; re-run it as the second control arm, exit 1 for an absent feature (no change)
- [x] Step 9: `internal/cli/status_test.go` `Test_status_shows_a_dash_for_a_completed_feature` — the already-green half of R14, asserted end-to-end for the first time: `brief status` on a feature whose steps are all done prints `<name> 2/2 - 0` (new)
- [x] Step 10: mutation-verify Step 9 — remove the `if next == ""` substitution in `assemble.RenderStatusText` so the line renders `alpha 2/2  0`; Step 9 must redden. Restore and `diff` (verify)
- [x] Step 11: `internal/assemble/render.go` `RenderText` — rewrite the doc comment's last sentence: state the rule (`RenderText writes nothing when b.Step is nil; the caller decides what to say about a feature with no open step`), dropping the `SCENARIO-12` reference — a scenario id in a doc comment violates the documentation convention and is stale the moment this lands (update)
- [x] Step 12: `internal/cli/start.go` `startUsage` — add the sentence documenting the complete/empty case, mirroring how `statusUsage` documents its no-features notice (update)
- [x] Step 13: run the standing brief's verification commands on `internal/cli` and `internal/assemble`; report the exact test count and the delta (verify)
- [x] Step 14: all tests green → mark SCENARIO-12 done in `specification.md`, and rewrite `docs/specifications/brief/STATE.md` folding in the Handoff below (update)

## Out of scope — do not build

- **SCENARIO-13's refusal.** Nothing in this scenario may make `start` read
  `cfg.SpecificationFile` or `cfg.ProgressHeading`. 13's red is guaranteed today because
  `Start` never opens the specification file at all — it reads `cfg.StateFile` and the
  step files only. Keep it that way.
- **`readSteps`' intolerance.** SCENARIO-11 deliberately left it strict and pinned that
  with `internal/assemble/assemble_test.go`
  `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`. Do not touch
  that test, `readSteps`, `featureStatus`'s tolerance, `assemble.Problem`, or
  `RenderStatusText`'s `!` row.
- **`--json` (SCENARIO-15)**, the optional-convention degradation notice
  (SCENARIO-14), `Problem.Line`, and any new sentinel for completion.
- **`assemble.Start`'s signature.** It keeps returning `(Brief, nil)` for a complete
  feature. No `ErrComplete`, no bool, no second return value.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- `Brief.Step` stays a `*Step`, nil means "no open step", and `assemble.Start` returns
  `(Brief, nil)` for it — never an error — because SCENARIO-15's `--json` serializes that
  pointer directly as R14's `"step": null` discriminator; an error or a sentinel would
  force 15 to reconstruct the payload from a failure path.
- The notice lives in `cli.runStart`, keyed on `brief.Step == nil`, written inline with
  `fmt.Fprintf(stderr, …)` and followed by `return nil` — the same split SCENARIO-10 used
  for `runStatus`'s `len(rows) == 0` notice. `assemble` returns data; `cli` renders copy.
- **Two notices, not one:** `Done+Open > 0` → "feature is complete, N of N steps done";
  `Done+Open == 0` → "no step files yet". A bare `brief new feature x` leaves a directory
  that is *conforming* (SCENARIO-11 decided that) and hits the same nil-`Step` branch;
  one merged notice would tell a first-run user their empty feature is "complete".
- **Path-form rule for `start`'s stderr, settled here because SCENARIO-13 renders on the
  same path:** a message naming a **specific on-disk object** (a feature directory, a
  file) uses the **absolute** path — SCENARIO-11's malformed-row lines already do. A
  message naming a **configured location** that is not one object stays config-relative —
  SCENARIO-10's `no features found in docs/specifications` does. 12's notice names one
  feature directory, so it is absolute. 13 and 14 inherit this; do not re-decide it.
- Read notices and read refusals both drop R14a's `(no files changed)` tail. Only write
  refusals carry it.

**Left unbuilt** — named so nobody assumes it exists:

- `assemble.Server.Next` / `.Show` / `.Handoff` / `.StateGet`, `--json` (15),
  `assemble.Problem.Line`, `markdown.Headings`, R13 truncation — all still unbuilt.
- No refusal for a missing progress list — SCENARIO-13 owns it, and `Start` still never
  opens `cfg.SpecificationFile`.
- No optional-convention shortfall notice — SCENARIO-14 owns it. After 12, `start` can
  emit at most one stderr line; 14 is what makes two possible.

**Traps** — things that look right and are not:

- **`brief start <f> | wc -c == 0` is exit-code-conditional.** Measured: `start nosuch`
  and `start` on a feature with no state file both produce **0 bytes on stdout** at
  **exit 1**. The scriptable "complete" test is `exit 0 && wc -c == 0`, never byte count
  alone. R14's prose omits the exit-code half.
- **`"step": null` cannot distinguish complete from zero-steps** — both are
  `Step == nil`. SCENARIO-15 must use `done`/`open` (`2`/`0` vs `0`/`0`) to tell them
  apart, or accept the conflation deliberately.
- `runStart` returns immediately after the notice, so `RenderText` is never reached with a
  nil `Step` from this path. Its nil guard stays as defense-in-depth and is covered only
  by `assemble/render_test.go` `Test_RenderText_writes_nothing_when_there_is_no_next_step`
  — deleting that guard will not redden any `cli` test.
- `brief finish <feature> <n>` takes the **step id** (`SCENARIO-01`), not the number;
  `finish alpha 1` refuses with "no step file found". Relevant to any fixture built by
  driving the real binary.
