---
id: SCENARIO-04
status: done
---

# SCENARIO-04: --json lists dropped entries in a structured field

## Scenario

Scenario: SCENARIO-04 — --json lists dropped entries in a structured field
  When I finish with --json and entries are dropped
  Then "dropped_entries" is the last field and each element carries
       severity, rule, path, line, detail, heading, tag, text in that order
  And with no drops "dropped_entries" is []

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/finish_internal_test.go` `Test_finish_json_is_one_exact_document_mem` — extend the exact-bytes `want` with `,"dropped_entries":[]` after `modified`; fails because today's document has no such key at all (its zero-drop fixture is the no-drop `[]` case)
- [x] Step 2: `internal/cli/finish_dropped_internal_test.go` `Test_finish_json_dropped_entries_key_order_and_values_mem` — exact-bytes golden run with `--json`, fixture built inline: old body keeps one entry under each dropped heading (so the diff cannot pass by ignoring the new body) and drops one untagged `## Open debts` entry, one tagged `## Traps` entry, and one tagged `## Left unbuilt` entry over 80 runes; every expected `severity`/`rule`/`path`/`line`/`detail`/`heading`/`tag`/`text` value in `want` is a literal (never built by calling `dropDetail` or read off a `scaffold` result — `path` alone may come from the fixture's own `filepath.Join`/`memJSONString`); asserts `dropped_entries` is the document's last top-level key, each element's keys appear in order severity/rule/path/line/detail/heading/tag/text, the Open-debts element carries `"rule":"dropped-debt"` and `"tag":null`, the Traps element carries `"rule":"dropped-entry"` and its tag as a JSON string, and the over-80-rune element's `text` is the full literal string (no `…`) while its `detail` is the same string cut to 80 runes plus `…`; fails because `finishDocument` has no `dropped_entries` field to marshal
- [x] Step 3: `internal/cli/finish_dropped_internal_test.go` `Test_finish_json_dropped_entries_detail_matches_the_text_row_mem` — two independent fixtures built from the same literal old/new state bodies (a fresh mem tree and fresh `memWriteInput` calls each time, not the same tree reused, since a second run on the same tree would be an R11 no-op with `Dropped` empty): one run without `--json` to capture the stdout WARN row's detail (the substring after the `WARN  <path>:<line>  ` prefix), one run with `--json` to decode the document; asserts the JSON element's `detail` is byte-identical to the text-mode row's detail; fails because `dropped_entries` does not exist yet

### Green

- [x] Step 4: `internal/cli/finish_dropped.go` `finishDroppedJSON` — new unexported struct, doc comment stating the field order and that `Tag` is `nil` for an untagged entry
- [x] Step 5: `internal/cli/finish_dropped.go` `finishDroppedEntries` — builder mirroring `checkFeatures`'s empty-vs-nil discipline, reusing `dropDetail` for `Detail` and `dropSeverity` for `Severity`
- [x] Step 6: `internal/cli/finish.go` `finishDocument` — add `DroppedEntries []finishDroppedJSON \`json:"dropped_entries"\`` as the struct's last field; extend the type's doc comment to name it (last, after `modified`, always present, `[]` when empty)
- [x] Step 7: `internal/cli/finish.go` `runFinish` — in the `out.json` branch, populate `DroppedEntries` on the constructed `finishDocument` via `finishDroppedEntries`

### Sweep

- [x] Step 8: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 9: full verification per `.claude/rules/agent-briefs.md`; two named mutations, each restored and diffed before the next:
  - `finishDroppedEntries` returning `var out []finishDroppedJSON` (nil) instead of a sized, non-nil slice → `Test_finish_json_is_one_exact_document_mem`'s `"dropped_entries":[]` assertion goes red (renders `null`) — confirmed
  - the tag branch always populating `Tag` (e.g. `Tag: &d.Tag` unconditionally) instead of leaving it `nil` for `d.Tag == ""` → `Test_finish_json_dropped_entries_key_order_and_values_mem`'s `"tag":null` assertion for the Open-debts element goes red (renders `"tag":""`) — confirmed

### Discovered during Verify (out of this scenario's own plan, required to keep the full suite green)

- [x] Step 10: the full-suite run surfaced that `Test_every_command_help_names_its_json_documents_top_level_fields` and `Test_help_finish_json_is_the_exact_document`/`Test_prints_finish_flag_prose_in_its_flag_table` already pin `finishLong`'s JSON-field sentence against the live document's own keys — contrary to this plan's own Handoff, which assumed no such test existed. Added `"dropped_entries"` to `finishLong`'s `jsonFieldsParagraph(...)` call (binding per specification.md's Surface & Copy: "`jsonFieldsParagraph` for `finish` lists `dropped_entries` after `modified`") and updated the two golden strings in `internal/cli/help_json_test.go` and `internal/cli/help_test.go` to match. SCENARIO-10's own drop-reporting *prose paragraph* (a separate sentence, still absent) remains unbuilt.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `dropped_entries` is `finishDocument`'s last field, always a non-nil (possibly empty) slice — never `omitempty`, never conditionally omitted. SCENARIO-09/10 must preserve both the position and the always-present rule.
- Each element's `path` is `res.StatePath` verbatim, repeated per element (not hoisted to the document) — matches the Surface & Copy sample, which shows `path` inside the element.
- JSON `detail` is produced by calling the existing `dropDetail(d)` helper, not a second string-building path — keeps text and JSON renderings from drifting apart.
- `tag` is `*string`, `nil` for `d.Tag == ""`; a non-empty tag is a plain JSON string (never wrapped in the `tagged <token>` prose the text row uses).
- `finishLong` and `jsonFieldsParagraph("feature", "step", "changed", "handoff_path", "state_path", "next", "modified")` are left untouched here — confirmed no existing test (`Test_help_finish_json_is_the_exact_document`, which pins `finishLong`'s description as an exact-bytes golden) pins a field list against the struct, so nothing reddens by leaving it out of scope. SCENARIO-10 adds `dropped_entries` to that call and to the golden in the same change.

**Left unbuilt** — named so nobody assumes it exists:
- `dropped_entries` mentioned in `finishLong`'s JSON paragraph / `jsonFieldsParagraph` call — SCENARIO-10.
- `finishLong`'s drop-reporting prose paragraph — SCENARIO-10.
- Continuation lines/indented sub-items in `markdown.Entries`, the empty/duplicate heading guard, CR-strip's own CLI proof — SCENARIO-07/08, unchanged from prior handoffs.
- A refusal-carries-no-`dropped_entries` test for a finish that **would** drop entries — not built here. `Test_finish_json_refusal_is_unchanged_mem` already `ElementsMatch`es the error document's keys against `{"schema","command","ok","exit_code","error"}` (excludes `dropped_entries`) and stays green untouched by this scenario's changes, but its fixture (`newMemFinishFixture`) has only paragraph lines — zero entries — and its refusal is an unknown-step usage error that fires before any diff runs. It pins the error document's key *shape*, not D5's "a finish that would drop entries is refused with no rows" — that is an absence claim with no control arm until a drops-bearing fixture exists. SCENARIO-09 still owns that case; it should not treat the existing test as having proven it.

**Traps** — things that look right and are not:
- `runFinish`'s `out.json` branch returns before the text-mode stdout WARN-row loop ever runs, so one invocation never produces both the JSON document and the text rows. Proving "detail identical to the text row detail" needs two independently-built fixtures (fresh mem tree, fresh `memWriteInput` calls) run separately — reusing one mem tree for both runs turns the second into an R11 no-op (`Test_finish_json_decodes_next_and_changed_correctly_mem`'s "changed is false" case shows exactly this shape), and its `Dropped` comes back empty.
- `checkFindingJSON`'s `Line *int` (nullable, for a whole-file finding) is not the shape here: `DroppedEntry.Line` is always populated for a drop, so `finishDroppedJSON.Line` is a plain `int`, never a pointer — do not copy the `checkFindingJSON` nullable-line pattern verbatim.
- `assert.JSONEq` on a single decoded field (e.g. `doc["dropped_entries"]` in isolation) cannot pin key *order* — order-sensitive claims (the field being last, the per-element key sequence) need the raw-bytes exact-document golden, not a map-decode comparison.
