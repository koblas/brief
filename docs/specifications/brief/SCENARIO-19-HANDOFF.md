# SCENARIO-19 Handoff: A state body missing a required heading is refused

## What landed

`(*scaffold.Server).Finish` now checks the `state` argument against
`cfg.StateHeadings`: every one of `cfg.StateHeadings.Ordered()`'s four
headings must have a `markdown.Section` match in the argument bytes,
presence-only (an empty section is valid), in any order. The check is
`checkArgumentHeadings(body, source, label, headings)`, new in
`internal/scaffold/finish.go`, placed immediately after
`checkArgumentFence(state, StateSource, "state")` and before the
specification read — an open fence would leave a later heading unreadable,
so the fence must close first. Only the first missing heading (in
`Ordered()` order) is named; Finish is a refusal that stops, unlike
`assemble.Start`'s shortfall degrade (SCENARIO-14), which completes and
reports every shortfall.

New sentinel `ErrMissingStateHeading` in `internal/scaffold/errors.go`,
added to `doc.go`'s sentinel enumeration alongside `ErrOverCap` (which 17/18
had introduced but never listed — `go doc ./internal/scaffold` was one
sentinel short before this scenario).

`internal/cli/finish.go`'s `finishUsage` gained one clause on the `--state`
line; no other cli production change — the `StateSource`-to-real-path
`switch` branch predates this scenario and needed nothing.

## Refusal copy

```
brief finish: <path>: state is missing the "## Binding decisions" section; add a "## Binding decisions" heading to the state body — an empty section is valid — and retry (no files changed)
```

`Line` is always 0 — a missing heading has no line. Deliberately unlike
14's `no %q heading found`, so the two stay distinguishable by text as well
as by error type (13/14 share a `Detail` string, per STATE.md's trap).

## Tests added (13: 12 scaffold + 1 cli)

`internal/scaffold/finish_headings_test.go` (new file):

- `Test_refuses_a_state_body_missing_a_required_heading` — pins the whole
  refusal line against the fixture's own retitled heading (`"## Gotchas"`).
- `Test_accepts_a_state_body_whose_sections_are_all_empty` — presence, not
  content, is the trigger.
- `Test_accepts_a_state_body_whose_headings_are_out_of_configured_order` —
  records the no-order-enforcement decision as a green test.
- `Test_accepts_the_state_body_new_feature_writes` — hands `Finish` the
  exact bytes `NewFeature` wrote to disk, read back rather than re-derived.
- `Test_refuses_a_state_body_whose_headings_are_only_inside_a_fenced_block`
  — the measured case that succeeded before this scenario; proves
  `markdown.Section`, not a substring scan.
- `Test_refuses_an_empty_state_body_naming_the_first_configured_heading`.
- `Test_names_the_first_configured_heading_when_several_are_missing` —
  vacuous alone (also passes checking only `Ordered()[0]`); paired with the
  next test and a mutation for the real proof.
- `Test_refuses_a_state_body_missing_only_the_last_configured_heading` —
  proves the loop covers every position.
- `Test_reports_the_state_cap_before_a_missing_heading`.
- `Test_reports_the_state_s_unclosed_fence_before_a_missing_heading`.
- `Test_a_state_body_missing_a_heading_on_a_done_step_reports_the_heading_not_the_re_finish_refusal`
  — the heading check pre-empts `(refinish).verdict()`.
- `Test_a_refused_missing_heading_leaves_every_file_byte_identical` — paired
  snapshot + mtime probes, control arms cited from
  `finish_idempotent_test.go`, not duplicated.

`internal/cli/finish_test.go`:

- `Test_refuses_a_state_body_missing_a_heading_and_names_the_state_path` —
  through `cli.Run`, `Equal` on the whole stderr line, exit 1, empty
  stdout, `(no files changed)` tail.

## Fixture updates — nine, not six

The architect's plan enumerated six pre-existing tests that would break.
Running the suite after wiring the check surfaced three more, all in
`internal/scaffold/finish_test.go`, using a throwaway `[]byte("s")` state
body to reach a specification- or state-file check unrelated to headings:
`Test_refuses_a_specification_with_no_progress_heading_on_finish`,
`Test_refuses_a_progress_list_with_no_entry_for_the_step`,
`Test_refuses_a_missing_state_file_on_finish`. Each was repointed at
`oldStateBody(cfg)` (already defined in that file for this exact fixture's
headings) in place of `"s"` — none of the three write a state file to disk,
so the fix needed no other change; `checkArgumentHeadings` validates only
the argument bytes.

The six the plan named:

- `internal/scaffold/finish_cap_test.go`: added `stateBodyOfLines(cfg, n)`
  (four configured headings plus distinct filler, exactly `n` lines) and
  repointed `Test_accepts_a_state_body_of_exactly_the_configured_cap` and
  its no-trailing-newline twin at it, in place of headingless `bodyOfLines`.
- `internal/scaffold/finish_idempotent_test.go`: all four
  `differentState := []byte("DIFFERENT-STATE-BODY\n")` literals repointed
  at `differentStateBody(fx.cfg)`. Two of these (in
  `Test_re_finishing_a_done_step_with_a_different_state_body_is_refused`
  and `Test_a_refused_re_finish_leaves_every_file_byte_identical`) were the
  dangerous kind STATE.md flagged: before the fix they still failed, but
  with `ErrMissingStateHeading` in the chain instead of
  `ErrAlreadyFinished` — the wrong sentinel, not a build break, which is
  exactly why "run the suite" beats "read one file" for this sweep.
- `internal/scaffold/finish_test.go`: new `differentStateBody(cfg)` helper
  beside `newStateBody`, four configured headings each carrying
  `DIFFERENT-STATE-ENTRY`.

`internal/scaffold/scaffold_test.go`: `fixtureConfig`'s comment tightened
to name `newStateBody`/`oldStateBody`/`differentStateBody` as the 16-line
bodies StateCapLines(20) stays above, and to note that the cap/heading
tests deliberately build boundary-sized bodies at or over that line.

`internal/cli/finish_test.go`'s own state fixtures needed no change — every
one already carries the four default headings; confirmed empirically by
the full-suite run rather than assumed from the plan's file-by-file read.

## Mutation verification (Steps 24-28), each stashed via file copy and
restored byte-identical

- **(24) delete the `checkArgumentHeadings` call** → red: Steps 1, 13, 14,
  15, 16 (5 tests). Exact match.
- **(25) tighten the predicate to `!found || strings.TrimSpace(section) ==
  ""`** → red: Step 10 (`..._sections_are_all_empty`) only. Step 1
  (missing-heading refusal) stays correctly passing — the mutation does not
  change *why* it refuses. Exact match.
- **(26) replace `markdown.Section` with `strings.Contains`** → red: Step
  13 (fenced-headings) only. Exact match — this is the one mutation that
  isolates fence-awareness.
- **(27a) reverse the iteration over `Ordered()`** → red: Step 15
  (`..._when_several_are_missing`) only. Exact match.
- **(27b) check only `Ordered()[0]`** → red: Step 16
  (`..._missing_only_the_last_configured_heading`) only; Step 15 alone
  stays green under this mutation, confirming it is vacuous without Step
  16 — the same failure shape 17/18 paid for with the cap-order test.
- **(28a) move the heading check above `checkArgumentCap`** → red: Step 17
  only. **(28b)** move it above `checkArgumentFence` (still after the cap
  band) → red: Step 18 only. **(28c)** move it below the `verdict()` switch
  → red: Step 19 only. Each run individually, restored between.

## Verification

- `go build ./...` — clean.
- `go test ./...` — exit 0, unpiped. 380 `--- PASS` lines (`grep -c --
  "--- PASS"`, unanchored) vs 367 before this scenario — delta +13,
  matching the 13 new tests exactly (12 scaffold + 1 cli). 0 `--- SKIP`, 0
  `--- FAIL`.
- `go test -race ./internal/scaffold/... ./internal/cli/...` — clean.
- `golangci-lint run ./...` — 0 issues.
- `go doc ./internal/scaffold ErrMissingStateHeading` and
  `go doc ./internal/scaffold` — read correctly, sentinel enumeration
  includes both `ErrOverCap` and `ErrMissingStateHeading`.

## Binding decisions

- Presence-only, all four required, any order, empty sections valid —
  `markdown.Section(...).found == false` is the trigger, identical to
  SCENARIO-14's read-side rule. Four pre-existing `cli/finish_test.go`
  fixtures with empty sections were already load-bearing on this.
- Order is deliberately unenforced — `assemble.stateSections` reads by
  name, so an order rule has no consumer. `Ordered()` only decides which
  missing heading is named first.
  `Test_accepts_a_state_body_whose_headings_are_out_of_configured_order` is
  the recorded decision; a later scenario must not quietly tighten this.
- One refusal line naming the first missing heading — Finish stops at the
  first fault, unlike 14's degrade path.
- `checkArgumentHeadings` sits after `checkArgumentFence`, ahead of the
  specification read and `(refinish).verdict()`. This narrows R11: an
  identical re-finish is a true no-op only when the bytes also carry the
  four headings.
- Recorded, not fixed: `markdown.Section(body, "")` returns `found ==
  true` (matches the first blank line) — a repository configuring an empty
  state heading would pass this check silently. Belongs to the unowned
  heading-value-validation debt.

## Left unbuilt / traps / debts — see rewritten STATE.md
