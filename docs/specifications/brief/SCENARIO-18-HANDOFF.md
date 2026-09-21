# SCENARIO-18 Handoff: An over-cap state body is refused and nothing lands

## What landed

`(*scaffold.Server).Finish` now checks the `state` argument against
`cfg.StateCapLines` (default 80, yaml `state-cap-lines`), reusing 17's
generic `checkArgumentCap(body, source, label, limit)` helper verbatim with
`StateSource`, `"state"` and `cfg.StateCapLines`. `cfg.StateCapLines`, three
`grep` hits and no consumer before this scenario, is now read.

The check sits in `Finish`'s "cap band": immediately after the handoff cap
check, immediately before `checkArgumentFence(state, StateSource, "state")`.
No production file outside `internal/scaffold/finish.go` changed —
`internal/cli/finish.go`'s `StateSource`-to-real-path `switch` branch
predates this scenario and needed nothing.

## Tests added (8, scaffold + cli)

- `Test_refuses_a_state_body_one_line_over_the_configured_cap` — `Equal` on
  the whole `Error()` string, `Path == StateSource`, `Line == 0`.
- `Test_accepts_a_state_body_of_exactly_the_configured_cap` (+ no-trailing-
  newline twin) — boundary is `count > cap`.
- `Test_a_refused_over_cap_state_body_leaves_every_file_byte_identical` —
  paired snapshot + mtime probes, control arms cited from
  `finish_idempotent_test.go`, not duplicated.
- `Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps` —
  both bodies over their own caps; refusal names the handoff. Vacuous
  without mutation (d).
- `Test_reports_the_state_cap_before_the_state_s_unclosed_fence` — a state
  body both over cap and opening an unclosed fence reports the cap, not
  `ErrUnterminatedFence`.
- `Test_an_over_cap_state_body_on_a_done_step_reports_the_cap_not_the_re_finish_refusal`
  — cap band pre-empts `(refinish).verdict()`.
- `Test_refuses_a_state_body_over_the_cap_and_names_the_state_path` (cli) —
  through `cli.Run`, `Equal` on the whole stderr line including
  `(no files changed)`.

## Fold-ins done (bounded, as authorised)

- Renamed `overCapHandoff` → `bodyOfLines` (now used for both bodies) and
  made it emit genuinely distinct lines (`"line %d"` instead of repeated
  `"line"`), matching what its comment already claimed.
- Stripped the `(SCENARIO-17)` narrative markers from the two `Finish`
  doc-comment sentences edited for this scenario; added no `(SCENARIO-18)`
  marker. No wider sweep of `finish.go` or any other file — in particular
  `errors.go`'s pre-existing `SCENARIO-17/18` citation in `ErrOverCap`'s doc
  comment was left untouched, out of this scenario's authorised scope.

## Mutation verification (Step 11), each stashed and restored byte-identical

- **(a) delete the state cap call** → red: Steps 3, 4, 7, 9, 10 (5 tests).
  Green: Step 6 (both at-cap tests), Step 8, every 17-era handoff test.
  Exact match to prediction.
- **(b) `checkArgumentCap`'s `n <= limit` → `n < limit`** → red: all four
  at-cap tests (2 handoff + 2 state). Green: every over-cap test (handoff
  and state, scaffold and cli). Exact match — this is the one guard the two
  caps share.
- **(c) move the state cap check below `checkArgumentFence`** → red: Step 9
  only (`Test_reports_the_state_cap_before_the_state_s_unclosed_fence`).
  Exact match.
- **(d) swap the handoff and state cap call sites** → red: Step 8 only
  (`Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps`).
  Exact match — this is the mutation that proves Step 8 is not vacuous.

No mutation touched a call site's `limit` argument, per the plan's warning:
Steps 3/4 assert the whole `Error()` string, which embeds the limit.

## Verification

- `go build ./...` — clean.
- `go test ./...` — exit 0, unpiped. 367 `--- PASS` lines (`grep -c -- "---
  PASS"`, unanchored) vs 359 before this scenario — delta +8, matching the
  8 new tests exactly. 0 `--- SKIP`, 0 `--- FAIL`.
- `go test -race ./internal/scaffold/... ./internal/cli/...` — clean.
- `golangci-lint run ./...` — 0 issues.
- `go doc ./internal/scaffold` — `Finish`'s doc comment reads correctly,
  no scenario-id markers.

## Binding decisions

- Both caps live in one adjacent cap band: handoff, then state, then
  `checkArgumentFence(state, …)`. Three orderings depend on this, each
  pinned by its own mutation-verified test (see above).
- `cfg.StateCapLines` is now consumed via `checkArgumentCap` and
  `ErrOverCap` — the same sentinel 17 introduced, `RefusalError.Path`
  (`StateSource`) is what tells the two caps apart.
- `fixtureConfig.StateCapLines = 20` — above the 16-line `newStateBody`
  maximum, distinct from `HandoffCapLines = 10` and the 80-line default.
  SCENARIO-19 must keep it above whatever state bodies it adds.

## Left unbuilt / traps / debts — see rewritten STATE.md

Carried into `STATE.md` rather than duplicated here; SCENARIO-19 must place
its required-heading check **after** the fence check, not inside the cap
band — a body that fails the fence scan cannot have its headings read.
