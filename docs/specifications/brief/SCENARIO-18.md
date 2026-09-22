---
id: SCENARIO-18
status: done
---

# SCENARIO-18: An over-cap state body is refused and nothing lands

## Scenario

```
Scenario: SCENARIO-18 An over-cap state body is refused and nothing lands  [orig: 06b]
  Given an open step whose checklist is complete
  When I finish it with a replacement state body over the configured cap
  Then the refusal names the measured line count and the cap
  And both files are byte-identical
```

## Measured starting point (measured here — do not re-measure)

- **`cfg.StateCapLines` is still unconsumed.** `grep -rn StateCapLines --include="*.go" .`
  returns exactly three hits: `internal/platform/config/config.go:58` (the
  `state-cap-lines` field), `:84` (the shipped default **80**), and a mention inside
  `internal/scaffold/errors.go`'s `ErrOverCap` doc comment. No code reads it.
- **An over-cap state body finishes at exit 0 today.** Built `./cmd/brief` (go1.27.1), ran
  `brief new feature widgets` + `brief new step widgets` in a temp dir, then
  `brief finish widgets SCENARIO-01 --handoff handoff.md --state state.md` with a 100-line
  `state.md`: stdout `brief finish: SCENARIO-01 is done`, **exit 0**, and `STATE.md` lands
  at 100 lines against the 80-line default cap.
- **Fixture state-body enumeration** (the boundary is exactly `internal/scaffold/*_test.go`,
  because `fixtureConfig` is package-local to those files; `--state` appears in only one
  other test file, `internal/cli/finish_test.go`, which runs at `config.Default()`'s cap of
  80 with bodies of ≤15 lines). Every `state` argument handed to `Finish` in `scaffold`'s
  tests, enumerated from every `.Finish(` call site:
  - `newStateBody(cfg)` (`finish_test.go:82`) — four headings × `"## X\n\nNEW-STATE-ENTRY\n\n"`
    = **16 lines**. This is the maximum.
  - `"## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"`
    (`finish_test.go:309`, `:352`, `:410`, `step_test.go:85`) — 7 lines.
  - `badState` (`finish_test.go:317`) — 4 lines. `differentState`
    (`finish_idempotent_test.go:135/153/175/198`) — 1 line. `[]byte("s")` — 1 line.
  - So `fixtureConfig.StateCapLines = 20`: strictly above the 16-line maximum, distinct from
    the 10-line `HandoffCapLines` fixture value (so a wrong-key bug is visible), and distinct
    from the 80-line default (so the value is proven to come from config).

## Decisions this scenario takes

1. **Nothing new is designed.** The check is a second call to the existing generic
   `checkArgumentCap(body, source, label, limit)` helper with `StateSource`, `"state"` and
   `cfg.StateCapLines`. `markdown.CountLines`, `ErrOverCap`, the `StateSource` placeholder
   and `cli/finish.go`'s `StateSource` → `sourceLocator(*statePath)` switch branch are all
   reused verbatim; **no production change outside `internal/scaffold/finish.go`.**
2. **Order — the "cap band".** The state cap check sits immediately **after** the handoff cap
   check and immediately **before** `checkArgumentFence(state, StateSource, "state")`
   (`finish.go:151-157`). Consequences, each pinned by its own test: the **handoff** cap is
   reported first when both bodies are over (16's handoff-first rule); the state **cap** is
   reported before the state's unclosed **fence**; and, being in the argument band ahead of
   `r.verdict()`, an over-cap state on an already-done step reports `ErrOverCap`, never
   `ErrAlreadyFinished`.
3. **Copy** — R14a, write refusal, so the `(no files changed)` tail is present. Rendered
   straight out of the shared helper, no new format string:

   ```
   brief finish: <state source>: state is 81 lines, over the cap of 80; cut the state to 80 lines or fewer, or raise state-cap-lines in .brief.yaml, and retry (no files changed)
   ```

   `RefusalError.Path` is `StateSource`, `Line` is **0** (a count, not a location), exit
   **1**, stderr only, stdout empty.
4. **Boundary is `count > cap`** — exactly-at-cap accepted, `cap+1` refused, each pinned by
   its own test and mutation-verified on the **state call site** so the handoff side stays
   green.

## User-visible contract

`brief finish <feature> <step> --handoff <path> --state <path>` with a `--state` body over
`state-cap-lines`: stdout empty, one R14a line on stderr (copy above, `<state source>` = the
`--state` path, or `<stdin>` when it was `-`), **exit 1**, and every file byte-identical with
its modification time unmoved. Validation matrix: the happy path (at-cap accepted) and the
invariant violation (cap+1 refused) are this scenario's arms; missing argument, unreadable
`--state` file, unknown feature/step and malformed input are already pinned by the 05/16/17-era
tests in `internal/cli/finish_test.go` — already covered, not omitted.

## Implementation Plan

- [x] Step 1: `internal/scaffold/scaffold_test.go` `fixtureConfig` — set `StateCapLines = 20`
      and extend the existing cap comment to record the 16-line `newStateBody` maximum the
      value clears (update)
- [x] Step 2: `internal/scaffold/finish_cap_test.go` — rename `overCapHandoff` → `bodyOfLines`
      (used for both bodies now) and make it emit genuinely distinct lines, which its comment
      already claims and today's repeated `"line"` does not; 17's pinned copy names counts,
      not content, so nothing else moves (update)
- [x] Step 3: `internal/scaffold/finish_cap_test.go`
      `Test_refuses_a_state_body_one_line_over_the_configured_cap` — `Finish` with a
      `cap+1`-line state; `errors.Is(err, scaffold.ErrOverCap)`, `Path == scaffold.StateSource`,
      `Line == 0`, and `Equal` on the whole `Error()` string. **Do not weaken that `Equal` to
      `Contains`**: `over the cap of 20` is what separates a correct check from one reading
      `cfg.HandoffCapLines` (red)
- [x] Step 4: `internal/cli/finish_test.go`
      `Test_refuses_a_state_body_over_the_cap_and_names_the_state_path` — through `cli.Run`
      against `newFinishCLIFixture` (default cap 80) with an 81-line `--state` file built by
      `overCapBody(81)`; `Equal` on the whole stderr string including `over the cap of 80` and
      the `(no files changed)` tail, `cli.ExitCode(err) == 1`, stdout empty. Red for the same
      single reason as Step 3 — no `cli/finish.go` change exists behind it, and it greens the
      moment Step 5 lands (red)
- [x] Step 5: `internal/scaffold/finish.go` — second `checkArgumentCap` call for the state
      argument, between the handoff cap check and `checkArgumentFence` (green)
- [x] Step 6: `internal/scaffold/finish_cap_test.go`
      `Test_accepts_a_state_body_of_exactly_the_configured_cap` plus
      `..._with_no_trailing_newline` — both succeed and both land, read `cfg.StateFile` back
      (green)
- [x] Step 7: `internal/scaffold/finish_cap_test.go`
      `Test_a_refused_over_cap_state_body_leaves_every_file_byte_identical` — `snapshotTree`
      paired with the `pinModTimes`/`pinnedModTime`/`modTimes` probe, exactly as 17's handoff
      twin does; both probes' control arms already live in `finish_idempotent_test.go` — cite
      them, do not duplicate (new)
- [x] Step 8: `internal/scaffold/finish_cap_test.go`
      `Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps` — both bodies
      over their own caps; refusal `Path == HandoffSource` and the whole message names
      `handoff`. This test is vacuous without mutation (d) in Step 11 — it passes with the
      state check deleted (new)
- [x] Step 9: `internal/scaffold/finish_cap_test.go`
      `Test_reports_the_state_cap_before_the_state_s_unclosed_fence` — a state body both over
      cap and opening a fence it never closes reports `ErrOverCap`, not `ErrUnterminatedFence`
      (new)
- [x] Step 10: `internal/scaffold/finish_cap_test.go`
      `Test_an_over_cap_state_body_on_a_done_step_reports_the_cap_not_the_re_finish_refusal` —
      `newFinishedFixture`, then `Finish` with an over-cap state; `errors.Is(ErrOverCap)` true,
      `errors.Is(ErrAlreadyFinished)` false (new)
- [x] Step 11: mutation-verify, **one variable at a time, each stashed** per the standing
      brief. Each mutation names its predicted red set — if the actual set differs, stop and
      report rather than proceeding: (a) delete the state cap call → reds Steps 3, 4, 7, 9
      (that body's unclosed fence fires instead of the cap) and 10; Step 6, Step 8 and every
      17-era handoff test stay green. (b) in `checkArgumentCap`, change `n <= limit` to
      `n < limit` → reds Step 6's two at-cap tests plus 17's
      `Test_accepts_a_handoff_of_exactly_the_configured_cap` and its no-trailing-newline twin;
      every over-cap test stays green. This is the one guard the two caps share — the distinct
      guards are the two call sites, isolated by (a) and (d); do **not** mutate the limit at a
      call site, since Steps 3 and 4 assert the whole `Error()` string and the limit is
      embedded in it. (c) move the state cap check below
      `checkArgumentFence` → reds Step 9 only. (d) swap the handoff and state cap call sites →
      reds Step 8 only (verify)
- [x] Step 12: `internal/scaffold/finish.go` `Finish` doc comment — say the state argument is
      checked against `cfg.StateCapLines`, in its real position in the enumerated order (after
      the handoff cap, before the state fence check); while editing **those two sentences
      only**, drop the `(SCENARIO-17)` narrative markers the convention bans and add no
      `(SCENARIO-18)` — no other sweep of the file. `go doc ./internal/scaffold` is the check
      (update)
- [x] Step 13: verification per the standing brief, then tick SCENARIO-18 in
      `specification.md` and rewrite `docs/specifications/brief/STATE.md` from this Handoff
      (update)

## Out of scope — do not build

SCENARIO-19 (state missing a required heading), 20 (open checklist item), 21 (unfinished
`depends-on`), 22 (`check`), and R13's `cfg.DefaultOutputBudgetBytes` byte budget — a byte
budget on the *read* path with truncation semantics, not a line cap on the write path. No
`--force`, no per-invocation override, no truncation, no validation of the cap *values*.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **Both caps live in one adjacent "cap band" in `Finish`'s argument phase, handoff first,
  then state, then `checkArgumentFence(state, …)`.** Three orderings depend on that and each
  is pinned: handoff reported first when both are over
  (`Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps`); state cap
  reported before the state's unclosed fence
  (`Test_reports_the_state_cap_before_the_state_s_unclosed_fence`); the band runs ahead of
  `(refinish).verdict()`, so an over-cap state on a done step reports `ErrOverCap`, never
  `ErrAlreadyFinished`. **SCENARIO-19's required-heading check goes after the fence check**,
  not inside the cap band — a body that fails the fence scan cannot have its headings read.
- **`cfg.StateCapLines` (default 80, yaml `state-cap-lines`) is now consumed**, via the same
  generic `checkArgumentCap` and the same `ErrOverCap` sentinel; `RefusalError.Path` =
  `StateSource` is what tells the two caps apart. `Line` is 0. Copy, tail included:
  `brief finish: <source>: state is <n> lines, over the cap of <cap>; cut the state to <cap>
  lines or fewer, or raise state-cap-lines in .brief.yaml, and retry (no files changed)`.
- **`fixtureConfig.StateCapLines = 20`** — above the 16-line `newStateBody` maximum (the
  largest state body any `scaffold` test hands `Finish`), distinct from `HandoffCapLines = 10`
  and from the 80-line default. SCENARIO-19 must keep it above whatever state bodies it adds.
- **No production code outside `internal/scaffold/finish.go` changed.** `cli/finish.go`'s
  `StateSource` branch predates 17 and needed nothing.

**Left unbuilt** — named so nobody assumes it exists:

- Required-heading validation of the state body (19), open-checklist refusal (20),
  `depends-on` refusal (21), `check` (22). `cfg.DefaultOutputBudgetBytes` — still unconsumed,
  R13, no owner. Validation of cap *values* (0 or negative is accepted as configured; a cap of
  0 renders the cosmetically odd "is 1 lines") — unowned.
- No `--force`, no per-invocation cap override, no truncation — refusal is the only outcome.

**Traps** — things that look right and are not:

- **`Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps` passes with the
  state check deleted.** Its only real proof is mutation (d), swapping the two call sites. Any
  later edit to the cap band must re-run that mutation, not just the suite.
- **The cap tests' bodies are bare `"line"` text with no state headings.** They finish
  successfully today because nothing validates the state body's structure — **SCENARIO-19 will
  refuse them**. 19 owns giving every at-cap/accepting state fixture the four configured
  headings; do not read those bodies as intentionally heading-less.
- **`checkArgumentCap`'s `n <= limit` is one guard shared by both caps** — mutating it reddens
  the handoff *and* state at-cap tests together, and that is correct, not a failure to isolate.
  The distinct guards are the two call sites; isolate those by deleting or swapping a call,
  never by mutating a call site's limit — every over-cap test asserts the whole `Error()`
  string and the limit is embedded in it, so a `limit-1` mutation reddens the over-cap tests
  too and proves nothing about the boundary.
- **This repository's own `STATE.md` (200+ lines) is now over the 80-line cap `brief` enforces**
  — `brief finish brief <step>` refuses on this feature's own state body as of this scenario.
  Harmless in practice (the tool is not self-hosted; `brief start brief` still fails on missing
  frontmatter, and pipeline agents write `STATE.md` by hand), and R18 gives the reporting role
  to `check` (22). Present tense now, not prospective.
- `cli/finish.go`'s placeholder swap only upgrades a `RefusalError.Path` it recognizes; both
  `StateSource` and `HandoffSource` have branches today, and any future placeholder needs its
  own.
