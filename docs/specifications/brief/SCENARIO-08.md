# SCENARIO-08: Creating a feature that already exists is refused

## Scenario

```gherkin
Scenario: SCENARIO-08 Creating a feature that already exists is refused  [orig: new]
  Given an existing conforming feature
  When I create a feature of the same name
  Then the command is refused naming the existing directory
  And every file in that directory is byte-identical
```

## Decisions this scenario takes (read before step 1)

**What is already green — probe-verified, do not manufacture a red for it.** A probe binary
built from this worktree, run twice in a temp project (`brief new feature alpha`, then
`brief new step alpha`, then `brief new feature alpha` again), gives today:

```
brief new feature: scaffold: mkdirat alpha: file exists
exit=1
```

and all three files in `docs/specifications/alpha` keep their checksums and their mtimes.
So **exit 1 is already correct** and **byte-identity already holds** — `root.Mkdir` fails
before any write, and `writeExclusive`'s `O_CREATE|O_EXCL` is a second guard behind it. The
one thing the scenario asks for that is missing is the refusal itself: it is a raw wrapped
`mkdirat` OS error that names neither the existing directory nor the fix, and carries no
sentinel a caller can branch on. That is the whole production delta.

**Nothing above line 422 governs this refusal.** The Triage Brief's binding "already exists —
do not re-plan: nothing" clause covers the pre-crossover empty module, and the Product Verdict's
eight SHIP-WITH-CHANGES items name SCENARIO-07 (item 6) but say nothing about an already-existing
feature, no `--force`, and no exit code for it. The copy below conforms to R14a's one-line
refusal template, which `renderRefusal` already implements.

**The refusal class is inherited, not decided here.** STATE.md and SCENARIO-07's handoff
already bind it: the already-exists refusal is a `*RefusalError` at **exit 1**, the opposite
class from `ErrInvalidFeatureName`'s bare sentinel → `usageError` → exit 2. Do not unify the
two paths in `cli/new.go`.

**The exact user-visible contract** (default config, working directory `wd`):

```
$ brief new feature payments          # docs/specifications/payments already exists
stdout: (empty)
stderr: brief new feature: <wd>/docs/specifications/payments: feature already exists; run 'brief new step payments' to add a step to it, or choose a different name (no files changed)
exit:   1
```

The `RefusalError` fields that produce it: `Path` = `filepath.Join(featureRoot, name)`,
`Line` = 0, `Problem` = `"feature already exists"`, `Fix` =
`"run 'brief new step <name>' to add a step to it, or choose a different name"`,
`Err` = the new sentinel `ErrFeatureExists`.

**`internal/cli` needs zero production change.** `runNewFeature`'s `ErrInvalidFeatureName`
branch does not match, and `renderRefusal` already renders any `*scaffold.RefusalError` into
that line and returns the error unchanged, so `ExitCode` gives 1. Add the cli *test*; do not
"fix" `new.go`.

**Detection point and predicate scope.** Keep `root.Mkdir` as the sole choke point (no
pre-`Stat`: it adds a syscall and a TOCTOU window for nothing). Map **only**
`errors.Is(err, fs.ErrExist)` to the refusal; every other `Mkdir` failure keeps its existing
`fmt.Errorf("scaffold: %w", err)` wrapping. An over-broad predicate would relabel a traversal
refusal (`"../escaped"`) as "feature already exists".

**Scope calls — do not gold-plate.**

- A directory that exists but is empty or malformed is refused identically. `NewFeature`
  never inspects the contents of what it finds; "conforming" in the Gherkin describes the
  fixture, not a condition to test for. SCENARIO-11 owns malformed features, for `status`.
- A plain *file* colliding with the name also yields `EEXIST` and so gets the same refusal
  wording. Degenerate; the `Path` names exactly what collided, which is enough.
- The refusal names the **requested** spelling. On a case-insensitive filesystem (this
  repo's dev platform) `Widgets` collides with `widgets` and the refusal names `Widgets`;
  no stat-and-resolve of the on-disk casing. **Do not add a case-fold test** — it would pass
  on APFS and fail on Linux CI. STATE.md parks case-fold collisions as unowned.

**Vacuity trap, the inverse of SCENARIO-07's.** After the refusal, `DirExists(featureDir)`
proves nothing — the directory existed before the call. The discriminating probe is
snapshot-map equality over the feature directory, which also catches an added temp file.
`snapshotTree` already exists in `package scaffold_test` (`internal/scaffold/finish_test.go`,
line 18): reuse it in place, do not move or duplicate it. It is non-recursive, which is
exactly right for a flat feature directory.

**Proof standard for the byte-identity claim.** Byte-identity is protected by two independent
guards (`Mkdir`, then `writeExclusive`'s `O_EXCL`), so it is **not individually falsifiable
through `NewFeature`**: relaxing the `Mkdir` leaf guard alone still leaves the specification
untruncated. Step 8 states the predicted outcome of each mutation, and the control arm in
step 5 is what proves the snapshot probe discriminates at all. Never disable both guards at
once to force a red — the standing brief forbids it, and it would prove nothing about either.

## Implementation Plan

- [x] Step 1: `internal/scaffold/scaffold_test.go` `Test_refuses_an_existing_feature_naming_its_directory` — fixture: `NewFeature("widgets")` then `NewStep("widgets")` on a `t.TempDir()` root with `fixtureConfig()`, then a second `NewFeature("widgets")`. Assert `errors.As` gives a `*scaffold.RefusalError`, its `Path` equals `filepath.Join(root, "specs", "widgets")`, `Line` is 0, and `errors.Is(err, scaffold.ErrFeatureExists)` (red — the sentinel does not exist and the error is a wrapped `mkdirat`)
- [x] Step 2: `internal/scaffold/errors.go` `ErrFeatureExists` — add the sentinel with a doc comment stating when it is returned and that it travels inside a `*RefusalError` (new)
- [x] Step 3: `internal/scaffold/scaffold.go` `(*Server).NewFeature` — on `root.Mkdir` failing with `errors.Is(err, fs.ErrExist)`, return the `*RefusalError` pinned in *Decisions*; every other `Mkdir` error keeps its current wrapping (green)
- [x] Step 4: `internal/scaffold/scaffold_test.go` `Test_leaves_an_existing_features_files_byte_identical_when_it_refuses` — same fixture as step 1; `snapshotTree` over the feature directory before and after the refused call, `assert.Equal` on the two maps (expected **green on arrival** — say so in the report; map equality also asserts no temp file appeared) (new)
- [x] Step 5: `internal/scaffold/scaffold_test.go` `Test_the_byte_identity_probe_sees_a_change_when_the_scaffold_writes_one` — control arm for step 4: same fixture, same probe, same directory, one variable changed (call `NewStep` instead of the refused `NewFeature`); `assert.NotEqual` on the two snapshots (new)
- [x] Step 6: `internal/scaffold/scaffold_test.go` `Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists` — strengthen `require.Error` to `require.ErrorIs(t, err, scaffold.ErrFeatureExists)` so it pins the class, not merely "an error" (update)
- [x] Step 7: `internal/scaffold/scaffold_test.go` `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root` — add `require.NotErrorIs(t, err, scaffold.ErrFeatureExists)` (golangci-lint's testifylint require-error rule forces `require` over `assert` here): a traversal refusal must never be relabelled as an existing feature (update)
- [x] Step 8: mutation-verify, one at a time, stashing each per the standing brief. Both forms below compile — (a) leaves `io/fs` still used by `NewStep`'s `fs.ReadDir(root.FS(), ".")`, and `os.Root.MkdirAll` exists on the pinned 1.27.1. (a) drop the `fs.ErrExist` mapping added in step 3, returning the wrapped error as before → **step 1 red, steps 4–5 stay green** (this is the mutation that proves S08's production change); (b) relax the leaf guard by swapping `root.Mkdir(name, 0o755)` for `root.MkdirAll(name, 0o755)` → it succeeds on the existing directory and the call then dies in `writeExclusive` with `write widgets/SPEC.md: openat …: file exists`, so **step 1 red, step 4 still green** — report whether that prediction held. Report which mutation reddened which test
- [x] Step 9: `internal/cli/run_test.go` `Test_refuses_on_one_line_when_the_feature_already_exists` — `cli.Run` creates `payments`, then runs `new feature payments` again; assert `require.ErrorIs(err, scaffold.ErrFeatureExists)`, `cli.ExitCode(err) == 1`, stdout empty, and `oneLine(t, &stderr)` equals the exact line pinned in *Decisions*. **Green on arrival** at this position, and correctly so: there is no cli production change behind it — it pins the line and exit code steps 2–3 produce through the existing `renderRefusal`. Do not hoist it earlier to manufacture a red; the `new_step_test.go` refusal tests are the shape to copy (new)
- [x] Step 10: `internal/scaffold/doc.go` — add `ErrFeatureExists` to the package comment's list of sentinels a `*RefusalError` wraps (update)
- [x] Step 11: `internal/scaffold/scaffold.go` — update `NewFeature`'s doc comment so its existing "refuses an existing feature outright" claim names the sentinel and the refusal shape, and still records that `writeExclusive`'s `O_EXCL` is the second guard. Contract only, no history narrative; verify with `go doc ./internal/scaffold` and `go doc ./internal/scaffold Server.NewFeature` (update)
- [x] Step 12: verification per the standing brief (`go build`, unpiped `go test ./...` from the repo root, `go test -race` on `./internal/scaffold/... ./internal/cli/...`, `golangci-lint run ./...`); report the exact test count and the delta
- [x] Step 13: `docs/specifications/brief/specification.md` — hand-tick `- [ ] SCENARIO-08:` to `- [x]` in the post-crossover progress list (~line 824). **Do not** self-invoke `brief finish`: these files carry no frontmatter and STATE.md assigns that migration to the crossover (update)
- [x] Step 14: `docs/specifications/brief/SCENARIO-08-HANDOFF.md` — write the handoff file from the section below plus what the mutations actually showed (new)
- [x] Step 15: `docs/specifications/brief/STATE.md` — rewrite per the rolling-STATE convention: move the already-exists entry out of *Left unbuilt* into *Binding decisions*, fold in this scenario's traps (update)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- An existing feature is refused as a `*RefusalError` wrapping `scaffold.ErrFeatureExists`,
  rendered by `renderRefusal` at **exit 1** with the `(no files changed)` tail — the opposite
  class from `ErrInvalidFeatureName` (bare sentinel → `usageError` → exit 2), in the same
  function. Unifying the two paths silently changes SCENARIO-07's exit code.
- `root.Mkdir` is the sole detection point, and **only** `errors.Is(err, fs.ErrExist)` maps to
  that refusal — no pre-`Stat` (TOCTOU), and no broadening, or the `"../escaped"` traversal
  refusal gets relabelled "feature already exists" (`Test_returns_an_error_when_the_feature_
  name_escapes_the_feature_root` asserts `NotErrorIs` against exactly that).
- The refusal's `Path` is the **requested** spelling, `filepath.Join(featureRoot, name)` — no
  stat-and-resolve of the on-disk casing. On a case-insensitive filesystem the named path can
  therefore differ in case from the directory that actually collided.
- `NewFeature` never inspects what it found: empty, partial and malformed existing directories
  all get the identical refusal. SCENARIO-11 owns malformed-feature reporting, for `status`.
- `internal/cli` carries **no** production change for this scenario; `renderRefusal` already
  covers it.

**Left unbuilt** — named so nobody assumes it exists:

- Any distinction between an existing *directory* and an existing plain *file* at the feature
  path — both render "feature already exists"; the `Path` is what distinguishes them.
- Case-fold collision detection, `.`/`..` as a whole name, length caps — unowned, as in S07.
- A `--force` / overwrite path. Nothing in R13/R17/R18 asks for one; adding it would undo the
  guard this scenario hardens.

**Traps** — things that look right and are not:

- **Byte-identity is double-guarded and so is not individually falsifiable through
  `NewFeature`.** Relaxing `Mkdir`'s leaf guard leaves the specification untruncated anyway —
  `writeExclusive`'s `O_CREATE|O_EXCL` returns `write widgets/SPEC.md: openat …: file exists`.
  The snapshot test stays green under that mutation; that is expected, not a broken test. Never
  disable both guards at once to force a red.
- `DirExists(<feature dir>/<name>)` after this refusal is **vacuous** — the directory existed
  before the call. Snapshot-map equality over the feature directory is the discriminating
  probe, and it catches an added temp file too.
- `snapshotTree` lives in `internal/scaffold/finish_test.go` in `package scaffold_test` and is
  non-recursive (regular files *directly* under the directory). Reuse it in place; a second
  copy in the same package will not compile.
- Today's refusal already exits 1 and already leaves the tree byte-identical (probe-verified).
  The only real defect is the message and the missing sentinel — do not manufacture a red for
  the parts that arrive green.
