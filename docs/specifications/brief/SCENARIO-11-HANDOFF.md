# SCENARIO-11 Handoff: One malformed feature does not blind status to the rest

## What was built

- `assemble.Problem{Path, Detail, Fix}` and `FeatureStatus.Problem *Problem`
  (`internal/assemble/status.go`). Nil when the feature read cleanly; when set,
  `Done`/`Total`/`Next`/`Blocked` stay at zero.
- `Status`'s entry loop is now three-way: directory → `featureStatus` (unchanged
  reading logic, now tolerant); symlink (`e.Type()&fs.ModeSymlink != 0`) → marked
  `!` row named after the symlink, never opened or resolved; anything else
  (regular file, etc.) → skipped, no row — SCENARIO-10's decision, unchanged.
- `featureStatus` no longer returns an error. A failure opening/listing the
  feature's own directory, or a `readSteps` failure (unreadable or unparseable
  step file), becomes that row's `Problem` via the new `newProblem(base, err,
  nameable bool) *Problem` helper. `nameable` discriminates: an OS-level failure
  surfaces as `*fs.PathError` and, when `nameable` is true (a step-file failure),
  its own `.Path` (relative to the feature root) is joined onto `base` to name
  the file; a directory-level failure keeps `base` alone (its `PathError.Path` is
  already the feature's own name — joining would repeat it); a frontmatter parse
  failure is not a `PathError` at all and carries no file name of its own, so
  `Path` stays at the feature directory.
- `readSteps` (`internal/assemble/assemble.go`) is untouched — tolerance lives
  only in `featureStatus`, confirmed by
  `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`.
- `RenderStatusText` renders `<name> ! ! !` when `row.Problem != nil` — four
  single-token fields, matching 07/08/09's arity contract.
- `internal/cli/status.go`'s `runStatus` writes one stderr line per malformed
  row, in row order, **before** the "no features found" `len(rows) == 0` check
  is reached (it isn't, when any row exists) and **before**
  `RenderStatusText`: `brief status: <absolute path>: <detail>; <fix>\n`, via
  `flattenOneLine` on `Detail`/`Fix`, written inline — not through
  `renderRefusal`. Returns `nil`: exit 0 always.
- `statusUsage` now states the `!` row and the exit-0 rule.
- Doc comments updated: `Status`, `FeatureStatus`, `Problem` (new),
  `RenderStatusText`, and `internal/assemble/doc.go`'s package doc (Start
  refuses; Status degrades — same shared `readSteps`, different caller
  reaction). Verified with `go doc ./internal/assemble`.

## Tests added (19 new; 283 → 302, 0 skips, 0 fails)

`internal/assemble/status_test.go` (11): marks a feature whose frontmatter
doesn't parse (the core claim, with a byte-identical-rows control on the other
three features); one Problem per feature not per file; marks a step file that
can't be read (escaping symlink); marks a feature directory that can't be
listed (mode 000, skipped under euid 0); marks a symlinked feature directory
rather than dropping it; skips a regular file with no row (control for the
symlink test); leaves an id/filename mismatch unmarked; leaves a no-step-files
feature unmarked; still propagates an unreadable top-level feature directory;
still propagates an invalid step-file pattern; returns nil not `[]` for zero
features.

`internal/assemble/assemble_test.go` (1): `Start` still refuses a step file
with no frontmatter — the SCENARIO-13 tripwire. Materially overlaps the
pre-existing `Test_returns_an_error_when_a_step_file_has_no_frontmatter`
(line ~460); added anyway per the plan, as an explicit SCENARIO-11-scoped
regression pin rather than relying on the pre-existing test's name to signal
intent.

`internal/assemble/render_test.go` (1): the `! ! !` literal.

`internal/cli/status_test.go` (6): the other-three-features-unchanged control
arm (two independent runs, both asserted against literal stdout, never
cross-derived); the exact stderr line; the 10↔11 seam for an all-malformed
repo (row printed, notice absent); the same seam for an all-symlink repo;
the same seam's deliberate opposite for an all-regular-file repo (still "no
features found" — SCENARIO-10 preserved); N malformed → N stderr lines.
Plus a `--help` assertion for the updated usage text.

## Mutation verification (all four, one at a time, stashed via `$TMPDIR`, diffed byte-identical after restore)

- (a) Reverted `featureStatus` to `(FeatureStatus, error)`, propagating up:
  `Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse`,
  `Test_status_reports_the_other_features_unchanged_when_one_is_malformed`,
  `Test_status_names_the_reason_for_a_malformed_feature_on_stderr` all went
  red on `"assemble: no frontmatter found"` — the tolerance is real.
- (b) `continue`d past `row.Problem != nil` instead of appending:
  `Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice`
  went red with exactly the predicted symptom — empty stdout, "no features
  found" on stderr, exit 0 — the 10↔11 interaction that mutation (a) cannot
  reach (it errors out before the `len(rows) == 0` branch).
- (c) Dropped the symlink branch, falling back to `!e.IsDir() { continue }`:
  `Test_status_marks_a_symlinked_feature_directory_rather_than_dropping_it`
  and `Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice`
  went red; `Test_status_on_a_repository_whose_only_entry_is_a_regular_file_still_says_no_features_found`
  stayed green, as required.
- (d) Made the top-level `OpenRoot` failure `return nil, nil` unconditionally:
  `Test_status_still_propagates_an_unreadable_top_level_feature_directory`
  went red.

## Binding decisions (unchanged from the plan; repeated here as the record)

- A malformed feature renders `<name> ! ! !` — `!` in all three computed
  fields, never a single-field marker.
- `status` exits 0 when features are malformed, even when every feature is —
  `check` (22) is where that becomes a failure.
- Tolerance lives in `featureStatus`, never `readSteps` — `readSteps` is
  shared with `Start`; SCENARIO-13's red depends on it staying intolerant.
- `FeatureStatus.Problem` is `*Problem`, nil when clean, counts stay zero when
  set — 15's `--json` marshals nil to `null`.
- One Problem per feature, first failure wins.
- Exact stderr copy, exit 0: `brief status: <absolute path>: <detail>; <fix>`,
  written inline in `runStatus`, not via `renderRefusal`.
- A symlink in the feature root is marked `!` and never resolved; a regular
  file is skipped with no row (SCENARIO-10, preserved).
- Malformed is step-file-shaped only: unparseable/absent frontmatter, or a
  step file or feature directory that cannot be read. An id/filename mismatch
  and an empty feature directory are conforming.
- `Status` still never opens `specification.md`.

## Left unbuilt

- `Problem.Line` — no line number carried; 13 adds one if its refusal needs
  it.
- `start`'s malformed handling is untouched — 13's.
- `--json` / any `Problem` marshaling (15), `check`'s findings (22),
  `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, 12's complete-feature
  stderr.
- Following a symlinked feature directory — a new behaviour, deliberately not
  decided here.

## Traps for the next reader

- `Status`'s entry filter is three-way on purpose (directory / symlink /
  everything else) — do not collapse to "not a directory → mark".
- `!` on a symlink says brief will not follow it, not that the target is
  broken.
- `assemble.Problem` duplicates `scaffold.RefusalError` minus `Line`,
  knowingly (`assemble` must not import `scaffold`); field names don't line
  up (`RefusalError.Problem` is text, `Problem.Detail` is).
- `stepfile.ParseFrontmatter`'s errors name no file — `newProblem`'s
  `nameable` parameter exists because of this; a `*fs.PathError` failure
  (open/read) does name one via its own `.Path`, a parse failure does not.
- `brief status` still cannot read `brief`'s own tree — after this scenario
  it prints `brief ! ! !` plus a stderr line instead of exiting 1. The
  crossover still owns adding frontmatter to this repo's own plan files.
