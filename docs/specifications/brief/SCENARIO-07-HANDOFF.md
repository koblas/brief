# SCENARIO-07 — Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Feature-name validation lives in `scaffold.NewFeature` as its first statement, before
  `os.MkdirAll(featureRoot)` — "no directory is created" covers the configured feature directory
  itself, not just the leaf, and a guard placed after `MkdirAll` makes SCENARIO-07 false while
  every other test stays green. Mutation-verified: moving the guard after `MkdirAll` only
  reddens the `docs/specifications`/`<root>/specs` absence assertions, nothing else.
- An invalid feature name is `scaffold.ErrInvalidFeatureName`, a bare sentinel — **not** a
  `*RefusalError` — and `cli/new.go` maps it through `usageError` to `ErrUsage` → **exit 2**,
  with no `(no files changed)` tail. **SCENARIO-08's already-exists refusal lands in the same
  function and is the opposite class**: a `*RefusalError` rendered by `renderRefusal` at exit 1.
  Two refusals, one function, two exit classes — do not unify them into `renderRefusal`, or S07's
  exit code silently becomes 1.
- The predicate is `unicode.IsSpace` over the whole name, plus empty. It is `strings.Fields`'
  own predicate, which is what makes the `status` four-field contract (SCENARIO-09) hold; NBSP
  and NEL are refused on purpose, because `awk` would not split on them where Go does.
  Mutation-verified: narrowing the predicate to `r == '\n'` reddens every non-LF whitespace test
  individually (leading, trailing, tab, CR, NBSP, interior space, plus the cli whitespace test)
  while the LF test stays green — the predicate is exercised per-form, not just as a group.
- `cli/new.go`'s `runNewFeature` branches once on `errors.Is(err, scaffold.ErrInvalidFeatureName)`
  and then only distinguishes empty vs whitespace to pick which of the two pinned message
  templates to print — it does not re-implement whitespace detection. Deleting the empty-name
  branch in `validateFeatureName` (a plain comparison, no import) reddens exactly the empty-name
  tests in both `scaffold` and `cli` — the raw `mkdirat : empty path` OS error surfaces instead,
  proving nothing in the OS or `flag` package does this job for a guard-less path.
- The name constraint is stated in `newFeatureUsage`'s help text; `product-vision`'s final pass
  reviews that copy.
- `%q` in the cli error copy is load-bearing for R14: a name containing a newline renders the
  name escaped (`"pay\nments"`) so the refusal stays on one stderr line — verified by
  `Test_keeps_the_refusal_on_one_line_when_the_feature_name_contains_a_newline`.

**Left unbuilt** — named so nobody assumes it exists:

- Already-exists refusal in `NewFeature` (still a raw `os.Mkdir` `EEXIST` wrapped as
  `scaffold: …`) — SCENARIO-08.
- Non-whitespace control characters, leading `-`, dot-prefixed names, `.`/`..` as a whole name,
  case-fold collisions with an existing feature, and any length cap on a name — unowned; none is
  reachable from R13/R17/R18 as written.
- `NewStep` does **not** validate its `feature` argument against `ErrInvalidFeatureName`; an
  already-created bad-name directory is still addressable. Deliberate — creation is the choke
  point.
- No validation on `assemble`'s read path: a bad-name directory planted by hand still appears to
  `status` (SCENARIO-11's malformed-feature `!` line is the intended backstop).

**Traps** — things that look right and are not:

- `fs.Args()` on `[]string{""}` has **len 1**, so an empty name sails past `cli`'s
  `len(rest) == 0` guard and reaches `NewFeature`. "No name given" and "empty name" are two
  different messages on two different paths; do not collapse them.
- `renderRefusal` returns its error **unchanged**, so anything reaching it exits 1. Reaching for
  it is how this scenario's exit code gets lost. `runNewFeature` must branch on
  `ErrInvalidFeatureName` before calling `renderRefusal`, not inside it.
- Asserting only `NoDirExists(<feature dir>/<name>)` passes vacuously — the leaf never exists in
  a fresh `t.TempDir()` regardless of the guard. The assertion that discriminates is
  `NoDirExists` on the **configured feature directory** (`docs/specifications`), paired with the
  control arm (`Test_creates_the_feature_directory_under_the_configured_feature_directory` in
  `scaffold`, `Test_creates_the_feature_and_prints_its_path` in `cli`) proving the same probe
  sees the directory for an accepted name.
- A name containing whitespace is a legal directory name on every platform `brief` targets, so
  no OS error will do this job for you — nothing fails without the guard; the scaffold succeeds.

## What was built

- `internal/scaffold/errors.go`: `ErrInvalidFeatureName` sentinel.
- `internal/scaffold/scaffold.go`: unexported `validateFeatureName(name string) error`, called as
  `NewFeature`'s first statement; doc comment on `NewFeature` updated.
- `internal/scaffold/doc.go`: one new paragraph naming the bare-sentinel exception to the
  `*RefusalError` convention.
- `internal/scaffold/scaffold_test.go`: 9 new tests (interior/leading/trailing space, tab, LF,
  CR, NBSP, empty name, and the "creates nothing at all" absence test), plus a control-arm
  `DirExists(<root>/specs)` assertion added to the existing
  `Test_creates_the_feature_directory_under_the_configured_feature_directory`.
- `internal/cli/new.go`: `runNewFeature` branches on `errors.Is(err, scaffold.ErrInvalidFeatureName)`
  before `renderRefusal`, routing through `usageError` with the two pinned message templates;
  `newFeatureUsage` gained one line stating the name constraint.
- `internal/cli/run_test.go`: 3 new tests (whitespace usage error, empty-name usage error,
  newline-stays-one-line), plus two `DirExists` control-arm assertions added to
  `Test_creates_the_feature_and_prints_its_path`.

## Verification

- `go build ./...` — exit 0.
- `go test ./...` — exit 0, all packages `ok`.
- `go test -v ./...` — 263 `--- PASS`, 0 `--- FAIL`, 0 `--- SKIP` (delta: +12 tests over the
  251 that existed before this scenario — 9 in `scaffold`, 3 in `cli`).
- `go test -race ./internal/scaffold/... ./internal/cli/...` — exit 0.
- `golangci-lint run ./...` — 0 issues.
- Mutation-verified individually (each backed up to `$TMPDIR`, restored, diffed byte-identical
  after): (a) narrowed `unicode.IsSpace` to `r == '\n'` — reddened every non-LF whitespace form
  test plus the cli whitespace test, LF test stayed green; (b) deleted the empty-name branch —
  reddened both empty-name tests (scaffold and cli), surfacing the raw `mkdirat : empty path` OS
  error; (c) moved the guard after `os.MkdirAll` — reddened only the configured-feature-directory
  absence assertions (`scaffold`'s "creates nothing" test and both cli whitespace/empty tests),
  nothing else.
