---
id: SCENARIO-15
status: done
depends-on: []
---

# SCENARIO-15: Start emits a structured payload on request

## Scenario

```gherkin
Scenario: SCENARIO-15 Start emits a structured payload on request  [orig: new, OQ4]
  Given a feature with an open step
  When I start it asking for JSON
  Then I get the same brief as structured fields
  Given a feature whose steps are all done
  When I start it asking for JSON
  Then the step field is null, which is how a structured caller detects completion
```

## Scope and surface decisions

**Commands covered: `start` only.** The Gherkin names only `start`. *Decisions taken* 3 in
`specification.md` settles it in the same direction — "`--json` on `start` only, for now …
`status`'s four-field table already *is* the machine format; a second stable format on an
already-parseable surface is a schema to keep stable forever for nothing" — and open question
8 defers `next`/`show`/`state get`/`handoff`. The command table carries `--json` on the
`start` row and on no other row. **`status --json` is deferred, not forgotten; do not build
it.**

**Flag: a boolean `--json`, not `--format=json`.** The command table's literal text is
`--json`. There is no existing `--format` in `internal/cli`, and a one-value enum flag is a
worse contract than a boolean.

**Argument order: `--json` may precede or follow the feature.** `flag.FlagSet.Parse` stops at
the first non-flag argument, so `brief start demo --json` would otherwise be "too many
arguments" (exit 2). `runFinish` already solved this with `splitLeadingPositionals`;
`runStart` reuses it. This is behaviour-preserving for today's tests — `{"start","a","b"}`
still yields two leftover positionals, `{"start","--nope"}` still yields a parse error — so
expect those existing tests to be **green on arrival**.

**The payload is `json.Marshal` of the `Brief`, with no normalization step.** A nil pointer
and a nil slice both marshal as `null`. That is the uniform rule: `"step": null` (R14's named
discriminator), and `"shortfalls": null` when there are none. SCENARIO-14 kept
`Brief.Shortfalls` nil rather than `[]`, and SCENARIO-09/10 kept `Status` returning `nil` for
zero features with the note "(15's `--json` needs `null`)" — prior scenarios shaped these
values for this wire format. Emitting `[]` would make `"step": null` the exception rather than
the rule and would add a place the Go value and the wire value can drift.

**Schema — every field always present, `omitempty` on nothing:**

| JSON field | Go | Notes |
| --- | --- | --- |
| `done` | `Brief.Done` int | never `omitempty` — `0` is load-bearing |
| `open` | `Brief.Open` int | never `omitempty` — `0` is load-bearing |
| `step` | `*Step` | **object, or `null`** — the completion discriminator |
| `inherited` | `[]Section` | every configured state heading, in `cfg.StateHeadings.Ordered()` order; `null` if the slice is nil |
| `shortfalls` | `[]Shortfall` | `null` when none |
| `step.id`, `step.title` | string | |
| `step.acceptance`, `step.checklist` | `Section` value | always objects, never null |
| section `heading`, `body`, `found` | string, string, bool | `found` is the absent-heading vs empty-body discriminator (SCENARIO-14); dropping it would destroy it |
| shortfall `path`, `detail`, `fix` | string | `path` absolute, same value as the stderr line |

`done`/`open` must survive at zero because `"step": null` alone cannot tell a **complete**
feature (`2`/`0`) from a feature with **no step files yet** (`0`/`0`) — that is the standing
trap from SCENARIO-12. `omitempty` on either one deletes the distinction.

**JSON is strictly richer than the text render, deliberately.** `RenderText` omits a section
whose body is empty; `RenderJSON` emits **every** section, including `found: false` and empty
bodies. The two renderers are not interchangeable.

**Stdout purity.** Under `--json`, stdout is exactly one compact JSON document followed by one
`\n`, and nothing else. The SCENARIO-14 shortfall lines and the SCENARIO-12 complete /
no-step-files notices still go to **stderr**, unchanged, exit still 0. Encode into a
`bytes.Buffer` (`json.NewEncoder`, `SetEscapeHTML(false)` so `<`/`&` in markdown bodies stay
readable, `Encode` supplies the trailing newline) and do a single `w.Write` — stdout is a
complete document or zero bytes, never a partial one.

**R14 tension, resolved by R14 itself.** R14's first half says nothing-to-return is "empty
stdout, one line on stderr, exit 0". Under `--json` a complete feature's stdout is **not**
empty — it carries the document with `"step": null`. R14's own next sentence authorizes
exactly this: "`--json` gives the structured caller the same discriminator as `"step":
null`." So `brief start <f> --json` always emits a document on any exit-0 path.

**Exit codes are unchanged from 12/13/14.** `--json` alters no exit code:

| Case | Exit | stdout | stderr |
| --- | --- | --- | --- |
| open step | 0 | payload, `"step"` an object | empty (or shortfall lines) |
| all steps done | 0 | payload, `"step": null`, `done`>0 | "feature is complete" line |
| no step files yet | 0 | payload, `"step": null`, `done`=`open`=0 | "no step files yet" line |
| missing optional convention | 0 | payload | one shortfall line each |
| malformed feature (13) | 1 | **zero bytes** | one R14a line, no `(no files changed)` |
| unknown feature | 1 | **zero bytes** | one R14a line |
| no feature / undefined flag | 2 | empty | usage line |

**A refusal under `--json` stays a plain R14a stderr line — no JSON error envelope.** R14a
fixes one refusal template for every command, and a script "tells refused from finished by
exit code alone"; a second, separately-versioned error schema is precisely the "schema to keep
stable forever for nothing" *Decisions taken* 3 rejects. `runStart` calls `Start` before it
renders anything, so a refusal never reaches the renderer — stdout stays at zero bytes by
construction.

**Out of scope for 15:** `status --json` (deferred, above); R13's output budget and truncation
(no owner; not in the Gherkin); 16–21's `finish` refusals; 22's `check`. No `cmd/brief`
change, no new `assemble.Server` method, no new dependency and therefore no `WithX` option, no
`Store` method — the data 15 serializes already exists on `Brief`.

## Implementation Plan

- [x] Step 1: `internal/assemble/render_test.go` `Test_RenderJSON_writes_every_field_of_a_brief_with_an_open_step` — unmarshal the output and compare the whole decoded value against the expected shape: `done`, `open`, `step.id`/`title`/`acceptance`/`checklist`, `inherited` in configured order (red)
- [x] Step 2: `internal/assemble/brief.go` — add `json:"…"` tags to `Brief`, `Step`, `Section` and `Shortfall`, no `omitempty` anywhere; extend the doc comments to state the wire contract, including that the lowercase key names are the contract (new)
- [x] Step 3: `internal/assemble/render.go` `RenderJSON` — buffered `json.NewEncoder` with `SetEscapeHTML(false)`, one `w.Write` of the finished document; doc comment says it always writes a document, including when `b.Step` is nil, unlike `RenderText` (green)
- [x] Step 4: `internal/assemble/render_test.go` `Test_RenderJSON_marshals_a_nil_step_as_null` — assert the **raw bytes** contain `"step":null`; control arm in the same test file asserts the open-step brief's bytes contain `"step":{` (unmarshalling collapses null-vs-absent, so this one is byte-level) (green on arrival)
- [x] Step 5: `internal/assemble/render_test.go` `Test_RenderJSON_keeps_the_zero_counts_a_caller_needs` — raw bytes carry `"done":0` and `"open":0` for a brief with no steps at all, so a caller can tell it from a completed feature (green on arrival)
- [x] Step 6: `internal/assemble/render_test.go` `Test_RenderJSON_marshals_absent_shortfalls_as_null` — raw bytes carry `"shortfalls":null` when the slice is nil, and an array when it is populated (green on arrival)
- [x] Step 7: `internal/assemble/render_test.go` `Test_RenderJSON_keeps_a_section_RenderText_omits` — assert the **raw bytes** of the absent-heading section carry `"found":false` (decoding collapses an omitted key into `false`, so this claim is byte-level too); the same brief through `RenderText` omits that section entirely (green on arrival)
- [x] Step 8: `internal/assemble/render_test.go` `Test_RenderJSON_leaves_angle_brackets_and_ampersands_unescaped` — a body containing `<` and `&` is not rewritten to `<`/`&` (green on arrival)
- [x] Step 9: `internal/cli/start_test.go` `Test_start_prints_the_brief_as_json_when_asked` — through `cli.Run` with `--json` against the existing `newStartFixture`: stdout unmarshals to the expected payload, stderr is empty, error is nil (red)
- [x] Step 10: `internal/cli/start.go` — register the `--json` bool flag and peel the feature argument with `splitLeadingPositionals` before `fs.Parse`. Required behaviour, not a required line position: the shortfall lines and the `Step == nil` "feature is complete" / "no step files yet" stderr notices are written exactly as today, and under `--json` the `Step == nil` branch **falls through to `assemble.RenderJSON` instead of returning `nil`**; the `assemble.RenderText` call stays gated on the flag being unset. Every exit-0 `--json` path writes exactly one document. Update `startUsage` to `brief start [--json] <feature>`, describing the flag and the `"step": null` discriminator (green)
- [x] Step 11: `internal/cli/start_test.go` `Test_start_json_emits_a_null_step_for_a_completed_feature` — exit 0, stdout raw bytes carry `"step":null` and a non-zero `"done"`, the SCENARIO-12 "feature is complete" line is still on stderr (green on arrival)
- [x] Step 12: `internal/cli/start_test.go` `Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files` — exit 0, stdout carries `"step":null` with `"done":0` and `"open":0`, the "no step files yet" line is still on stderr (green on arrival)
- [x] Step 13: `internal/cli/start_test.go` `Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing` — the SCENARIO-14 shortfall line is on stderr, stdout is one line, ends in `\n`, and unmarshals with the shortfall also present in the payload (green on arrival)
- [x] Step 14: `internal/cli/start_test.go` `Test_start_json_writes_no_bytes_to_stdout_when_it_refuses` — a malformed feature (13's missing progress heading) and an unknown feature: `cli.ExitCode` is 1, stdout length is 0, stderr is one R14a line with no `(no files changed)` tail (green on arrival)
- [x] Step 15: `internal/cli/start_test.go` `Test_start_json_still_reports_usage_errors` — `--json` with no feature, and an undefined flag alongside `--json`: `cli.ExitCode` is 2, stdout empty (green on arrival)
- [x] Step 16: `internal/cli/start_test.go` `Test_start_accepts_the_json_flag_after_the_feature` and `Test_start_json_help_prints_usage_not_json` — `{"start","demo","--json"}` behaves as `{"start","--json","demo"}`; `--json --help` prints `startUsage` to stdout and exits 0 (green on arrival)
- [x] Step 17: `internal/cli/start_test.go` `Test_start_without_json_is_byte_identical_to_the_text_brief` — the same fixture without `--json` produces exactly today's stdout; `render.go`'s `RenderText` path is untouched (green on arrival)
- [x] Step 18: `internal/cli/cli.go` — move `splitLeadingPositionals` out of `finish.go` beside the other shared command helpers, doc comment updated to say both `finish` and `start` use it; no behaviour change (refactor, done ahead of step 10 so `start.go` could use it immediately)
- [x] Step 19: mutation-verify, **one at a time**, each copied to `$TMPDIR` (not `git stash` — the stash stack is shared across worktrees) and diffed byte-identical after restore: (a) `json:"step,omitempty"` reddened `Test_RenderJSON_marshals_a_nil_step_as_null` and `Test_start_json_emits_a_null_step_for_a_completed_feature`; (b) `json:"done,omitempty"` reddened `Test_RenderJSON_keeps_the_zero_counts_a_caller_needs` and `Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files`; (c) `json:"found,omitempty"` reddened `Test_RenderJSON_keeps_a_section_RenderText_omits`. All three restores diffed byte-identical to the original `brief.go`
- [x] Step 20: `go doc ./internal/assemble RenderJSON` and `go doc ./internal/assemble Brief` read as the contract a caller needs; verification commands per the standing brief; report the test-count delta
- [x] Step 21: all tests green → mark SCENARIO-15 done in `specification.md`, and rewrite `docs/specifications/brief/STATE.md` folding in the Handoff below

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **`--json` is on `start` and nowhere else.** *Decisions taken* 3 and open question 8 defer
  `status`/`next`/`show`/`state get`/`handoff`; the rule of thumb for adding it later is
  "structured payload or record set, not a scalar".
- **The payload is `json.Marshal` of `assemble.Brief` — nil marshals to `null`, everywhere,
  with no normalization.** `"step": null` (R14's discriminator), `"shortfalls": null` when
  none. Any later `--json` surface inherits this rule rather than re-deciding it; emitting
  `[]` on one surface and `null` on another is the thing this forbids.
- **No field carries `omitempty`.** `done`/`open` at `0` are what separate a complete feature
  (`2`/`0`) from one with no step files (`0`/`0`) — `"step": null` alone cannot. `found` at
  `false` is what separates an absent heading from an empty body (SCENARIO-14).
- **Under `--json`, stdout is one compact document plus `\n`, or zero bytes.** All notices —
  12's complete/no-steps lines, 14's shortfall lines — stay on stderr. A refusal is still a
  plain R14a stderr line at exit 1 with **no** JSON envelope, because R14a fixes one refusal
  template and a script distinguishes refused from finished by exit code alone.
- **Exit codes are untouched by `--json`:** 0 open-step / complete / no-steps / shortfall, 1
  malformed or unknown feature, 2 usage.
- **`RenderJSON` always writes a document; `RenderText` writes nothing when `Step` is nil.**
  JSON also keeps sections `RenderText` omits for an empty body. The renderers are not
  interchangeable.
- **`start` accepts `--json` before or after the feature**, via `splitLeadingPositionals`
  (moved to `cli.go` in Step 18 — `finish.go` no longer owns it).

**Left unbuilt** — named so nobody assumes it exists:

- `status --json`, and `--json` on `next`/`show`/`state get`/`handoff` — deferred by
  *Decisions taken* 3 / open question 8, not missed.
- R13's output budget and truncation — still unowned. `cfg.DefaultOutputBudgetBytes` (8192)
  remains unconsumed.
- No JSON error/refusal envelope, no `"feature"` key in the payload, no schema-version key.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`; 16–21's `finish` refusals; 22's
  `check`.

**Traps** — things that look right and are not:

- **R13's eventual owner must never truncate a `--json` payload.** Cutting at a line boundary
  yields an unparseable document. The budget has to skip, or refuse, a structured payload —
  it cannot trim one.
- **The struct tags in `brief.go` are the wire contract.** Renaming a `Brief`, `Step`,
  `Section` or `Shortfall` field now silently changes user-visible JSON unless the tag is kept
  — and without tags Go emits `Done`/`Open`/`Step`/`Heading`, which is a different contract
  again.
- `brief start <f> | wc -c == 0` is **not** the completion test under `--json` — stdout is
  never empty on an exit-0 `--json` run. The structured test is `"step": null` plus
  `done`/`open`, per SCENARIO-12's trap.
- `SetEscapeHTML(false)` is deliberate; a plain `json.Marshal` would rewrite `<` and `&` in
  markdown bodies to `<`/`&`. A test pins it.
- `start --json --help` prints the usage text to stdout, not JSON. Intended, not a stdout
  purity bug.
- A feature name starting with `-` is taken as a flag by `splitLeadingPositionals` —
  pre-existing for `finish`, now shared by `start`, unowned.
- Unmarshalling **collapses** null-vs-absent and absent-vs-zero: a test that only decodes into
  a struct or map cannot prove `"step": null` is present rather than omitted, and an omitted
  `"found"` decodes to `false` either way. Those four claims (`step`, `done`/`open`,
  `shortfalls`, `found`) are asserted on raw bytes on purpose; do not "clean them up" into
  decoded comparisons.
