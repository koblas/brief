---
id: SCENARIO-03
status: done
depends-on: []
---

# SCENARIO-03: start --json success carries the common header

## Scenario

```gherkin
Scenario: SCENARIO-03 start --json success carries the common header
  When I run "brief start demo --json" on a feature with an open step
  Then the document has schema, command "start", ok true, exit_code 0, plus every existing brief field unchanged
```

Rules in force: R1 (one document on stdout, **stderr empty**, exit codes unchanged), R2
(header first, flat payload, no `data` wrapper), R6 (paths absolute in JSON).

## User-visible contract

- `brief start demo --json` (or `--json demo`), open step → stdout: one line
  `{"schema":1,"command":"start","ok":true,"exit_code":0,"done":…,"open":…,"step":{…},"inherited":[…],"shortfalls":…}`;
  stderr **empty**; exit 0.
- Same, feature complete → identical shape with `"step":null`, `done` > 0; stderr empty; exit 0.
- Same, no step files yet → `"step":null`, `"done":0`, `"open":0`; stderr empty; exit 0.
- Same, optional convention missing → the shortfall is carried only in `shortfalls[]`
  (`path` absolute, `detail`, `fix`); stderr empty; exit 0.
- Text mode (no `--json`): byte-for-byte unchanged — shortfall lines and the
  complete / no-step-files notices stay on stderr.
- Refusals / usage errors under `--json`: unchanged (SCENARIO-01/02).

## Decision: what happens to start's stderr lines under --json

All three stderr writes (per-shortfall line, "feature is complete", "no step files yet") are
**suppressed** in JSON mode; **no new field is added**. Justification:
- R1 requires stderr empty in `--json` mode; exit 0 is a success, not an error document.
- Shortfalls are already payload: `Brief.Shortfalls` marshals as `shortfalls` with an
  absolute `path` (R6 satisfied without change), `detail`, `fix` — the stderr line is a
  flattening of exactly those three fields.
- The two notices are already payload-derivable: `step: null` is the discriminator (R14,
  product verdict), and `done`/`open` separate complete (`done+open > 0`) from empty
  (`0/0`). The notices' only extra content — the feature directory and the
  `brief new step` hint — is human copy, not data. Adding a `notices` field would duplicate
  the discriminator in English, which R8's spirit (scripts never parse English) rejects.

Implementation shape: runStart branches on JSON mode **before** any stderr write and returns
after writing the document; the text path below it is untouched. One branch, not three
per-write guards.

Document shape: a cli-local `startDocument` struct embedding `jsonHeader` first, then
`assemble.Brief` anonymously **by value**, encoded with `writeJSONDocument`. Go's
embedded-struct flattening yields the flat payload with header keys first; `Brief` has no
field colliding with a reserved name. The header comes from a new `reporter` method in
`json.go` (`successHeader`), so `json.go` stays the one place any document header is built
(it already owns `usageError`'s and `refusal`'s). A generic "write header + any payload"
method is not possible: Go cannot embed a type parameter, so each command owns its
`<cmd>Document` struct and asks the reporter for the header.

`assemble.RenderJSON` loses its only production caller but is **left in place** with its
tests — removing it is a refactor outside this scenario (listed under Left unbuilt).

## Implementation Plan

Strict red first: Steps 1-3 must be observed failing before Step 4.

- [x] Step 1: `internal/cli/start_test.go` `Test_start_json_success_document_golden` — exact-bytes golden for the open-step fixture with no shortfalls (`newStartFixture(t, "open")`): pins header first, then `done, open, step, inherited, shortfalls` key order, `"shortfalls":null`; stderr empty, exit nil (red)
- [x] Step 2: `internal/cli/start_test.go` — update `Test_start_prints_the_brief_as_json_when_asked` to decode the header too (`schema` 1, `command` "start", `ok` true, `exit_code` 0) alongside the existing Brief fields (red)
- [x] Step 3: `internal/cli/start_test.go` — flip the three stderr claims to R1: `Test_start_json_emits_a_null_step_for_a_completed_feature`, `Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files`, `Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing` now assert stderr **empty** and the header present; the shortfall test keeps its `shortfalls[0].path` assertion and adds that the path is absolute; rewrite each test's doc comment to state the new rule (red)
- [x] Step 4: `internal/cli/json.go` `(reporter) successHeader` — returns `newJSONHeader(commandName(r.cmd), 0)`; doc comment states it is the header every success document embeds (new)
- [x] Step 5: `internal/cli/start.go` `startDocument` — `jsonHeader` then `assemble.Brief`, both embedded by value; doc comment states the flat-payload rule (new)
- [x] Step 6: `internal/cli/start.go` `runStart` — JSON branch placed before the shortfall loop and the notice block: build `startDocument` from `out.successHeader()` and the brief, write via `writeJSONDocument(out.stdout, …)`, return; text path unchanged; replace the stale "--json always writes a document … stderr notice above" comment with the current rule (green)
- [x] Step 7: `internal/cli/start_test.go` — confirm the text-mode controls still pass unchanged: `Test_start_says_the_feature_is_complete_when_every_step_is_done`, `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature`, `Test_start_names_an_absent_acceptance_heading_and_still_prints_the_brief`, `Test_start_without_json_is_byte_identical_to_the_text_brief` (control arms for Step 9's mutations — no edit expected)
- [x] Step 8: `internal/assemble/brief.go` `Shortfall` doc — "cli/start.go writes one stderr line per entry" becomes text-mode-only; leave every "RenderJSON's wire contract" phrasing alone (RenderJSON still exists) (update)
- [x] Step 9: mutation-verify, each stashed per `agent-briefs.md`, one at a time, naming the test that reddens:
  (a) drop `jsonHeader` from `startDocument` → Step 1 golden and Step 2 red;
  (b) move the JSON branch after the shortfall loop → shortfall test (Step 3) red, text control still green;
  (c) move it after the notice block → completed-feature and no-step-files tests red;
  (d) `successHeader` passes a non-zero exit code to `newJSONHeader` → golden red (proves `ok`/`exit_code` are pinned, not copied)
- [x] Step 10: `go build ./...`, `go test ./...` (unpiped; report count + delta — expected +1 test (the golden), three tests' assertions flipped, zero deletions), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-03 done in specification.md, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Success documents are a per-command `<cmd>Document` struct embedding `jsonHeader` (from `reporter.successHeader()` in `json.go`) first, then the payload by value, written with `writeJSONDocument` — R2 flat payload; `json.go` stays the only place headers are built. SCENARIO-07/09/10/11/12/13 follow this shape.
- In `--json` mode a successful command writes **nothing** to stderr (R1): what a stderr line said as data must already be a payload field; pure human copy (hints, directory names) is dropped, never moved into a `notices` field.
- The JSON branch in a command runs before any stderr write and returns — one branch per command, not per-write guards.
- `start --json` discriminators: `step:null` + `done`/`open` (complete vs no step files). No extra field.
- `assemble.Brief`'s json tags remain start's payload contract, unchanged.

**Left unbuilt** — named so nobody assumes it exists:
- `startLong` help prose still says the notices go to stderr unconditionally — SCENARIO-14 owns help text including the `--json` paragraph.
- Relative paths in text-mode start shortfall/notice lines (R6 text side) — SCENARIO-04 / SCENARIO-05 touch those lines.
- start's pflag `--json` stays registered for the help row only (unchanged, per STATE.md).
- `assemble.RenderJSON` — exported, tested, no production caller after this scenario (it emits a header-less document). Removing it is a refactor, unowned by any scenario.

**Traps** — things that look right and are not:
- Unmarshalling into `assemble.Brief` still succeeds on the new document (header keys are ignored) — so decoding alone proves nothing about the header; assert header fields explicitly or use the golden.
- Embedding `assemble.Brief` by value flattens; embedding it as `*assemble.Brief` also flattens but a nil pointer silently drops all payload keys — embed by value.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future payload struct would be silently resolved by encoding/json's depth rules (both dropped at equal depth) — keep payload field names disjoint.
- Checking stderr empty in the completed-feature test without a text-mode control arm is vacuous; the existing text tests in Step 6 are that control and must stay.
