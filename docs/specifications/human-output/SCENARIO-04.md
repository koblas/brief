---
id: SCENARIO-04
status: done
depends-on: []
---

# SCENARIO-04: A malformed feature names the step file at fault

## Scenario

```gherkin
Scenario: SCENARIO-04 A malformed feature names the step file at fault
  Given a feature whose step file SCENARIO-01.md has no frontmatter
  When I run "brief status" or "brief start <feature>"
  Then the diagnostic names ".../SCENARIO-01.md" (relative path), not the feature directory
  And its fix is "run 'brief check <feature>' to list every fault"
```

## Decisions this plan takes

- **What fails without a file name.** `readSteps` (`internal/assemble/assemble.go`) fails in
  exactly two ways. (1) `root.ReadFile` returns an `*fs.PathError` whose `Path` is the entry
  name. `newProblem` already joins that onto the feature dir, so the file is already named
  and `readClassFix` stays. (2) `stepfile.ParseFrontmatter` fails in three ways: bare
  `ErrNoFrontmatter`, an unclosed delimiter (wraps `ErrNoFrontmatter`), and a YAML decode
  error. None of the three carries a file name, and that is the whole bug. Start's missing
  `id:` and missing-checklist refusals are built inline with `stepPath` and **already name
  the file**, so no step is planned for them. Check's three `newProblem` calls never receive
  a `readSteps` error, so Check is **unaffected**.
- **Where the name comes from:** `readSteps` knows `e.Name()`, so it wraps the parse error
  there in a new unexported typed error. The error holds the entry name and the wrapped
  error, and `Unwrap` returns the wrapped error so `errors.Is(err, stepfile.ErrNoFrontmatter)`
  still holds. `newProblem` gets a third branch for this type:
  - `Path` = feature dir joined with the entry name.
  - `Detail` = the wrapped error's own message. For no frontmatter this is still exactly
    `no frontmatter found`.
  - `Fix` = `run 'brief check <feature>' to list every fault`, with `<feature>` taken from
    `filepath.Base(base)`, which is the **feature dir**. Do not take it from the new path.

  Rejected: reusing `*fs.PathError` with a "parse" Op. It would hit the read-class branch and
  get `Fix = readClassFix`.
- **The old fallback cannot be reached from any current call site.** Every other
  `newProblem` caller (assemble.go 207/246, status.go 125/131, check.go 231/294/358) passes an
  error from open, read or readDir, which is always an `*fs.PathError`. The fallback stays as
  a default for an error it cannot classify: `Path = base`, the trimmed detail, and
  `Fix = readClassFix`, because the frontmatter copy no longer describes anything. Because no
  fixture can reach it, the change is **deliberately not red-tested**, and the doc comment
  says so.
- **Relativizing is only a rendering concern.** `Problem.Path`, `RefusalError.Path` and
  `Shortfall.Path` stay absolute inside `assemble`, because S07's `status --json` needs an
  absolute `problem.path`. `internal/cli` relativizes when it renders text.
- **One helper, `displayPath(wd, p)`, in `internal/cli/refusal.go` beside `jsonPath`.** It
  uses the same guards as `jsonPath`:
  - `""` and `"<stdin>"` stay unchanged.
  - A path that is already relative stays unchanged. finish rewrites the path to the user's
    own `--state`/`--handoff` arg, so this case happens.
  - An absolute path goes through `filepath.Rel(wd, p)`, falling back to `p` on error (R6).
    **A `..` chain is accepted.** A path outside wd renders as `../…`. The subdirectory
    contract below needs this. As a result, finish's cap refusals, whose path is the user's
    absolute `--handoff`/`--state` temp file, may render a traversal chain. That is
    acceptable until S11 redesigns finish's text.
- **All refusal text goes relative, not just start's.** Start's refusal renders through
  `refusalClassification.textLine()`, which every command shares. So `textLine` takes `wd`,
  and every command's refusal text line becomes relative. As a result, the JSON `message`
  (which by contract is the text line, byte for byte) is relative, while JSON `path` stays
  absolute. That fits R3 + R6.
- **The rest of start's text also goes relative now:** the shortfall lines, the
  "feature is complete" notice and the "no step files yet" notice. STATE.md assigned these to
  "S04/05". S04 now owns them, so after this scenario start is fully relative.
- **Status line frame stays `brief status: <path>: <detail>; <fix>`.** Only `<path>` changes:
  it becomes the relative step file, and `<fix>` becomes the new text. S06 changes the frame
  to `brief status: <feature>: <rel path>: …` together with the table and the summary line
  that give it context, so adopting that frame here would make S06 rewrite the same assertion
  twice.

## User-visible contract after this scenario

For a feature `delta` whose step file `SCENARIO-01.md` has no frontmatter, run from the repo
root:

- `brief status`: stdout keeps today's row. stderr gets
  `brief status: docs/specifications/delta/SCENARIO-01.md: no frontmatter found; run 'brief check delta' to list every fault`.
  Exit 0.
- `brief start delta`: stdout is empty. stderr gets
  `brief start: docs/specifications/delta/SCENARIO-01.md: no frontmatter found; run 'brief check delta' to list every fault`.
  Exit 1.
- `brief start delta --json`: stdout is one document with `error.kind "refusal"`. `path` is
  the **absolute** step-file path, `message` is the relative text line above, `problem` is
  `no frontmatter found`, `fix` is the check fix, and `line` is null. stderr is empty. Exit 1.
- From a subdirectory of a repo that has `.brief.yaml` at its root, the text path becomes
  `../docs/specifications/…`.

## Implementation Plan

Verification commands are the standing ones in `.claude/rules/agent-briefs.md`.

- [x] Step 1: `internal/assemble/status_test.go` `Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse`: rewrite the assertions. `Problem.Path` becomes the step file, `Problem.Fix` becomes the exact check-fix string (replacing the vacuous `NotEmpty`), and `Detail` stays `no frontmatter found`, so only one variable changes (red)
- [x] Step 2: `internal/assemble/status_test.go` `Test_status_reports_one_problem_per_feature_not_one_per_file`: add an assertion that the named file is the first bad file in sorted order (red)
- [x] Step 3: `internal/assemble/assemble_test.go` `Test_start_names_the_step_file_whose_frontmatter_cannot_be_read`: a table test covering no frontmatter, unclosed delimiter and bad YAML. Include a row where STEP-01 is well-formed and STEP-02 is bad, and expect STEP-02 named, which proves the name is not hard-coded to the first file. Each row asserts `RefusalError.Path`, the exact `Fix`, and that `errors.Is(ErrNoFrontmatter)` is kept where it applies (red)
- [x] Step 4: `internal/assemble/assemble_test.go` `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`: change the `refusal.Path` expectation from `featureDir` to the step file. Do **not** fold it into Step 3's table, because its comment records why tolerance lives in `featureStatus` and not in `readSteps` (update)
- [x] Step 5: `internal/assemble/assemble.go` `readSteps`: wrap each `ParseFrontmatter` failure in the new typed error carrying `e.Name()`. Leave the ReadFile wrap unchanged (green)
- [x] Step 6: `internal/assemble/errors.go`: add the unexported typed step-file parse error (`Error`, `Unwrap`), and add a `newProblem` branch for it. The unreachable fallback's Fix becomes `readClassFix`. It is deliberately not red-tested (see Decisions), so do not build a fixture for it (green)
- [x] Step 7: `internal/assemble/errors.go` and `internal/assemble/doc.go`: rewrite the doc comments on `newProblem`, the new type and `Problem` so they state the rule (a step-file parse failure names the step file and points at check) with no history and no scenario ids. Check that doc.go's Start/Status paragraphs still hold (update)
- [x] Step 8: `internal/cli/status_test.go` `Test_status_names_the_reason_for_a_malformed_feature_on_stderr`: rewrite it to expect the exact relative step-file line above, keeping today's frame (red)
- [x] Step 9: `internal/cli/start_test.go` `Test_start_names_the_malformed_step_file_relative_to_the_working_directory`: a test through `cli.Run`. It checks the exact text line plus exit 1. It also has a `--json` arm on the same fixture: absolute `path`, relative `message`, check `fix`, empty stderr. It also has a row run from a subdirectory with `.brief.yaml` at the root, expecting a `../`-prefixed text path (red)
- [x] Step 10: `internal/cli/refusal.go` `displayPath`: the pure relativize helper, using the same guards as `jsonPath` (new)
- [x] Step 11: `internal/cli/refusal.go` `refusalClassification.textLine`: take `wd` and render the path through `displayPath`, and have `(reporter) refusal` pass `r.wd`. `jsonPath` stays absolute (green)
- [x] Step 12: `internal/cli/status.go` `runStatus`: render `row.Problem.Path` through `displayPath(wd, …)` in the stderr loop. `statusLong` still says the old wording, but S06 and S14 own help copy, so leave it (green)
- [x] Step 13: `internal/cli/start.go` `runStart`: render the shortfall lines and both nil-Step notices through `displayPath` (update)
- [x] Step 14: rewrite every existing test that pins an absolute path in refusal or notice text. Drive this from `grep -rn 'filepath.Join(wd\|filepath.Join(root\|filepath.Join(featureDir\|Sprintf("brief' internal/cli/*_test.go`, and rewrite each hit that ends up in stderr or in `message`. At minimum that is:
  - `internal/cli/start_test.go` (lines ~138, 178–184, 302, 331, 526)
  - `internal/cli/run_test.go` (~57)
  - `internal/cli/finish_test.go` (~263, 308, 350). A user temp-file path outside wd renders as a `../` chain; expect that on purpose and derive it with `filepath.Rel` from the fixture. Do not add a guard.
  - the golden `message` in `internal/cli/json_refusal_test.go` (~115). `path` stays absolute.

  Also check `check_test.go`, `check_drift_test.go`, `new_step_test.go` and `exit_test.go`. The json_refusal matrix's message-equals-text check stays self-consistent (update)
- [x] Step 15: mutation-verify each change individually, stashing per agent-briefs (use a tagged `git stash push` and apply it by SHA, because the stash stack is shared):
  - (a) In `readSteps`, pass the feature dir instead of `e.Name()`, or have `newProblem`'s new branch skip the join. Step 1's and Step 3's Path assertions should go red.
  - (b) Take the feature name for Fix from the joined step-file path instead of `base`. Step 1's and Step 3's Fix assertions should go red.
  - (c) Make `displayPath` return `p` unchanged. Steps 8 and 9 should go red.
  - (d) Make `jsonPath` use `displayPath`. Step 9's JSON `path` assertion should go red.

  Report which test each mutation turned red.
- [x] Step 16: `go build ./...`, `go test ./...` (report the count and the delta), `go test -race ./internal/assemble/... ./internal/cli/...`, `golangci-lint run ./...`. Then mark SCENARIO-04 done in `specification.md` and fold the Handoff into `STATE.md` (update)

## Handoff

**Binding decisions:** a later scenario must not contradict these without saying so.
- A step-file frontmatter parse failure (no frontmatter, unclosed delimiter, bad YAML) is
  reported with `Path` = the step file's absolute path, `Detail` = the parser's own message,
  and `Fix` = `run 'brief check <feature>' to list every fault`. S06's malformed line and
  S07's `problem` object render these fields as they are.
- The feature name in that Fix comes from the feature dir (`base`), never from the file path.
  Otherwise the output reads `brief check SCENARIO-01.md`.
- A read failure (`*fs.PathError`) keeps `readClassFix` (`make it readable and re-run`).
  Only parse failures point at check. status_test pins `readClassFix` in three places.
- `assemble` paths stay absolute. Text rendering in `internal/cli` relativizes them through
  `displayPath(wd, p)` in `refusal.go`: `""`, `<stdin>` and relative paths pass through
  unchanged, and an absolute path goes through `filepath.Rel` with an absolute fallback.
  S06, S08, S10 and S11 use this helper for their text paths, and S07/S09 use `jsonPath`
  (absolute). Do not add a second relativizer.
- Every command's refusal text line is now relative, because `textLine(wd)` is the shared
  seam. So the JSON `message` is relative while JSON `path` is absolute. That split is
  intended, not a bug.
- Start's text output is fully relative: refusal, shortfall lines and both nil-Step notices.

**Left unbuilt:** named so nobody assumes it exists.
- Status stderr frame `brief status: <feature>: <rel path>: …` and the summary line: S06.
  Today's frame is `brief status: <rel path>: <detail>; <fix>`.
- `statusLong` help prose still describes the `! ! !` row and the old wording: S06/S14.
- `check` finding paths are still absolute in text (`RenderFindings`): S08.
- `new` stdout path and `new`/`finish` success stderr are still absolute: S10/S11.
- Not-found refusal naming the directory and the known features: S05.

**Traps:** things that look right and are not.
- Wrapping the parse failure as an `*fs.PathError` looks like it would name the file for
  free. It routes into the read-class branch and silently changes Fix to `readClassFix`.
- `assert.NotEmpty` on `Problem.Fix` (old status_test) passes with a wrong feature name in
  the Fix. Pin the exact string.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`. finish's refusal path can
  be the user's relative `--state`/`--handoff` arg, and calling `Rel` on it computes a path
  relative to the wrong base.
- `displayPath` accepts `..` chains, which the subdirectory case needs. A "no `..`" guard
  added to keep finish's temp-file paths tidy would break the subdirectory contract.
- The `newProblem` fallback cannot be reached from any current caller. A test that seems to
  cover it is testing a fake error shape, not a real path.
- Tests that assert `Contains(stderr, <absolute path>)` go red once text is relative. That
  is expected churn, not a regression. Rewrite them to the relative form, and do not switch
  them to the JSON path.
