---
id: SCENARIO-07
status: done
depends-on: []
---

# SCENARIO-07: A feature name that would break the status contract is refused

## Scenario

```gherkin
Scenario: SCENARIO-07 A feature name that would break the status contract is refused  [orig: new]
  Given a project using the shipped default profile
  When I create a feature whose name contains whitespace
  Then the command is refused with a usage exit
  And no directory is created
  # status is a whitespace-separated four-field contract; it dies the moment a name has a
  # space in it. Rejecting at creation is what makes that contract hold.
```

## Decisions this scenario takes (read before step 1)

- **The guard lives in `internal/scaffold`, the exit class in `internal/cli`.** One predicate,
  one call site. `NewFeature` refuses; `cli/new.go` only branches on the exported sentinel and
  routes through the existing `usageError` helper. No second check in `cli` (CLAUDE.md: no logic
  under `internal/cli`), and no new exit code (`ExitCode` already maps `ErrUsage` → 2).
- **Exit 2, not 1.** The Gherkin says "usage exit"; *Decisions taken* 6 in `specification.md`
  ("Whitespace in feature names is rejected at creation, exit 2 — SCENARIO-07") pins it. R14a's
  "refusals exit 1" parenthetical governs write refusals (S08's already-exists); this one is an
  invocation defect.
- **A plain sentinel, not a `*RefusalError`.** `renderRefusal` renders every
  `*scaffold.RefusalError` with the `(no files changed)` tail and returns it unchanged → exit 1.
  A `*RefusalError` here would need special-casing before `renderRefusal`, and its
  `Path`/`Problem`/`Fix` would be built and discarded. All five existing exit-2 lines in `cli`
  use the bare usage shape with no tail; match them.
- **Predicate: any `unicode.IsSpace` rune anywhere in the name, plus the empty name.**
  `unicode.IsSpace` is exactly what `strings.Fields` uses, and `strings.Fields` is the four-field
  `status` contract's real consumer — space, tab, newline, CR, VT, FF, NEL (U+0085), NBSP
  (U+00A0) and the Unicode Z categories, leading, trailing and interior alike, one rule. Empty is
  a separate branch of the same guard (empty is not whitespace) and the same sentinel: an empty
  first field breaks the four-field contract identically, and today `""` reaches `root.Mkdir("")`
  and surfaces a raw `scaffold: mkdir : ...` at exit 1. `flag`'s `fs.Args()` on `[]string{""}`
  has len 1, so `""` passes `cli`'s `len(rest) == 0` check and does reach `name`.
- **Ordering is load-bearing.** `NewFeature`'s current first statement is
  `os.MkdirAll(featureRoot, 0o755)`. The guard must become the new first statement — after it,
  "no directory is created" is false, because `docs/specifications/` itself appears.
- **Out of scope, deliberately** (do not gold-plate): already-exists (SCENARIO-08, a
  `*RefusalError` at exit 1), non-whitespace control characters, leading `-`, dot-prefixed names,
  length caps (R13/R17/R18). **Traversal is already covered** and needs no step —
  `scaffold_test.go` `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root`
  pins `../x` through `root.Mkdir`, and STATE.md's traversal-vs-`ErrNoSuchFeature` note concerns
  `NewStep`, not `NewFeature`.

**The exact stderr line** (one line, exit 2, stdout empty, no `(no files changed)` tail):

```
brief new feature: name "pay ments" contains whitespace; run 'brief new feature <name>' with a name containing no whitespace
```

and for the empty name:

```
brief new feature: name is empty; run 'brief new feature <name>' with a non-empty name
```

`%q` in that copy is load-bearing: a name containing a newline must still produce one stderr
line (R14).

Verification commands and the mutation/stash protocol are in `.claude/rules/agent-briefs.md`.
Doc comments follow the `clean-architecture` rules: state the rule, cite the R-number where the
package already does, and keep scenario IDs out of the comment text.

## Implementation Plan

- [x] Step 1: `internal/scaffold/scaffold_test.go` `Test_refuses_a_feature_name_containing_whitespace` — `NewFeature` through the fixture config returns an error satisfying `errors.Is(err, scaffold.ErrInvalidFeatureName)` for an interior space, plus one `assert.ErrorContains` on the `%q`-quoted offending name so step 7's message text is not dead (red: sentinel does not compile yet)
- [x] Step 2: `internal/scaffold/scaffold_test.go` — one test per remaining whitespace form, each a separate named func, all asserting the same sentinel: leading space, trailing space, tab, LF, CR, NBSP (U+00A0) (red)
- [x] Step 3: `internal/scaffold/scaffold_test.go` `Test_refuses_an_empty_feature_name` — same sentinel for `""` (red)
- [x] Step 4: `internal/scaffold/scaffold_test.go` `Test_creates_nothing_at_all_when_the_name_is_refused` — after a refused call on a fresh `t.TempDir()`, `assert.NoDirExists` on **both** `<root>/specs/<name>` and the configured feature directory `<root>/specs` itself (red)
- [x] Step 5: `internal/scaffold/scaffold_test.go` `Test_creates_the_feature_directory_under_the_configured_feature_directory` (existing, line 38) — the control arm for step 4 already exists: identical fixture, identical call, **only the name differs** (`"widgets"`). Add the explicit `assert.DirExists(<root>/specs)` to it so the step-4 probe is shown to fire for an accepted name. Do **not** write a second test (update)
- [x] Step 6: `internal/scaffold/errors.go` — add `ErrInvalidFeatureName`, documented as the sentinel `NewFeature` returns for a name carrying whitespace or an empty name, and why (`status`'s whitespace-separated four fields) (new)
- [x] Step 7: `internal/scaffold/scaffold.go` `validateFeatureName` — unexported pure predicate over `unicode.IsSpace` + empty, wrapping `ErrInvalidFeatureName` with the offending name via `%q` (green)
- [x] Step 8: `internal/scaffold/scaffold.go` `(*Server).NewFeature` — call `validateFeatureName` as the **first statement**, before `os.MkdirAll` (green)
- [x] Step 9: `internal/scaffold/scaffold.go` — extend `NewFeature`'s doc comment: what it refuses, the sentinel's name, and that the refusal precedes every filesystem call (update)
- [x] Step 10: `internal/scaffold/doc.go` — amend the "a caller-facing refusal is a `*RefusalError`" paragraph to state the one exception: an invalid name is an invocation defect reported as a bare `ErrInvalidFeatureName`, which `cli` classifies as a usage exit, not as a write refusal (update)
- [x] Step 11: `internal/cli/run_test.go` `Test_returns_a_usage_error_when_the_feature_name_contains_whitespace` — `cli.Run` with `[]string{"new", "feature", "pay ments"}` on a bare `t.TempDir()` (shipped default profile, no `.brief.yaml`): `require.ErrorIs(t, err, cli.ErrUsage)`, `assert.Equal(t, 2, cli.ExitCode(err))`, stdout empty, stderr exactly the pinned line via the existing `oneLine` helper, and `assert.NoDirExists` on both `<wd>/docs/specifications/pay ments` and `<wd>/docs/specifications` (red)
- [x] Step 12: `internal/cli/run_test.go` `Test_returns_a_usage_error_when_the_feature_name_is_empty` — same shape for `[]string{"new", "feature", ""}`, pinned empty-name line (red)
- [x] Step 12a: `internal/cli/run_test.go` `Test_keeps_the_refusal_on_one_line_when_the_feature_name_contains_a_newline` — `cli.Run` with a name containing `\n`; `oneLine` proves stderr is still a single line and the name appears escaped. `%q` in the copy is what holds R14's one-line contract here; `%s` would emit two lines (red)
- [x] Step 13: `internal/cli/new.go` `runNewFeature` — on `NewFeature`'s error, branch `errors.Is(err, scaffold.ErrInvalidFeatureName)` to `usageError(stderr, …)` with the pinned copy; everything else still goes to `renderRefusal` (green)
- [x] Step 14: `internal/cli/run_test.go` — control arm for step 11's absence claim: assert the existing `Test_creates_the_feature_and_prints_its_path` fixture (`"payments"`, same `wd` shape, same argv but for the name) leaves `<wd>/docs/specifications` **and** `<wd>/docs/specifications/payments` on disk; add the two `DirExists` assertions to that test rather than writing a second one (update)
- [x] Step 15: `internal/cli/new.go` `newFeatureUsage` — one line stating the name constraint ("A name may not be empty or contain whitespace."); `Test_prints_usage_to_stdout_when_help_is_requested_for_the_subcommand` asserts only `NotEmpty`, so no test changes (update)
- [x] Step 16: mutation-verify, one at a time, stashing each per the standing brief. **Every mutation must still compile** — deleting the `unicode.IsSpace` call would leave the `unicode` import unused, and a build failure is not evidence. (a) **narrow** the predicate from `unicode.IsSpace` to a one-rune equivalent (`func(r rune) bool { return r == '\n' }`) → each space / tab / CR / NBSP / leading / trailing test from steps 1–2 and the cli test in step 11 goes red individually, while the LF test stays green; (b) delete the empty-name branch (a plain comparison, no import) → steps 3 and 12 go red; (c) **move the guard from before to after `os.MkdirAll`** → only the `<root>/specs` / `<wd>/docs/specifications` absence assertions in steps 4 and 11 go red. Report which mutation reddened which test
- [x] Step 17: `go build ./...`, `go test ./...` unpiped from the repo root, `go test -race ./internal/scaffold/... ./internal/cli/...`, `golangci-lint run ./...`; report the exact test count and the delta
- [x] Step 18: `docs/specifications/brief/specification.md` — hand-tick `- [ ] SCENARIO-07:` to `- [x]` in the post-crossover progress list (~line 823). **Do not** self-invoke `brief finish`: the crossover has not landed, the `SCENARIO-0N.md` files carry no frontmatter, and STATE.md assigns that migration to the crossover (update)
- [x] Step 19: `docs/specifications/brief/STATE.md` — rewrite per the rolling-STATE convention, folding in the Handoff below (update)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Feature-name validation lives in `scaffold.NewFeature` as its first statement, before
  `os.MkdirAll(featureRoot)` — "no directory is created" covers the configured feature directory
  itself, not just the leaf, and a guard placed after `MkdirAll` makes SCENARIO-07 false while
  every other test stays green.
- An invalid feature name is `scaffold.ErrInvalidFeatureName`, a bare sentinel — **not** a
  `*RefusalError` — and `cli/new.go` maps it through `usageError` to `ErrUsage` → **exit 2**,
  with no `(no files changed)` tail. **SCENARIO-08's already-exists refusal lands in the same
  function and is the opposite class**: a `*RefusalError` rendered by `renderRefusal` at exit 1.
  Two refusals, one function, two exit classes — do not unify them into `renderRefusal`, or S07's
  exit code silently becomes 1.
- The predicate is `unicode.IsSpace` over the whole name, plus empty. It is `strings.Fields`'
  own predicate, which is what makes the `status` four-field contract (SCENARIO-09) hold; NBSP
  and NEL are refused on purpose, because `awk` would not split on them where Go does.
- The name constraint is stated in `newFeatureUsage`'s help text; `product-vision`'s final pass
  reviews that copy.

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
  it is how this scenario's exit code gets lost.
- Asserting only `NoDirExists(<feature dir>/<name>)` passes vacuously — the leaf never exists in
  a fresh `t.TempDir()` regardless of the guard. The assertion that discriminates is
  `NoDirExists` on the **configured feature directory** (`docs/specifications`), paired with the
  step-5/step-14 control arm proving the same probe sees it for an accepted name.
- A name containing whitespace is a legal directory name on every platform `brief` targets, so
  no OS error will do this job for you — nothing fails without the guard; the scaffold succeeds.
