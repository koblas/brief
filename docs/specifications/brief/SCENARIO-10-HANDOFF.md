# SCENARIO-10 — Handoff

## What landed

- `internal/assemble/status.go` `(*Server).Status`: `os.OpenRoot`'s failure
  is now branched — `errors.Is(err, fs.ErrNotExist)` returns `(nil, nil)`
  (zero features, not an error); every other failure (notably `ENOTDIR`
  when the feature root exists as a regular file) still wraps and
  propagates unchanged. Doc comment's last sentence rewritten to state the
  rule as it now is, checked with `go doc ./internal/assemble Server.Status`.
- `internal/cli/status.go` `runStatus`: after `srv.Status`, `len(rows) == 0`
  writes one line to stderr —
  `brief status: no features found in <cfg.FeatureDirectory>; run 'brief new feature <name>' to create one`
  — and returns `nil` before `RenderStatusText` is called, so nothing goes
  into the stdout renderer for this path. `statusUsage` gained one sentence
  describing the behavior so `--help` matches.
- `internal/cli/status_test.go`: two new `cli.Run` slice tests, one with no
  `docs/specifications` at all, one with it present and empty — both assert
  empty stdout, the exact stderr line, exit 0.
- `internal/assemble/status_test.go`: one new test (`Status` against a
  missing feature root returns zero rows, no error) and one control-arm
  test (feature root exists as a regular file, `Status` still errors).

## Test count

279 → 283 (+4): 2 in `internal/cli/status_test.go`, 2 in
`internal/assemble/status_test.go`. `go test ./...` exit 0 (283 `--- PASS`,
0 `--- SKIP`, 0 `--- FAIL`), `go test -race ./internal/assemble/...
./internal/cli/...` exit 0, `golangci-lint run ./...` 0 issues.

## Green on arrival (measured, not assumed)

Per the architect's plan: fixture B (feature root present, empty) and
fixture D (feature root holding only a regular file) already gave empty
stdout + exit 0 at HEAD (5370c62) — `Status` returns no rows for either
shape and `RenderStatusText` writes nothing for an empty slice. Step 2's
test (fixture B) was red only on the stderr assertion, not on stdout/exit,
confirmed by running it before Step 7 landed. Step 4 (the ENOTDIR control
arm) was green on arrival — no code change makes a regular-file feature
root anything but an error, and it must stay that way.

## Mutations verified (stashed to $TMPDIR, never `git stash`), each alone, restored and diffed byte-identical

- Guard 1 (`internal/assemble/status.go`): widened the `errors.Is(err,
  fs.ErrNotExist)` condition to `errors.Is(err, fs.ErrNotExist) || true`,
  swallowing every `os.OpenRoot` failure. Reddened
  `Test_status_propagates_a_feature_root_that_is_not_a_directory`
  ("An error is expected but got nil"). Restored; `diff` against the
  pre-mutation copy in `$TMPDIR` was empty.
- Guard 2 (`internal/cli/status.go`): moved the stderr `Fprintf` above the
  `len(rows) == 0` check so the notice always fires. Reddened both
  `Test_status_prints_one_line_per_feature_and_nothing_else` and
  `Test_status_prints_one_line_for_a_single_feature` on
  `assert.Empty(t, stderr.String())`. Restored; `diff` empty.

Each mutation was run and restored individually — neither test suite was
touched while both guards were simultaneously disabled.

## Left unbuilt (unchanged from the plan, restated for the next scenario)

- Malformed-feature tolerance, the `!` field, per-feature stderr reasons —
  SCENARIO-11. Binding constraint: because the "no features" notice keys
  on `len(rows) == 0` in `cli`, SCENARIO-11 must emit a row for a malformed
  feature rather than dropping it, or a repository whose only feature is
  malformed prints "no features found" and exits 0, swallowing exactly
  what 11 exists to surface.
- `brief start`'s complete-feature "nothing to return" notice — SCENARIO-12,
  same R14 shape, should match this tone.
- `--json` / `"step": null` — SCENARIO-15. `Status` still returns `nil`,
  never `[]FeatureStatus{}`, for zero features — unchanged by this
  scenario, load-bearing for 15's `null` marshaling.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet` — not built.

## Traps for the next scenario

- A typo'd `feature-directory` config value now exits 0, printing "no
  features found in `<typo>`" — this is the R14 shape working as intended;
  do not "fix" it into a refusal without reopening this decision.
- `errors.Is(err, fs.ErrNotExist)` — never a string match. `ENOTDIR`'s
  message ("not a directory") differs from ENOENT's and would slip through
  a substring guard.
- A feature directory with zero step files is conforming (`0/0 - 0`, per
  SCENARIO-09), not "no features" — only a root with zero *directories*
  (or only non-directory entries) triggers the notice.
- `Status`'s doc comment now states the rule correctly; keep `go doc
  ./internal/assemble Server.Status` in sync if this branches again.

## Ticked by hand, not through `brief finish`

`docs/specifications/brief/specification.md`'s SCENARIO-10 line, this
handoff, and `STATE.md` were written by hand — no `SCENARIO-NN.md` in this
tree carries YAML frontmatter, so `brief finish` cannot resolve the step.
Same route SCENARIO-07, 08 and 09 took.
