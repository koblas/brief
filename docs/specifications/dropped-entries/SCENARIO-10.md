---
id: SCENARIO-10
status: done
---

# SCENARIO-10: finish --help documents drop reporting

## Scenario

Scenario: SCENARIO-10 — finish --help documents drop reporting
  When I run brief finish --help
  Then it contains the ruled drop-reporting paragraph before the JSON paragraph
  And the JSON paragraph lists dropped_entries after modified

## Context

CLI-only (help text), no HTTP surface — `api-conventions` skill not invoked.

SCENARIO-04 already added `dropped_entries` to `finish`'s `jsonFieldsParagraph` call and to
the two exact-help goldens (`finishHelp` in `internal/cli/help_test.go`,
`Test_help_finish_json_is_the_exact_document`'s `description` in
`internal/cli/help_json_test.go`), so the scenario's second clause is already true. This
scenario adds only the missing paragraph: `finishLong` (`internal/cli/finish.go`) carries no
prose about drop reporting today — it jumps straight from the flag-body paragraph to
`jsonFieldsParagraph(...)`.

`docs/specifications/dropped-entries/specification.md`'s *Surface & Copy* `--help` block is
binding, verbatim, five lines, all ≤72 columns (well inside the package's 80-column budget
enforced by `Test_every_leaf_help_line_fits_in_80_columns`):

```
Each entry under the four state headings that is missing from the new
body is listed on stdout as a WARN finding (rule dropped-debt under the
open-debts heading, dropped-entry otherwise); its line is in the file as
it was before replacement. Removal is reported, never refused; a
reworded entry counts as removed. Exit status stays 0.
```

It is appended to `finishLong` as its own paragraph (blank line before and after), between
the existing flag-body paragraph and the `jsonFieldsParagraph(...)` call.

Repo-wide grep for `finish`'s distinctive help text (`at most one of --handoff and --state`,
excluding `.git`) found only the two goldens listed below plus the unrelated `-`-flag-clash
error string in `finish_internal_test.go`/`docs/specifications/brief/SCENARIO-05.md` (error
copy, not help prose — no change needed). The three SHIP WITH CHANGES doc edits from
`product-vision`'s items 2–4 (the `<feature> (in flight|complete)`/"prints them bare" clause
and Findings-render decision in `docs/specifications/brief`'s spec and STATE.md, and the
Default profile's `## Open debts ... (SCENARIO-XX)` tag) are already present — confirmed by
grep — so nothing here needs to fold them in.

## Implementation Plan

### Red
- [x] Step 1: `internal/cli/help_test.go` `Test_finish_help_documents_drop_reporting_before_the_json_paragraph` — new test pinning the verbatim drop-reporting paragraph and its position (after the flag-body paragraph, before the `With --json, ...` paragraph) in `brief finish --help` stdout; fails: paragraph absent today
- [x] Step 2: `internal/cli/help_test.go` `finishHelp` — insert the paragraph into the byte-exact golden; fails `Test_prints_finish_flag_prose_in_its_flag_table` (byte mismatch) until `finishLong` is edited
- [x] Step 3: `internal/cli/help_json_test.go` `Test_help_finish_json_is_the_exact_document`'s `description` literal — insert the same paragraph at the same position; fails (byte mismatch) until `finishLong` is edited

### Green
- [x] Step 4: `internal/cli/finish.go` `finishLong` — append the verbatim paragraph as its own paragraph, before the `jsonFieldsParagraph(...)` call

### Sweep
- [x] Step 5: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify
- [x] Step 6: full verification per `.claude/rules/agent-briefs.md`; mutate `finishLong` to move the new paragraph to after the `jsonFieldsParagraph(...)` call → Step 1's position assertion, `Test_prints_finish_flag_prose_in_its_flag_table` and `Test_help_finish_json_is_the_exact_document` all go red; restore and diff byte-identical

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The drop-reporting paragraph text is fixed verbatim by `specification.md`'s *Surface &
  Copy* block and now lives in `finishLong` as its own paragraph, ahead of the JSON
  paragraph — do not fold it into or reorder it against `jsonFieldsParagraph`'s output.

**Left unbuilt**: none — this was the feature's last open item per STATE.md's *Left
unbuilt*.

**Traps**: none new. Same trap as SCENARIO-04 applies again here — `finishLong` is pinned
byte-identical in two independent places (`help_test.go`'s `finishHelp`,
`help_json_test.go`'s `description` literal); an edit to one without the other leaves a
golden silently stale until the suite is run.
