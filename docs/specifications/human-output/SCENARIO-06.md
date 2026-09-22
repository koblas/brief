---
id: SCENARIO-06
status: done
---

# SCENARIO-06: status reads as a table for people

## Scenario

Scenario: SCENARIO-06 status reads as a table for people
  Given features in progress, complete and malformed
  When I run "brief status"
  Then stdout has a header FEATURE/DONE/BLOCKED/NEXT, words instead of "!" ("(complete)", "(malformed, see below)"), and the next step's title
  And stderr lists one line per malformed feature naming its file, then a summary line with counts
  And with no features, stdout is empty (no header) and the exit code is 0

## User-visible contract

Command: `brief status` (no arguments; argument/flag errors unchanged, exit 2).

stdout, when at least one feature row exists (aligned with `text/tabwriter`, 2-space padding,
last column unpadded, no trailing spaces on any line). Row shapes below are schematic; the
right-hand notes are not output:

```
FEATURE  DONE  BLOCKED  NEXT
<name>   <done>/<total>  <blocked>  <id>  <title>      in progress
<name>   <done>/<total>  <blocked>  (complete)         complete (Total > 0, Done == Total)
<name>   0/0   0        -                              zero step files
<name>   -     -        (malformed, see below)         Problem set
```

- NEXT for an in-progress row with an empty title is `<id>` alone (no trailing `  `).
- Row order is unchanged (`fs.ReadDir` byte order). BLOCKED now precedes NEXT.

stderr, written AFTER stdout's table:
1. one line per malformed row, in row order:
   `brief status: <feature>: <path relative to wd>: <detail>; <fix>` — `<fix>` is
   `Problem.Fix`, which for a step-file fault is the ruled
   `run 'brief check <feature>' to list every fault`.
2. then always (when rows exist) one summary line:
   `brief status: N features: A in progress, B complete, C malformed` — `1 feature` when N is 1;
   all three buckets printed even when zero; bucket words never pluralize.

No features: stdout empty (no header), stderr exactly the existing
`brief status: no features found in <dir>; run 'brief new feature <name>' to create one`,
no summary line. Exit 0 in every case above; exit 1 only for the existing refusals
(invalid config, unreadable feature root, invalid step pattern).

This repo's own tree after S06 (observed today: 4 malformed, not 3):

```
FEATURE       DONE  BLOCKED  NEXT
brief         -     -        (malformed, see below)
cli-cobra     -     -        (malformed, see below)
human-output  -     -        (malformed, see below)
version-flag  -     -        (malformed, see below)
```
stderr: four `brief status: <feature>: docs/specifications/<feature>/SCENARIO-01.md: no frontmatter found; run 'brief check <feature>' to list every fault`
lines, then `brief status: 4 features: 0 in progress, 0 complete, 4 malformed`.

## Implementation Plan

Feature data (`internal/assemble`):

- [x] Step 1: `internal/assemble/status_test.go` — repoint every `Next` assertion at the new `*NextStep` (nil for complete / zero-step / malformed rows); add `Test_status_names_the_next_step_s_title_and_path` (fixture heading distinct from the id) and `Test_status_reports_the_feature_directory_path` (red)
- [x] Step 2: `internal/assemble/status_test.go` `Test_complete_*` table — `(FeatureStatus).Complete` over in-progress / all-done / zero-step / malformed (red)
- [x] Step 3: `internal/assemble/status.go` `NextStep{ID, Title, Path}` + `FeatureStatus.Next *NextStep` + `FeatureStatus.Path` (absolute feature dir, set for every row incl. Problem rows) (new)
- [x] Step 4: `internal/assemble/status.go` `featureStatus` — fill `Next` via `pattern.ID`, `markdown.Title(rest)`, and the step path joined as `assemble.go`'s `Start` does (`pattern.Name(e.number)`); set `Path` in `Status` from `entryPath` (green)
- [x] Step 5: `internal/assemble/status.go` `(FeatureStatus).Complete` — `Problem == nil && Total > 0 && Done == Total` (green)
- [x] Step 6: `internal/assemble/render_test.go` — replace `Test_render_status_writes_four_space_separated_fields_per_feature` and `Test_render_status_text_prints_the_marker_for_a_malformed_feature` with exact-bytes tests: header + aligned columns; `(complete)`; zero-step `-`; malformed `-  -  (malformed, see below)`; empty title → id only; longest name in the LAST row; malformed row last; no line ends in a space; empty `rows` writes nothing (red)
- [x] Step 7: `internal/assemble/render.go` `RenderStatusText` — rewrite over `text/tabwriter` (header only when `len(rows) > 0`; flush error wrapped like the other render errors; tabs/newlines in name/title flattened to a space); rewrite its doc comment to the new contract (green)

Command slice (`internal/cli`):

- [x] Step 8: `internal/cli/status_test.go` — `newStatusFixture`'s step helper writes a title distinct from the id; rewrite every byte-pinned stdout/stderr expectation to the new table, the `<feature>: <rel path>` stderr frame and the summary line (red)
- [x] Step 9: `internal/cli/status_test.go` `Test_status_writes_the_table_before_the_malformed_lines_and_the_summary` — ONE shared buffer passed as both stdout and stderr to `cli.Run`, exact interleaved bytes (red)
- [x] Step 10: `internal/cli/status_test.go` `Test_status_summary_*` table — `1 feature` vs `N features`; zero buckets printed; mixed in-progress/complete/zero-step/malformed counts, with the zero-step feature counted as in progress (red)
- [x] Step 11: `internal/cli/status_test.go` — no-features cases (absent root, empty root, regular-file-only) assert stdout `""` and stderr the single notice with no summary line; malformed-only and symlink-only still print header + row (update)
- [x] Step 12: `internal/cli/status.go` `runStatus` — render the table first, then the per-malformed stderr lines in the new frame, then the summary via an unexported `statusSummary(rows)` (green)
- [x] Step 13: `internal/cli/status.go` `statusLong` + `internal/cli/cli.go` status summary (`print a FEATURE/DONE/BLOCKED/NEXT table of every feature`) + `internal/cli/help_test.go:70` pin — describe the table, the words, the stderr lines, summary, exit 0 (update)
- [x] Step 14: mutation-verify, each individually, stash-protected per `agent-briefs.md`:
  M1 render `-` instead of `(complete)` for a complete row → only the complete-row tests redden;
  M2 move the stderr loop back above `RenderStatusText` → only Step 9's test reddens;
  M3 drop `Total > 0` from `Complete` → only the zero-step rows redden;
  M4 print the summary when `len(rows) == 0` → only the no-features tests redden;
  M5 always write `  <title>` even when empty → only the empty-title render test reddens
- [x] Step 15: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/assemble/... ./internal/cli/...`, `golangci-lint run ./...`; run `go run ./cmd/brief status` in the repo and compare to the contract block above
- [x] Step 16: all tests green → mark SCENARIO-06 done in specification.md; rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `FeatureStatus.Next` is `*NextStep{ID, Title, Path}` (nil = no open step) and
  `FeatureStatus.Path` is the absolute feature dir on every row — S07's ruled payload
  `next:{id,title,path}|null` and feature `path` map 1:1 onto them; S07 must not re-derive.
- `(FeatureStatus).Complete()` = `Problem == nil && Total > 0 && Done == Total` is the one
  definition of "complete" — S07's `complete` bool must call it, not recompute.
- A zero-step feature is NOT complete: DONE `0/0`, BLOCKED `0`, NEXT `-`, counted in the
  summary's **in progress** bucket (every non-malformed, non-complete row is in progress).
- `NextStep.Title` is `markdown.Title` of the step body after frontmatter; empty when the
  step has no `# ` heading, and the table then shows the id alone.
- `NextStep.Path` = feature dir joined with `pattern.Name(n)` — same expression as `Start`.
- Table rendering stays in `assemble.RenderStatusText` (it carries no paths); stderr lines and
  the summary live in `internal/cli/status.go` because they need `displayPath(wd, …)`.
- Summary copy pinned: `brief status: N features: A in progress, B complete, C malformed`,
  `1 feature` singular, all three buckets always present, printed only when rows exist.
- stdout table is written and flushed before any stderr line.
- stderr fix text is `Problem.Fix` verbatim. For step-file faults that is the ruled
  `run 'brief check <feature>' to list every fault`; symlinked / unopenable / unlistable
  feature dirs keep their own specific fix (`replace it with a real directory`, read-class
  fix) because `brief check` cannot repair those. Final product-vision may rule otherwise.

**Left unbuilt** — named so nobody assumes it exists:
- `status --json` document (`statusDocument`) — S07.
- `Problem.Line` — `Problem` still has no line field; S07's `problem.line` needs it or `null`.
- `status` help's `For scripts, use --json; the text layout may change.` sentence — S14.
- `assemble.RenderJSON` removal — still unowned.

**Traps** — things that look right and are not:
- `newStatusFixture` used to write `# <id>` as the heading, so title == id; any NEXT-title
  assertion on that fixture proves only that the id is echoed twice.
- Separate stdout/stderr buffers cannot observe "stderr after the table"; only the
  shared-buffer test pins it.
- `text/tabwriter` pads only tab-terminated cells: the NEXT cell must never be followed by
  `\t`, and an empty title must not leave `<id>  ` behind, or lines gain trailing spaces.
- A tab or newline inside a feature name or title corrupts tabwriter columns — flatten first.
- Defining complete as "Next == nil" makes a zero-step feature read `(complete)`.
- The orchestrator brief said this repo has 3 malformed features; it has 4 (`brief`,
  `cli-cobra`, `human-output`, `version-flag`) — `human-output`'s own plans lack frontmatter.
