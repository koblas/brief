# SCENARIO-11: finish reports what it wrote and what's next

## Scenario

Scenario: SCENARIO-11 finish reports what it wrote and what's next
  When I finish a step
  Then stderr names the step, the files written, and the next step with its start command
  And a re-finish with identical inputs says "already done with identical inputs; nothing written"
  And "--json" gives {feature, step, changed, handoff_path, state_path, next}

## User-visible contract (pinned)

Command: `brief finish <feature> <step> --handoff <path> --state <path> [--json]`. Refusal,
usage-error and write-failure paths and their exit codes are unchanged (S02). This scenario
changes only the success output (exit 0).

Text mode: stdout is empty. stderr is exactly one line:
- wrote, next exists: `brief finish: <feature> <step> done; wrote <handoff rel>, replaced <state rel>; next: <id> — run 'brief start <feature>'`
- wrote, nothing open: `brief finish: <feature> <step> done; wrote <handoff rel>, replaced <state rel>; <feature> is complete`
- R11 no-op re-finish: `brief finish: <feature> <step> already done with identical inputs; nothing written`
  (no next clause; the no-op writes nothing, so it names no files)

`<handoff rel>` is the step's handoff file that was written. `<state rel>` is the feature's
state file that was replaced, **not** the `--state` input. Both are relative via
`displayPath(wd, p)`.

`--json`: stdout is one document, stderr is empty:
`{<jsonHeader>, "feature", "step", "changed", "handoff_path", "state_path", "next"}`.
Paths are absolute, passed through verbatim from scaffold. `changed` is `false` only on the R11
no-op. `next` is a bare id string or `null`, not a `{id,title,path}` object like status's.
On a refusal or a write-path failure, the header's `files_changed` reports what actually
happened on disk (R3): `false` when nothing was written, `true` when at least one of finish's
four writes (handoff, state, step file, specification) landed before the one that failed, via
`errors.Is(err, scaffold.ErrPartialWrite)`.

Definition of `next`: the step `brief start <feature>` would brief after this finish, which is
the **lowest-numbered step file (by `pattern.Number`) whose frontmatter status is not "done"**,
with the finished step counted as done. `depends-on` is ignored for ordering, so a blocked step
is still `next` (this matches `assemble.Start` at `assemble.go` and `assemble.Status`'s NEXT).
A lower-numbered open step than the one just finished is also `next`. A sibling whose
frontmatter cannot be read or parsed counts as not done, following the `siblingFrontmatter`
precedent.

## Design decision: where `next` is computed

`next` is computed **inside `scaffold.Finish`**, during the validation phase before the first
write, and returned in a new `scaffold.FinishResult`. cli does not call `assemble` afterwards:
- `assemble.Start` refuses (`ErrMalformedFeature`) on a next step missing `id:` or its
  checklist heading. If cli called it after a successful four-file write, it would render a
  completed write as a refusal with the `(no files changed)` tail, which is false.
- `assemble.Status` walks every feature to answer a question about one.
- scaffold already has `root`, `pattern`, `siblingFrontmatter` and `pattern.ID(n)`, and S10
  keeps step ids inside scaffold.
The rule is duplicated between assemble and scaffold. That drift is pinned by a cli-level
agreement test (Step 13), following the `check_drift_test.go` precedent.

## Implementation Plan

- [x] Step 1: `internal/cli/finish_test.go` — first run `grep -n 'stderr' internal/cli/finish_test.go` and `grep -rn '"finish"' internal/cli/*_test.go`, and fix **every** success-path stderr assertion, including `Empty` ones, not only the four exact-equal lines (~72, ~85, ~112, ~554). Each becomes the pinned "is complete" line, because `newFinishCLIFixture` has one step. Rename the re-finish test at ~97 to `Test_finishing_an_already_finished_step_a_second_time_reports_the_no_op_and_succeeds` and assert the R11 no-op line on the second run. Re-derive the rationale in the comment block at ~225-233 rather than only swapping the quoted string: with the refusal deleted, `verdict()` falls through to `refinishWrite` and prints the `done; wrote …` line, so the whole-string assertion is still required (red)
- [x] Step 2: `internal/cli/finish_test.go` `Test_finish_names_the_next_open_step_and_its_start_command` — use a multi-step fixture helper (e.g. `newFinishCLIFixtureWithSteps`), finish SCENARIO-01 with SCENARIO-02 open, and assert the exact stderr line with the `next:` clause and empty stdout (red)
- [x] Step 3: `internal/cli/finish_test.go` `Test_finish_names_a_blocked_step_as_next` — SCENARIO-02 depends on an unfinished SCENARIO-03. Finishing SCENARIO-01 gives `next: SCENARIO-02` (red)
- [x] Step 4: `internal/cli/finish_test.go` `Test_finish_names_a_lower_numbered_open_step_as_next` — finish SCENARIO-02 while SCENARIO-01 is open, which gives `next: SCENARIO-01` (red)
- [x] Step 5: `internal/scaffold/finish.go` `FinishResult` — new exported type `{Feature, Step string; Changed bool; HandoffPath, StatePath string; Next string}` with a doc comment covering: absolute paths, `StatePath` is the replaced state file, `Next` is `""` when nothing is open, and the `next` definition above (new)
- [x] Step 6: `internal/scaffold/finish.go` `(*Server).Finish` — change the signature to `(FinishResult, error)`. Add an unexported next-open-step helper that takes the `[]os.DirEntry` that `findStepFile` already read (have `findStepFile` return them) and the finished step's number to exclude, and returns a plain id `string` with **no error**. It must not do a second `ReadDir`, which would add a new failure mode inside R14a's pinned refusal order. Call it once in the validation phase, before the `r.verdict()` switch. Return the result with `Changed: false` on `refinishNoop` and `Changed: true` after the fourth write. Refusals and write failures return a zero `FinishResult`. Update the `Finish` doc comment and the `doc.go` paragraph about the return value (green)
- [x] Step 7: every `Finish(` call site in `internal/scaffold/*_test.go` and `internal/cli/check_drift_test.go` (~90 hits) — mechanical switch to the two-value return, with no assertion changes. Do this in the same commit as Step 6 so the build never breaks (update)
- [x] Step 8: `internal/scaffold/finish_test.go` — Server-level tests for `FinishResult`: fields on a first finish (`Changed` true, absolute `HandoffPath`/`StatePath` built from the fixture root, `Next`), `Changed` false and `Next` still computed on the R11 no-op, `Next` `""` when every other step is done, and an unparseable sibling counted as not done and named as `Next` (red→green)
- [x] Step 9: `internal/cli/finish.go` `runFinish` — render the three pinned text lines to `out.stderr` through `displayPath(wd, …)`. Do not add a private `filepath.Rel`. stdout stays empty (green for Steps 1-4)
- [x] Step 10: `internal/cli/finish_json_test.go` `Test_finish_json_is_one_exact_document` — exact-bytes golden (`assert.Equal`, not `JSONEq`) for key order, with stderr empty. Build the paths from the same `wd` passed to `cli.Run`. Add a decode-level table covering `next` as a string, `next` as `null` (not absent), and `changed:false` on the R11 no-op (red)
- [x] Step 11: `internal/cli/json.go` or `internal/cli/finish.go` `finishDocument{jsonHeader; Feature, Step string; Changed bool; HandoffPath, StatePath string; Next *string}` — json tags `handoff_path`, `state_path` and `next` (no omitempty), with `jsonHeader` embedded first by value. In `runFinish`, put the `--json` branch **before** any stderr write and use `out.successHeader()` (green)
- [x] Step 12: `internal/cli/finish_json_test.go` `Test_finish_json_refusal_is_unchanged` — confirm that a refusal under `--json` still renders the R3 error document, not a `finishDocument`. This may already pass. If so, say so and give the reason (green-on-arrival allowed)
- [x] Step 13: `internal/cli/finish_start_agreement_test.go` `Test_finish_next_agrees_with_start` (not `check_drift_test.go`, which covers check-vs-finish drift) — on the blocked-next tree and on the lower-numbered tree, finish a step with `--json`, then run `brief start <feature> --json` through `cli.Run`, and assert that finish's `next` equals start's briefed step id. Capture both values; do not use literals (new)
- [x] Step 14: mutation verification. Run each mutation separately, stash it, then restore it and prove the file is byte-identical:
  (a) make the next-open-step helper return `""` → Steps 2/3/4 and 13 go red, not the whole suite;
  (b) set `Changed: true` on the `refinishNoop` return → the R11 no-op tests (Step 1 re-finish, Step 8, Step 10 table) go red;
  (c) make the helper skip steps with unmet depends-on → Step 3 and the Step 13 blocked arm go red;
  (d) remove the exclusion of the finished step from the scan → Step 2 goes red (next would name the step just finished);
  (e) move the `--json` branch after the stderr write → Step 10's stderr-empty assertion goes red.
- [x] Step 15: `go build ./...`, `go test ./...` (unpiped, with the test count and delta), `go test -race ./internal/cli/... ./internal/scaffold/...`, `golangci-lint run ./...`, then mark SCENARIO-11 done in specification.md and rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `scaffold.Finish` returns `(FinishResult, error)` with `FinishResult{Feature, Step, Changed, HandoffPath, StatePath, Next}`. It is a separate type from `scaffold.Result`, whose `Path`/`Created` contract S10 pinned for `new`.
- `next` = the lowest-numbered step file whose status is not done, counting the finished step as done and ignoring `depends-on` (a blocked step can be next). This is `assemble.Start`'s rule, duplicated in scaffold because the two packages cannot import each other. `Test_finish_next_agrees_with_start` is the pin, so changing either side's rule reddens it.
- `next` is computed in the validation phase, before any write, so no error path exists after the writes land. cli never calls `assemble` after `Finish`: Start refuses on a malformed next step and would misreport a completed write as `(no files changed)`.
- Text copy (stderr, stdout empty): `brief finish: <f> <s> done; wrote <handoff rel>, replaced <state rel>; next: <id> — run 'brief start <f>'` / `…; <f> is complete` / `brief finish: <f> <s> already done with identical inputs; nothing written`. S14's help text must quote these, not new wording.
- `finish --json`: `{header, feature, step, changed, handoff_path, state_path, next}`. `next` is a nullable scalar id (`*string`, no omitempty). Paths are absolute and verbatim. stderr is empty. `changed:false` only on the R11 no-op.
- `state_path`/`<state rel>` is the replaced feature state file, never the `--state` input source.
- The text no-op line carries **no** next clause, but the JSON no-op document **does** carry `next`. This asymmetry is deliberate: product-vision byte-pinned the re-finish string and fixed the JSON shape. Do not harmonize it.
- The next helper takes `findStepFile`'s dir entries and returns no error. Adding a `ReadDir` or an error to it perturbs R14a's refusal order.

**Left unbuilt** — named so nobody assumes it exists:
- A shared platform helper for the "next open step" rule (for example in `internal/platform/stepfile`) is unbuilt. The rule lives in assemble and scaffold, and only the drift test ties them. Nobody owns it yet.
- A `blocked` flag in finish's output does not exist. The ruled JSON shape has none.
- `--version`/`help`/`completion` JSON documents are unbuilt (S12/S13). The `--json` help row is S14.

**Traps** — things that look right and are not:
- `Changed`'s zero value is `false`. A write path that forgets to set it silently reports a no-op. Assert `Changed: true` on first finish, not just `false` on the no-op.
- `os.ReadDir` order is filename order, not step-number order. Pick the minimum by `pattern.Number`, as `assemble.readSteps` sorts.
- A sibling with unparseable frontmatter is named as `next`, but `brief start` then refuses the whole feature (`readSteps` fails on any bad step). That is truthful ("start stops here") but the two do not agree on that tree. Leave it out of the drift test's trees.
- The R11 no-op path returns early, before the writes. It must return the populated result too, not a zero `FinishResult`.
- `newFinishCLIFixture` has one step, so every existing success assertion becomes the "is complete" variant, not the `next:` variant.
- Adding `FinishResult{}, ` to every refusal return and the `next` computation pushed `Finish` over golangci-lint's `maintidx` budget (19, threshold 20; clean at HEAD). Fixed by extracting the four ordered writes into `applyFinishWrites` — a future addition to `Finish` should extract another helper rather than inline more branches into it.
