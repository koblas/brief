---
id: SCENARIO-05
status: done
depends-on: []
---

# SCENARIO-05: An unknown feature lists the known ones

## Scenario

Scenario: SCENARIO-05 An unknown feature lists the known ones
  When I run "brief start nosuch" (likewise finish, new step, check)
  Then stderr is: brief start: no feature "nosuch" in docs/specifications; known: <names>
  And the exit code is 1

## User-visible contract

- Commands: `brief start <f>`, `brief finish <f> <step> …`, `brief new step <f>`,
  `brief check <f>`. `brief check` with no argument and `brief new feature` are unaffected.
- Text mode — stdout empty, one stderr line, exit 1:
  - known features exist: `brief <cmd>: no feature "<name>" in <rel feature dir>; known: <a>, <b>, <c>`
  - none exist (feature dir empty or absent):
    `brief <cmd>: no feature "<name>" in <rel feature dir>; known: none; run 'brief new feature <name>' to create it`
  - `new step` and `finish` (write commands) append ` (no files changed)` — R14a in
    `docs/specifications/brief/specification.md` requires the tail on every write refusal. The
    ruled copy already departs from R14a's `<path>: …` shape (path mid-line), so it specifies
    the message, not the template; the disk-state promise is orthogonal to message shape and
    survives. `start`/`check` do not carry it.
  - `<name>` rendered with `%q`; `<rel feature dir>` = `displayPath(wd, <root>/<cfg.FeatureDirectory>)`,
    so a custom `feature-directory` and a subdirectory wd (`../…` chain) both follow.
  - Ordering = `fs.ReadDir` byte order, the same contract `assemble.Status` relies on; no
    separate sort, no sort assertion (a creation-order fixture would pass with no sort at all).
- `--json` — one error document on stdout, stderr empty, exit 1: `kind` `"refusal"`, `message` =
  the text line byte for byte, `path` = **absolute** feature dir, `line` null,
  `problem` = `no feature "<name>"` (no path inside it — R6 keeps JSON paths absolute and the
  path already lives in `path`), `fix` = the `known: …` segment (always non-empty),
  `files_changed` per existing `filesChangedFor`.
- Listing the feature dir fails (e.g. the configured feature dir is a regular file): exit 1,
  generic `failure` kind, the listing error — not a not-found line claiming `known: none`.

## Design decisions (this plan)

- **Where the list is computed:** `assemble` owns reading the feature layout, so it gains
  `(*Server).Features(ctx) ([]string, error)` — names of entries in the feature dir that
  `fs.ReadDir` reports `IsDir()` (symlinks and regular files excluded; a missing feature dir is
  `(nil, nil)`). cli composes: on not-found from any of the four commands it asks
  `assemble.Features` for the list. No new platform package; scaffold is untouched (its
  sentinel is only recognised, never re-produced).
- **One copy source:** a cli-internal typed error `*unknownFeatureError{name, dir, known, err}`
  (wrapping the original error, so `errors.Is(scaffold.ErrNoSuchFeature)` /
  `errors.Is(assemble.ErrNoSuchFeature)` still hold) built by one helper, and one
  `classifyRefusal` branch rendering it, hoisted **above** the `*scaffold.RefusalError` branch
  (scaffold's not-found arrives wrapped in one). The write tail is derived from the wrapped
  sentinel: `scaffold.ErrNoSuchFeature` → `noFilesChangedTail`.
- **Text layout:** `textLine` today renders `<path>: <problem>; <fix>`; the ruled line needs
  `<problem> in <path>; <fix><tail>`. Add a classification layout field selecting that shape,
  rendered at `textLine(wd)` time through `displayPath` (classify has no wd). `jsonPath` is
  unchanged and yields the absolute dir.
- The bare-`assemble.ErrNoSuchFeature` branch in `classifyRefusal` and its placeholder fix
  `run 'brief status' to list the known features` are deleted: every not-found now arrives
  enriched, and a kept branch would be unreachable and untested.

## Implementation Plan

- [x] Step 1: `internal/assemble/status_test.go` (or new `features_test.go`) `Test_features_lists_directory_entries_only` — fixture with dirs, a `README.md`, and a symlink; plus missing feature dir → empty, and feature dir that is a regular file → error (red)
- [x] Step 2: `internal/assemble/status.go` `(*Server).Features` — enumerate real directories under the feature dir; doc comment states order, exclusions, missing-dir and error contract (green)
- [x] Step 3: `internal/cli/unknown_feature_test.go` `Test_an_unknown_feature_names_the_known_ones` — table through `cli.Run`: start / check / new step / finish with two known features (exact stderr line, empty stdout, exit 1, tail only on new step + finish); `check a/b` (the `validFeatureArgument` reject) gets the same copy, in its own test (`Test_a_feature_argument_with_a_path_separator_gets_the_same_not_found_copy`, go-testing's no-branching-in-a-test-body rule); `brief check` with no argument does not (red)
- [x] Step 4: same file, split into `Test_an_unknown_feature_with_no_feature_directory_suggests_creating_one` and `Test_an_unknown_feature_with_an_empty_feature_directory_suggests_creating_one` (one table-driving `bool` in a shared test would have violated the no-logic-in-case-struct rule) — the `known: none; run 'brief new feature <name>' to create it` line for each of the four commands (red)
- [x] Step 5: same file `Test_an_unknown_feature_line_is_relative_to_the_working_directory` — wd in a subdirectory below the config root, and a custom `feature-directory` in `.brief.yaml`; expected dir derived from the fixture's own root, not a literal (red)
- [x] Step 6: same file `Test_an_unlistable_feature_directory_is_a_failure_not_a_not_found` — configured feature dir is a regular file: exit 1, stderr carries no `known:` (control arm: same argv against a real empty dir does print `known: none`) (red)
- [x] Step 7: `internal/cli/refusal.go` `unknownFeatureError` + `enrichUnknownFeature` helper that recognises either sentinel, calls `assemble.Features`, and returns the enriched error (or the listing error) (new)
- [x] Step 8: `internal/cli/refusal.go` `classifyRefusal` — `*unknownFeatureError` branch first (problem, fix, dir as path, tail from the wrapped sentinel, "problem in path" layout); delete the bare-sentinel branch and its placeholder fix (green)
- [x] Step 9: `internal/cli/refusal.go` `(refusalClassification).textLine` — render the new layout through `displayPath` (green)
- [x] Step 10: `internal/cli/start.go`, `check.go`, `finish.go`, `new.go` (new step path only) — route the feature call's error through the helper before `out.refusal` (green)
- [x] Step 11: `internal/cli/json_refusal_test.go` — `"start unknown feature"`, `"check unknown feature"` and `"new step on an unknown feature"` rows now expect `wantPath` = absolute feature dir; `refusalMatrixRows` doc comment updated (bare not-found shape is gone) (update)
- [x] Step 12: `internal/cli/new_step_test.go` `Test_refuses_on_one_line_for_an_unknown_feature_for_step` — rewritten to keep only the `errors.Is(scaffold.ErrNoSuchFeature)` and one-line-on-stderr assertions; the exact copy is `Test_an_unknown_feature_names_the_known_ones`'s own contract now (update)
- [x] Step 13: doc comments — `classifyRefusal` (order rationale now names the not-found branch), `internal/cli/json.go` refusal-kind list, `internal/assemble/doc.go` ("Start, Status and Features are the entry points", plus a paragraph naming what `Features` does) (update). The exact-bytes refusal golden `Test_json_mode_renders_a_refusal_as_one_document` uses the missing-STATE fixture and is untouched
- [x] Step 14: mutation-verify, one at a time, stashed to `$TMPDIR` per `.claude/rules/agent-briefs.md`, each restore diffed byte-identical:
  M1 drop the helper call in `finish.go` only → only the `finish` row of `Test_an_unknown_feature_names_the_known_ones` reddened, `start`/`check`/`new_step` stayed green;
  M2 made `textLine` always use the path-problem shape → all four rows reddened (re-verified again after the lint pass flipped the comparison's polarity);
  M3 forced `knownFeaturesFix`'s empty-list branch off → both empty-list tests (Step 4) reddened on all four rows, Step 3's non-empty-list test stayed green;
  M4 dropped the `IsDir` filter in `Features` → only `Test_features_lists_directory_entries_only` (Step 1) reddened, Step 3 stayed green (its fixture holds no file);
  M5 set the tail unconditionally → `start`/`check` rows reddened, `new_step`/`finish` stayed green (already carried the tail);
  M6 swallowed the listing error in the helper → `Test_an_unlistable_feature_directory_is_a_failure_not_a_not_found` (Step 6) reddened, its control arm (`Test_an_unknown_feature_with_no_feature_directory_suggests_creating_one`) stayed green;
  M7 moved the `*unknownFeatureError` branch below `*scaffold.RefusalError` in `classifyRefusal` → `new_step`/`finish` rows of Step 3 reddened, `start`/`check` stayed green, exactly as the trap predicts
- [x] Step 15: `go build ./...` clean; `go test ./...` green, unpiped (all 8 packages `ok`, 0 skips in `internal/cli`/`internal/assemble`); `go test -race ./internal/cli/... ./internal/assemble/...` green; `golangci-lint run ./...` → `0 issues` → mark SCENARIO-05 done in specification.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Not-found copy has one source: `*unknownFeatureError` (cli) rendered by the first
  `classifyRefusal` branch — every command taking a feature argument routes its error through
  the same helper; a new such command (or S06/S08 rework of status/check) must too.
- The known list comes from `assemble.(*Server).Features` — real directories only, `fs.ReadDir`
  order. `assemble` owns layout reading; cli never lists directories itself.
- Write-command not-found keeps ` (no files changed)`; read commands do not. The ruled copy
  already departs from R14a's `<path>:` prefix shape, so it specifies the message, not the
  template — R14a's disk-state promise is orthogonal and survives. Tail is derived from the
  wrapped sentinel (`scaffold.ErrNoSuchFeature`), not from the command name.
- `known:` lists only openable directories — symlink entries appear in `brief status` (as
  Problem rows) but never in `known:`, because `OpenRoot` refuses a symlink escape and listing
  it would name a feature that cannot be started. S06/S07 must not "reconcile" the two lists.
- JSON: `path` = absolute feature dir, `problem` = `no feature "<name>"` (no path),
  `fix` = `known: …` segment. `message` = text line (relative dir).
- A listing failure is a `failure`, never rendered as `known: none`.

**Left unbuilt** — named so nobody assumes it exists:
- A structured `known` array in the JSON error object — R3's field set is fixed; scripts use
  `status --json` (S07) for the list.
- Fixing Start/NewStep/Finish mapping *any* `OpenRoot` error (permission denied, symlink
  escape) to not-found — pre-existing, unowned.
- Removal of `scaffold.noSuchFeatureRefusal`'s `Problem`/`Fix` copy — now never rendered;
  refactor, unowned.

**Traps** — things that look right and are not:
- `classifyRefusal` must test `*unknownFeatureError` before `*scaffold.RefusalError`; scaffold's
  not-found is a `RefusalError` too, and lower placement silently keeps the old copy for
  `new step`/`finish` while `start` looks fixed.
- `scaffold.noSuchFeatureRefusal`'s `Fix` (`run 'brief new feature <f>' to create it`) looks
  live but is no longer printed; the empty-list copy lives in cli.
- An existing directory that `OpenRoot` cannot open is still reported as not-found, and its
  own name then appears in `known:` — contradictory but pre-existing; not a regression.
- A sort-order assertion on a creation-order fixture is vacuous: `fs.ReadDir` already sorts.
- Putting the feature dir inside JSON `problem` would leak a relative path into JSON (R6).
