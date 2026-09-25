---
id: SCENARIO-09
status: done
---

# SCENARIO-09: A refused finish and an identical re-finish report no drops

## Scenario

```gherkin
Scenario: SCENARIO-09 — A refused finish and an identical re-finish report no drops
  Given a finish that would drop entries but is refused
  Then no row is printed, the exit code is 1, and the error document has no dropped_entries
  And given an identical re-finish of a done step
  Then no row is printed and "dropped_entries" is []
```

## Context

D5 (`docs/specifications/dropped-entries/specification.md`): "Any refusal or usage error
prints no rows and its JSON error document has no `dropped_entries`." This is enforced at two
independent points, proved separately below rather than as one fact:

1. `scaffold.FinishFS`'s refinish switch returns for every refusal branch before its
   `droppedEntries(...)` call runs, so a refusal's `FinishResult` carries no drop data at all —
   not "computed then discarded", never computed.
2. `cli.runFinish`'s `srv.Finish` error branch returns before either the text-mode row loop or
   the `finishDocument` JSON build ever runs, so even if (1) ever changed, the CLI would still
   need its own guard to stay silent.

The only existing proof, `Test_finish_json_refusal_is_unchanged_mem`, refuses on
`ErrNoSuchStep` — before `state` is even read as `stateBytes` — so it pins the error
document's key *shape* only; it cannot distinguish "never computed" from "computed and
suppressed". This scenario instead uses R11's state-divergence refusal
(`refinishStateDiverged`), one branch away from point 1's `droppedEntries` call, paired with a
control arm that differs in exactly one variable — the recorded handoff file's presence
(`FinishFS`'s row-2 exemption: a done step whose handoff file is missing or unreadable takes
`refinishWrite` instead) — to prove the same old/new state pair really is drop-bearing when
the finish takes the write path.

**Shared fixture (Steps 1-4).** One "done" step, `SCENARIO-01`, whose recorded state is an
old/new pair where the new body omits one tagged entry the old body has. Build it as: write
both `--state` inputs (old-pair path and new-pair path) into `tree` before taking
`mem := tree.mem()`; finish once with the old-pair state to record it; take `mem` once and
reuse it for every subsequent call via `runFinishArgsMem` — a second `tree.mem()` call takes a
fresh copy of `tree`'s original entries and would silently discard the first finish's writes,
turning an intended re-finish into a first finish. Keep the handoff bytes identical across
every call in Steps 2-4 — only the *state* argument and the handoff *file's presence* vary,
never the handoff *bytes* — so the refusal each test hits is provably
`refinishStateDiverged`, never `refinishHandoffDiverged`.

No new production symbol: `finishDroppedEntries`, `finishDocument.DroppedEntries`,
`dropDetail`, `dropCountSuffix` all already exist (SCENARIO-01/04); `finishDroppedEntries`
already returns a sized non-nil slice for a nil/empty input, so it JSON-encodes `[]`, never
`null`, on every path including a hypothetically-leaked refusal. `### Green` below is empty on
that basis; if a Red step is unexpectedly red, stop and report the specific assertion rather
than backfilling a guard silently.

## Implementation Plan

### Red

Every item below is expected green on arrival per *Context*; report honestly if one is not.

- [x] Step 1: `internal/cli/finish_dropped_internal_test.go` — fixture helper building the
  shared done-step drop-bearing pair described in *Context* (helper, not a test)
- [x] Step 2: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_accepts_a_drop_bearing_re_finish_when_the_handoff_file_is_missing_mem` — control
  arm (handoff file removed); fails if the omitted entry's WARN row does not print
- [x] Step 3: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_refuses_a_state_diverged_re_finish_mem` — refusal arm
  (handoff file present); fails if the error is not `ErrAlreadyFinished` naming the state file
  with a "state differs" problem
- [x] Step 4: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_json_state_diverged_refusal_has_no_dropped_entries_key_mem` — Step 3's refusal
  with `--json`; fails if the raw stdout bytes contain the `dropped_entries` substring, or if
  the decoded error's `path` is not the state file
- [x] Step 5: `internal/scaffold/finish_dropped_test.go`
  `Test_finish_state_diverged_refusal_returns_no_dropped_entries` — two-row table on the same
  drop-bearing pair: handoff file present yields the refusal with `FinishResult.Dropped` empty;
  handoff file missing yields success with `Dropped` holding the entry; fails if the refusal
  row's `Dropped` is not empty
- [x] Step 6: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_identical_re_finish_of_a_drop_bearing_step_reports_the_noop_line_mem` — text-mode R11:
  first call (open step, drop-bearing pair) must print the WARN row; identical second call
  (`refinishNoop`) fails if stderr is not the existing "already done with identical inputs;
  nothing written" line
- [x] Step 7: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_json_identical_re_finish_reports_no_dropped_entries_mem` — Step 6's calls with
  `--json`; fails if the second call's document is not exactly `"changed":false`,
  `"modified":[]`, `"dropped_entries":[]` (an exact-document match, so a coincidental write
  that also nets zero drops cannot pass as a no-op)

### Green

None planned — see *Context*.

### Sweep

- [x] Step 8: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 9: full verification per `.claude/rules/agent-briefs.md`. Two guards, verified
  individually, each restored and diffed byte-identical before the next:
  - Guard A (`scaffold.FinishFS`'s `refinishStateDiverged` case): compute
    `droppedEntries(stateBytes, state, s.cfg.StateHeadings)` and attach it to the returned
    `FinishResult` instead of the zero value — must redden Step 5's refusal row. Moving the
    existing `droppedEntries` call earlier *without* attaching its result to a returned
    `FinishResult` is not a valid mutation here: the value would be computed and immediately
    discarded, reddening nothing — do not substitute that for Guard A.
  - Guard B (`cli.runFinish`'s `srv.Finish` error branch): disable it (e.g. append `&& false`
    to its `if err != nil` condition) so a refusal falls through to render the success-shaped
    output — must redden Step 3's and Step 4's exit-code assertions (both become 0) and Step
    4's raw-bytes assertion (stdout gains the `dropped_entries` key).

## Handoff

**STATE.md**: rewrite it with these concrete cuts — it is already near the comfortable size
for a rolling state file:
- Delete the SCENARIO-09 line under "Left unbuilt" (this scenario closes it).
- Merge the "unreachable through `brief finish`/`cli.Run`" binding decision (the
  `scannableHeadings`/`config.Resolve` paragraph) with the `checkArgumentHeadings` trap — they
  state the same fact twice, once as a decision and once as a trap.
- Compress the `\r`-tokenization binding decision to its rule (`normalizeEntryText`'s
  `ReplaceAll(raw, "\r", "")` only matters mid-word); drop the SCENARIO-08 fixture narrative —
  that belongs to that scenario's own audit trail, not STATE.md.

**Binding decisions** — a later scenario must not contradict these without saying so:
- D5 is enforced at two independent points, not one — `FinishFS`'s refinish switch (never
  computes `dropped` before a refusal branch returns) and `runFinish`'s `srv.Finish` error
  branch (never renders `res` on a non-nil `err`). A change to either alone is caught by its
  own test — Step 5 for the former, Steps 3/4 for the latter — neither substitutes for the
  other.

**Left unbuilt**:
- `finishLong`'s drop-reporting prose paragraph — still SCENARIO-10, untouched here.

**Traps**:
- `refinishNoop` never calls `droppedEntries` at all — it returns a literal
  `Dropped: []DroppedEntry{}` (read directly from `internal/scaffold/finish.go`). Steps 6/7
  exercise the identical-re-finish path and its stderr line; Steps 3/4/5 prove the
  state-diverged refusal's own contract. They are not redundant — do not drop either pair.
- `Test_finish_json_refusal_is_unchanged_mem` (existing) stays valid but stays weak — it
  refuses before `state` is even read as `stateBytes`, so it can never distinguish "never
  computed" from "computed and suppressed". Do not delete it; do not treat it as covering D5.
- `tree.mem()` takes a fresh copy of `tree`'s entries on every call — a second call after a
  finish has already written into the first `mem` silently discards those writes and turns an
  intended re-finish into a first finish. Write every `--state`/`--handoff` input into `tree`
  before taking `mem` once.
- A finish whose new-state file was never written into `mem` refuses earlier, inside
  `readSource`, before `srv.Finish` even runs — Guard B would not redden that variant. Steps
  3/4 must go through `srv.Finish`'s own refusal, not a missing-input usage error.
