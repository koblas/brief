# SCENARIO-08 — Handoff

## What shipped

Most of this scenario was already correct behavior before any code changed: a second
`brief new feature alpha` already exited 1 and already left the existing feature's files
byte-identical (double-guarded by `root.Mkdir` and `writeExclusive`'s `O_CREATE|O_EXCL`).
Confirmed green on arrival rather than manufactured red — see Steps 4, 5 and 9 below.

The one real production change: `(*Server).NewFeature` in `internal/scaffold/scaffold.go`
now maps `errors.Is(err, fs.ErrExist)` from `root.Mkdir` to a `*RefusalError` wrapping the
new sentinel `scaffold.ErrFeatureExists`, naming `filepath.Join(featureRoot, name)` at
`Line` 0, with `Fix` = `"run 'brief new step <name>' to add a step to it, or choose a
different name"`. Every other `Mkdir` failure — including the `"../escaped"` traversal case
— keeps its previous plain `fmt.Errorf("scaffold: %w", err)` wrapping.

`internal/cli` got zero production change, as the plan predicted: `runNewFeature`'s
`errors.Is(err, scaffold.ErrInvalidFeatureName)` branch does not match `ErrFeatureExists`,
so it falls through to the existing `renderRefusal`, which already knew how to render any
`*scaffold.RefusalError`.

## Step-by-step

- **Step 1** (RED): `Test_refuses_an_existing_feature_naming_its_directory` — compile-fail
  red (`scaffold.ErrFeatureExists` did not exist).
- **Steps 2–3** (GREEN): added the sentinel in `errors.go`; added the `fs.ErrExist` branch
  in `scaffold.go`. All scaffold tests green after.
- **Steps 4–5**: `Test_leaves_an_existing_features_files_byte_identical_when_it_refuses` and
  its control arm `Test_the_byte_identity_probe_sees_a_change_when_the_scaffold_writes_one`
  — both green on arrival, exactly as predicted; the control arm proves `snapshotTree`
  actually discriminates (`NotEqual` fires when `NewStep` runs instead of the refused
  `NewFeature`).
- **Steps 6–7**: strengthened `Test_does_not_overwrite_an_existing_specification_when_the_
  feature_directory_exists` to `require.ErrorIs(t, err, scaffold.ErrFeatureExists)`, and
  added `require.NotErrorIs(t, err, scaffold.ErrFeatureExists)` to
  `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root`. (Used
  `require.NotErrorIs`, not `assert.NotErrorIs` as the plan's prose said — golangci-lint's
  `testifylint` `require-error` rule forces `require` for error assertions in this repo;
  content and intent are unchanged.)
- **Step 8** — mutation-verified, one at a time, restoring from a `$TMPDIR` copy (not
  `git stash`, per the standing brief) and diffing byte-identical after each restore:
  - **Mutation (a)**: dropped the `fs.ErrExist` mapping (reverted `NewFeature`'s `Mkdir`
    branch to plain wrapping, and removed the now-unused `errors` import). Result:
    `Test_refuses_an_existing_feature_naming_its_directory` went red, **and so did**
    `Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists`
    (step 6's strengthened assertion also pins `ErrorIs(ErrFeatureExists)`, so it reddens
    under the same mutation — the plan's prose named only step 1, but step 6 asserts the
    identical sentinel and reddened for the identical reason). Steps 4–5 (byte-identity)
    stayed green, confirming they do not double as a test for the sentinel/refusal shape.
  - **Mutation (b)**: swapped `root.Mkdir(name, 0o755)` for `root.MkdirAll(name, 0o755)`.
    Predicted outcome held exactly: `Test_refuses_an_existing_feature_naming_its_directory`
    (and, again, the step-6 test) went red with `write widgets/SPEC.md: openat
    widgets/SPEC.md: file exists` — the call now dies in `writeExclusive`'s `O_EXCL` guard,
    one layer past `Mkdir`. Step 4 (byte-identity) stayed green, confirming the plan's trap:
    the two guards are independent, and relaxing only the leaf guard cannot falsify
    byte-identity through `NewFeature`.
  - Both mutations restored and diffed byte-identical against the pre-mutation file before
    proceeding.
- **Step 9**: `Test_refuses_on_one_line_when_the_feature_already_exists` in
  `internal/cli/run_test.go` — green on arrival, no cli production change behind it. Pins
  the exact line: `brief new feature: <wd>/docs/specifications/payments: feature already
  exists; run 'brief new step payments' to add a step to it, or choose a different name (no
  files changed)`, exit code 1, empty stdout.
- **Steps 10–11**: updated `internal/scaffold/doc.go`'s sentinel list and `NewFeature`'s doc
  comment to name `ErrFeatureExists` and the refusal shape, keeping the `O_EXCL`
  second-guard note. Verified with `go doc ./internal/scaffold` and
  `go doc ./internal/scaffold Server.NewFeature`.
- **Step 12**: full verification, unpiped, from the repo root:
  - `go build ./...` — clean.
  - `go test ./...` — all 7 packages with tests pass (`cmd/brief` has none).
  - `go test -v ./...` count: **267 `--- PASS`, 0 `--- FAIL`, 0 `--- SKIP`** — delta **+4**
    from 263 (the three new scaffold tests plus the one new cli test).
  - `go test -race ./internal/scaffold/... ./internal/cli/...` — clean.
  - `golangci-lint run ./...` — one finding on first pass (`testifylint`'s `require-error`
    on the step-7 `assert.NotErrorIs`), fixed by switching to `require.NotErrorIs`; `0
    issues` on the re-run.
- **Step 13**: hand-ticked `docs/specifications/brief/specification.md` line 824,
  `- [ ] SCENARIO-08:` → `- [x] SCENARIO-08:`. No `brief finish` self-invocation.

## Binding decisions — do not contradict without saying so

- An existing feature is refused as a `*RefusalError` wrapping `scaffold.ErrFeatureExists`,
  rendered by `renderRefusal` at **exit 1** with the `(no files changed)` tail — the
  opposite class from `ErrInvalidFeatureName` (bare sentinel → `usageError` → exit 2), in
  the same function (`runNewFeature`). Unifying the two paths would silently change
  SCENARIO-07's exit code.
- `root.Mkdir` is the sole detection point, and **only** `errors.Is(err, fs.ErrExist)` maps
  to the refusal — no pre-`Stat` (would add a TOCTOU window), no broadening (would relabel
  the `"../escaped"` traversal refusal as "feature already exists";
  `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root` asserts
  `NotErrorIs` against exactly that).
- The refusal's `Path` is the **requested** spelling, `filepath.Join(featureRoot, name)` —
  no stat-and-resolve of on-disk casing. On a case-insensitive filesystem the named path can
  differ in case from the directory that actually collided. Deliberately not tested (would
  pass on APFS, fail on Linux CI).
- `NewFeature` never inspects what it found: empty, partial and malformed existing
  directories, and a plain colliding file, all get the identical refusal wording.
  SCENARIO-11 owns malformed-feature reporting, for `status`.
- `internal/cli` carries no production change for this scenario.

## Left unbuilt — named so nobody assumes it exists

- Any distinction between an existing directory and an existing plain file at the feature
  path — both render "feature already exists"; `Path` is what would distinguish them if a
  caller inspected the filesystem itself.
- Case-fold collision detection, `.`/`..` as a whole name, length caps — unowned, as in S07.
- A `--force` / overwrite path — nothing asks for one; adding it would undo this scenario's
  guard.

## Traps

- **Byte-identity is double-guarded and not individually falsifiable through `NewFeature`
  alone** — confirmed by mutation (b): relaxing `Mkdir`'s leaf guard still leaves the
  specification untruncated because `writeExclusive`'s `O_EXCL` catches it one layer down.
  Never disable both guards at once to force a red.
- `DirExists(<feature dir>/<name>)` after this refusal is vacuous — the directory existed
  before the call. `snapshotTree` (in `internal/scaffold/finish_test.go`, `package
  scaffold_test`, non-recursive) is the discriminating probe; it also catches an added temp
  file. Reused in place, not duplicated.
- A test asserting `ErrorIs(err, scaffold.ErrFeatureExists)` on the pre-existing
  `Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists`
  reddens under the *same* mutations as the dedicated new test — they are not independent
  witnesses of the sentinel, they are the same witness twice. Noted here so a future reader
  mutation-testing this area does not expect them to diverge.

## Open debts

Unchanged from STATE.md as of SCENARIO-07 — this scenario closed the "already-exists
refusal" item that was the sole SCENARIO-08 entry under *Left unbuilt* and added no new
debt.
