---
id: SCENARIO-11
status: done
depends-on: []
---

# SCENARIO-11: One malformed feature does not blind status to the rest

## Scenario

```gherkin
Scenario: SCENARIO-11 One malformed feature does not blind status to the rest  [orig: new]
  Given four features, one of them malformed
  When I ask for status
  Then the malformed feature's line shows "!" and the reason is written to stderr
  And the other three features report normally
  And the command succeeds
  # R18 gives the failing role to check. status answers "what is the state of the world",
  # and "one of them is broken" is the true answer to that question.
```

## Measured current behaviour

Built `./cmd/brief` and ran `brief status` against a fixture tree per shape. Baseline
(three good features) prints `alpha 1/2 SCENARIO-02 0` / `beta 1/1 - 0` /
`gamma 0/1 SCENARIO-01 0`, exit 0. Then:

| Shape | Today | 11's call |
| --- | --- | --- |
| step file with no `---` delimiters | `brief status: assemble: no frontmatter found`, **exit 1** | tolerate + mark |
| frontmatter delimiters present, YAML invalid | `brief status: assemble: stepfile: parse frontmatter: yaml: line 1: …`, **exit 1** | tolerate + mark |
| empty frontmatter block (`---\n---`) | `assemble: stepfile: no frontmatter found: no closing frontmatter delimiter`, **exit 1** | tolerate + mark |
| one of two step files unparseable | **exit 1** — the whole command | tolerate + mark, one line for the feature |
| step file unreadable (mode 000) | `assemble: openat SCENARIO-01.md: permission denied`, **exit 1** | tolerate + mark |
| step file is a symlink escaping the feature root | `assemble: openat SCENARIO-01.md: path escapes from parent`, **exit 1** | tolerate + mark |
| feature directory unreadable (mode 000) | `assemble: openat delta: permission denied`, **exit 1** | tolerate + mark |
| frontmatter `id:` disagrees with the filename | row printed from `pattern.ID(n)`, exit 0 | **unchanged — not malformed** |
| feature dir with no step files / empty dir | `delta 0/0 - 0`, exit 0 | **unchanged — conforming** |
| step "file" is a directory | skipped by `readSteps`, `delta 0/0 - 0`, exit 0 | **unchanged** |
| feature entry is a regular file | skipped, **no row at all**, exit 0 | **unchanged — not a feature; pinned by 10** |
| feature entry is a symlink to a directory | skipped, **no row at all**, exit 0 | **tolerate + mark** |
| missing `specification.md` | invisible — `Status` never opens it | **unchanged — SCENARIO-13's, on `start`** |

None of today's error lines names the feature or the file. Also measured: a repository whose
**only** feature is malformed exits 1 today; after 11 it must print one `!` row and exit 0.

## Decisions this scenario pins

**The line.** `<name> ! ! !` — the name field is still the directory name, and fields 2, 3 and
4 are each the single token `!`. Four whitespace-separated, unpadded, single-token fields, so
07/08/09's arity contract holds and `awk '{print $1}'` still yields the name. Not a
single-field marker: `-` in field 3 is already "no next step" (09), `0/0` in field 2 is
already the empty-feature-dir render (measured above), and `0` in field 4 would assert a
blocked count that was never computed — every single-field variant leaves a fabricated value
in the other two fields. SCENARIO-10's Handoff (read for this: it says "the `!` field",
singular, and is the only place that could have pinned it) does **not** fix the field count,
so it is 11's to decide.

**Exit code: 0, always — including when every feature is malformed.** R14's "nothing to
return is not an error" is not the governing clause here, because there *is* something to
return: a row. R18 gives the failing role to `check`; `status` answers "what is the state of
the world", and `!` is a true answer. Exit 1 would also make `brief status` unusable in the
exact repository where it is most needed.

**stderr: one line per malformed feature, in row order.** Shape is `renderRefusal`'s template
minus the `(no files changed)` tail (R14a: read commands drop it), written directly in
`runStatus` — **not** routed through `renderRefusal`, which returns the error and would drive
exit 1:

```
brief status: <absolute path>: <detail>; <fix>
```

Absolute paths, matching every other `brief` diagnostic (`brief new feature: <wd>/docs/…`,
pinned by `internal/cli/run_test.go:57`).

`Detail` is the **underlying** error's message, not the error as `readSteps` returns it:
`readSteps` wraps with `fmt.Errorf("assemble: %w", err)`, and letting that through would put
an internal package name in user-facing copy and pin it as a literal in Steps 14–16. Strip the
`assemble: ` wrap, or never apply it on this path. From the measured table, the three details
a developer should expect are `no frontmatter found`,
`stepfile: parse frontmatter: yaml: line 1: did not find expected ',' or ']'` and
`permission denied` — the test literals are these, not invented ones.

Two fix strings, one per class:

- parse class → `fix its frontmatter, or run 'brief new step <feature>' to scaffold a conforming step file`
- read class → `make it readable and re-run`

N malformed features produce exactly N lines. **No interaction with SCENARIO-10's notice** —
that notice keys on `len(rows) == 0`, and a malformed feature always yields a row, so the two
are mutually exclusive by construction. Steps 18 and 19 are the tests that keep it that way.

**Non-directory entries in the feature root split into two, deliberately, and the split is
`e.Type()`, not the symlink's target.**

- **A regular file is skipped with no row — not malformed, not a feature.** SCENARIO-10's
  Handoff already pinned this ("Fixture D, root holding only a regular file, also triggers
  [the notice], correctly: `Status` skips non-directories"), so 11 must not contradict it
  without saying so, and does not. The justification stands on its own: `docs/specifications/`
  legitimately holds `README.md`, an index, a `.DS_Store`; marking every stray file as a broken
  feature would make `status` noisy in the common case, and a `!` row has no `<name>` to print
  since field 1 is defined as the feature *directory* name. A repository whose only entry is a
  regular file therefore still prints "no features found" and exits 0. Step 20 pins that.
- **A symlink gets a `!` row.** This is a change of behaviour and 11 owns it. The silent drop
  is precisely what this scenario exists to prevent: a symlinked feature directory is named
  like a feature, sits in the feature root, and vanishes from `status` entirely — the user is
  told "no features found" about a tree that visibly contains their feature. `<name>` is the
  symlink's own name, which is exactly what the directory name would have been. The rule is
  **any symlink entry, without resolving it**: `brief` does not follow symbolic links in the
  feature directory, so the target is irrelevant and there is no resolution branch to get
  wrong or to vary by target type. (Measured: `os.Root` already refuses a root-escaping
  symlink with "path escapes from parent", so following them was never coherent anyway.)
  Detail `symbolic link is not read as a feature directory`, fix `replace it with a real
  directory`.

Making `status` *follow* symlinked feature directories is a different question — new
user-visible behaviour, not tolerance — and is deliberately not decided here.

**One Problem per feature, never per file.** The first failure in `readSteps`' iteration order
stops reading that feature. This bounds stderr by the feature count and keeps the 1:1
row↔line mapping that makes "N malformed → N lines" checkable.

**Where the tolerance lives: `featureStatus`, NOT `readSteps`.** `readSteps` is shared with
`Start` (`assemble.go:90` and `status.go:96` are its only two callers). Making `readSteps`
tolerant would silently make `start` tolerant too and evaporate SCENARIO-13's red before 13 is
written. `featureStatus` catches the error `readSteps` returns and converts it.

**Carried in the data, not the renderer** — a new exported `assemble.Problem` with `Path`,
`Detail`, `Fix`, hung off `FeatureStatus` as a **pointer** (`Problem *Problem`), nil when the
feature read cleanly. A pointer so SCENARIO-15's `--json` marshals it to `null`, matching the
`"step": null` discriminator idiom STATE.md already pins. When `Problem != nil`, `Done`,
`Total`, `Next` and `Blocked` stay at their zero values — a partial count looks measured and
is not.

**Out of scope, so later scenarios keep a real red.** 11 touches neither `Start` nor
`internal/cli/start.go`; `Start` keeps refusing with exit 1 (SCENARIO-13 owns `start`'s
malformed refusal and the `ErrMalformedFeature` / `*RefusalError` unification). 11 does **not**
teach `Status` to read the specification file or its progress list — 13's Given is a
specification missing its progress list, and reading it here would cover 13's condition on the
read path. No `--json` (15), no `check` findings (22), no `Problem.Line`.

## Implementation Plan

- [x] Step 1: `internal/assemble/status_test.go` `Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse` — four features, one with a step file carrying no frontmatter; assert four rows, the malformed row has `Problem != nil` with zero `Done`/`Total`/`Next`/`Blocked`, the other three equal their literal `FeatureStatus` values (red — `Status` errors today)
- [x] Step 2: `internal/assemble/status.go` — exported `Problem` struct (`Path`, `Detail`, `Fix` strings) plus `FeatureStatus.Problem *Problem`; existing `assert.Equal` on whole `FeatureStatus` values still compiles and passes with a nil `Problem` (new)
- [x] Step 3: `internal/assemble/status.go` `featureStatus` — convert a per-feature `OpenRoot`/`ReadDir`/`readSteps` failure into a marked row instead of returning an error; pass the absolute display prefix down so `Problem.Path` is `filepath.Join(s.root, s.cfg.FeatureDirectory, <feature>[, <step file>])` (green)
- [x] Step 4: `internal/assemble/status_test.go` `Test_status_reports_one_problem_per_feature_not_one_per_file` — a feature with two unparseable step files yields exactly one row with one `Problem` (new)
- [x] Step 5: `internal/assemble/status_test.go` `Test_status_marks_a_feature_whose_step_file_cannot_be_read` — step file is a symlink escaping the feature root; deterministic and needs no privileges, unlike a chmod case (new)
- [x] Step 6: `internal/assemble/status_test.go` `Test_status_marks_a_feature_directory_that_cannot_be_listed` — directory at mode 000, skipped when `os.Geteuid() == 0` (new)
- [x] Step 7: `internal/assemble/status_test.go` `Test_status_marks_a_symlinked_feature_directory_rather_than_dropping_it` — a symlink named like a feature, pointing at a real directory holding a conforming step file; assert a row exists, named after the symlink, with a `Problem`. Today the entry produces no row at all (red)
- [x] Step 8: `internal/assemble/status.go` `Status` — an entry whose `e.Type()&fs.ModeSymlink != 0` becomes a marked row without being resolved or opened; an entry that is neither a directory nor a symlink is still skipped. `Problem.Path` is the entry itself, `filepath.Join(s.root, s.cfg.FeatureDirectory, <name>)`. Do not switch the existing `!e.IsDir() { continue }` to "not a directory → mark", which would sweep in regular files and contradict SCENARIO-10 (green)
- [x] Step 9: `internal/assemble/status_test.go` `Test_status_skips_a_regular_file_in_the_feature_directory_without_a_row` — the control for Step 7, differing in exactly one variable: a regular file beside three good features yields three rows, not four, and no `Problem`. Pins SCENARIO-10's decision against Step 8 widening (new; expect green on arrival)
- [x] Step 10: `internal/assemble/status_test.go` `Test_status_leaves_a_feature_whose_frontmatter_id_disagrees_with_its_filename_unmarked` and `Test_status_leaves_a_feature_with_no_step_files_unmarked` — pin the two tolerate-silently classifications so a later change cannot quietly reclassify them (new; expect green on arrival — say so rather than manufacturing a red)
- [x] Step 11: `internal/assemble/status_test.go` — three separate test funcs, so each claim has its own mutation: `Test_status_still_propagates_an_unreadable_top_level_feature_directory` (the directory *containing* the features, `cfg.FeatureDirectory` itself — not one feature's directory, which Step 6 tolerates), `Test_status_still_propagates_an_invalid_step_file_pattern`, and `Test_status_returns_nil_not_an_empty_slice_for_zero_features` (new; expect green on arrival)
- [x] Step 12: `internal/assemble/assemble_test.go` `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse` — regression pin that Step 3's tolerance did not leak through the shared `readSteps` into `Start`, which is SCENARIO-13's red (new; expect green on arrival)
- [x] Step 13: `internal/assemble/render_test.go` `Test_render_status_text_prints_the_marker_for_a_malformed_feature` — literal `delta ! ! !\n`, no padding, no trailing space (red)
- [x] Step 14: `internal/assemble/render.go` `RenderStatusText` — render `<name> ! ! !` when `row.Problem != nil` (green)
- [x] Step 15: `internal/cli/status_test.go` `Test_status_reports_the_other_features_unchanged_when_one_is_malformed` — the control arm. Two runs differing in exactly one variable (the malformed feature present / absent), **both asserted against literal expected stdout strings**: the three-feature literal, and that same literal with `delta ! ! !` spliced at its sorted position. Never derive one expectation from the other run's output (red)
- [x] Step 16: `internal/cli/status.go` `runStatus` — write one stderr line per row with a non-nil `Problem`, in row order, before `RenderStatusText`, using `flattenOneLine` on `Detail` and `Fix`; return nil (green)
- [x] Step 17: `internal/cli/status_test.go` `Test_status_names_the_reason_for_a_malformed_feature_on_stderr` — exact literal line, absolute path, no `(no files changed)` tail, `require.NoError` on `cli.Run` and `assert.Equal(t, 0, cli.ExitCode(err))` (new)
- [x] Step 18: `internal/cli/status_test.go` `Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice` — stdout is exactly `delta ! ! !\n`, stderr is exactly the one problem line, stderr does **not** contain `no features found`, exit 0. This is the 10↔11 interaction and the one most likely to rot (new)
- [x] Step 19: `internal/cli/status_test.go` `Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice` — the second half of that seam: the symlink case must not be swallowed by the notice either (new)
- [x] Step 20: `internal/cli/status_test.go` `Test_status_on_a_repository_whose_only_entry_is_a_regular_file_still_says_no_features_found` — the deliberate opposite, pinning SCENARIO-10's decision: empty stdout, the notice verbatim, exit 0. Steps 19 and 20 differ in exactly one variable and together fix the boundary (new; expect green on arrival)
- [x] Step 21: `internal/cli/status_test.go` `Test_status_writes_one_stderr_line_per_malformed_feature` — two malformed of four → exactly two stderr lines, in row order (new)
- [x] Step 22: `internal/cli/status.go` `statusUsage` + its assertion in `internal/cli/status_test.go` — the help text is user-visible contract and must state the `!` row and the exit-0 rule (update)
- [x] Step 23: doc comments — `Status` (its "a step file whose frontmatter does not parse, still propagate[s] an error" clause **inverts**; also say root-level failures still propagate, that a symlink entry is marked rather than followed, and that a regular file is skipped), `FeatureStatus` (zero counts when `Problem` is set), the new `Problem`, `RenderStatusText` (the `!` rule beside the `-` rule), and `internal/assemble/doc.go`'s Start/Status paragraph. Verify with `go doc ./internal/assemble` (update)
- [x] Step 24: mutation-verify **four** guards, one at a time, each stashed per the standing brief — (a) revert `featureStatus` to `return FeatureStatus{}, err`: Steps 1/15/17 go red at exit 1, proving the tolerance exists; (b) in `Status`, `continue` instead of `rows = append(rows, row)` when `row.Problem != nil`: Step 18 must go red **with the "no features found" symptom** — empty stdout, the notice on stderr, exit 0 — which is the 10↔11 interaction and the only mutation that tests it (mutation (a) cannot, because `runStatus` returns at `renderRefusal` before the `len(rows) == 0` branch); (c) drop the symlink branch added in Step 8: Steps 7 and 19 go red, Step 20 stays green; (d) return `nil, nil` instead of propagating the top-level `OpenRoot`/`ReadDir` failure: Step 11's top-level test goes red. Never two at once. Report which mutation reddened which test
- [x] Step 25: verification per the standing brief; report the exact test count and its delta
- [x] Step 26: mark SCENARIO-11 done in `docs/specifications/brief/specification.md`, write `SCENARIO-11-HANDOFF.md`, and rewrite `docs/specifications/brief/STATE.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **A malformed feature renders `<name> ! ! !` — `!` in all three computed fields, never in
  one** — `-` in field 3 is 09's "no next step" and `0/0` in field 2 is the empty-feature-dir
  render, so a single-field marker collides with a real value and fabricates the other two.
  Four single-token fields keep 07/08/09's arity contract.
- **`status` exits 0 when features are malformed, even when every feature is** — R18 gives the
  failing role to `check`; `!` is a true answer to "what is the state of the world". 22's
  `check` is where a malformed feature becomes a non-zero exit.
- **Tolerance lives in `featureStatus`, never in `readSteps`** — `readSteps` is shared with
  `Start` (`assemble.go:90`, `status.go:96`); moving it down makes `start` tolerant and
  destroys SCENARIO-13's red. `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`
  is the tripwire.
- **`FeatureStatus.Problem` is a `*Problem`, nil when clean, and the counts stay zero when it
  is set** — 15's `--json` marshals nil to `null`, the same discriminator as `"step": null`;
  a non-zero count beside a Problem would be a value that was never measured.
- **One Problem per feature, first failure wins** — bounds stderr by the feature count and
  keeps the 1:1 row↔line mapping that "N malformed → N lines" checks.
- **Exact stderr copy, exit 0:** `brief status: <absolute path>: <detail>; <fix>` — R14a's
  template minus the `(no files changed)` tail, written inline in `runStatus`, **not** via
  `renderRefusal` (which returns the error and would drive exit 1). Fixes:
  `fix its frontmatter, or run 'brief new step <feature>' to scaffold a conforming step file`
  and `make it readable and re-run`.
- **A symlink in the feature root is marked `!` and never resolved; a regular file is skipped
  with no row** — `brief` does not follow symbolic links in the feature directory, so the
  target is irrelevant. The regular-file half is SCENARIO-10's decision ("`Status` skips
  non-directories", and a root holding only a regular file correctly prints "no features
  found"), preserved unchanged; the symlink half is 11's change, because a silent drop is the
  failure this scenario exists to prevent. Whether `status` should *follow* a symlinked feature
  directory is undecided and needs its own scenario.
- **Malformed is step-file-shaped only: unparseable/absent frontmatter, or a step file or
  feature directory that cannot be read.** A frontmatter `id:` disagreeing with its filename
  and a feature directory with zero step files are **conforming** (09's `pattern.ID(n)` rule
  and its `0/0 - 0` render) — `check` (22) owns the id divergence as a finding, per R18.
- **`Status` still reads step files only** — it never opens `specification.md`. SCENARIO-13's
  Given (a specification missing its progress list) must stay invisible to `status`.

**Left unbuilt** — named so nobody assumes it exists:

- `Problem.Line` — no line number is carried; 13 adds one if its refusal needs it.
- `start`'s malformed handling is untouched: `assemble.ErrMalformedFeature`,
  `ErrNoSuchFeature` and `internal/cli/start.go` are exactly as SCENARIO-06 left them — 13.
- `--json` / any `Problem` marshaling (15), `check`'s `[SEVERITY]` findings (22),
  `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, 12's complete-feature stderr.
- No `renderNotice`/`renderProblem` helper in `cli` — 11 writes its line inline, as 10 did.

**Traps** — things that look right and are not:

- **`Status`'s entry filter is now three-way, and the two non-directory branches differ on
  purpose** — symlink → `!` row, other non-directory → skipped. A "not a directory → mark"
  simplification is the obvious-looking wrong move: it marks every `README.md` in
  `docs/specifications/` and contradicts SCENARIO-10. Steps 19 and 20 are the pair that
  catches it.
- **`!` on a symlink is not a claim the symlink is broken** — its target may be a perfectly
  good feature directory. It says `brief` will not follow it. Anyone tempted to "fix" this by
  resolving the link is opening a new user-visible behaviour, not closing a bug.
- **`assemble.Problem` is a second near-duplicate of `scaffold.RefusalError`** (it is
  `RefusalError` minus `Line`), added knowingly: `assemble` must not import `scaffold`, and
  STATE.md already parks the duplication on 13. Note the field names do **not** line up —
  `RefusalError.Problem` is the text field, `Problem.Detail` is; a reader mapping between them
  will trip.
- **`Status` must keep returning `nil`, not `[]FeatureStatus{}`, for zero features** — 15's
  `--json` depends on it. Do not switch to `make(...)` while touching the loop.
- **stderr-before-stdout is a code convention, not a test.** Tests capture separate buffers, so
  interleaving is unobservable and an assertion on it cannot fail. The reason for the order is
  that a stdout write failure must not swallow the diagnosis.
- **`stepfile.ParseFrontmatter`'s errors name no file** (measured: `no frontmatter found` with
  no path). `featureStatus` must supply the path; do not assume the wrapped error carries it.
- **`brief status` still cannot read `brief`'s own tree** — after 11 it no longer exits 1, it
  prints `brief ! ! !` plus a stderr line, because no `SCENARIO-NN.md` here carries
  frontmatter yet. The crossover owns adding it.
