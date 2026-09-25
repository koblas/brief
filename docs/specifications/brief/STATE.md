# brief — current state

All scenarios complete: SCENARIO-01..22, plus two fix passes (six MAJORs from the first
`/run-reviewers` round, then a second pass closing a MAJOR the first pass introduced plus a
MINOR path-separator regression) folded into the decisions below. No scenario was added.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `stepfile.Frontmatter.Done()` is the sole doneness authority everywhere.
- Handoff lives in its own file (`stepPattern.ID(n) + cfg.HandoffFileSuffix`), never
  spliced into the step file. `markdown.Section`/`FirstUnchecked`/`UnterminatedFence`/
  `CountLines` are the sole readers of their facts, fence-aware, shared by write and read.
- `scaffold` owns writing (`NewFeature`, `NewStep`, `Finish`); `assemble` owns reading
  (`Start`, `Status`, `Check`); the two never import each other. Shared rules live in
  `internal/platform/*`: `stepfile.DependencyIndex` ("blocked"), `internal/platform/conform`
  (four write-path predicates, each returning a path-less `Violation`).
- `Finish` validates in one fixed band ahead of `(refinish).verdict()`: handoff cap → state
  cap → state fence → state headings → open checklist item → unmet dependency → spec read →
  verdict. `tickProgressEntry` ticks only the marker span `checklistItemRe` matched, never
  a whole-line `Replace` — a title containing a literal `[ ]` must not be rewritten on an
  already-ticked entry, or R11's noop breaks.
- `Check(ctx, feature)` is the R18 backstop. `feature == ""`: `nil, nil` when the root is
  absent. Named feature: `ErrNoSuchFeature` for an absent root, a `.`/`..`/separator
  argument, or a name matching a regular file. A feature directory (named or in the
  all-features loop) that exists but can't be opened or listed is a `Finding` naming it
  (`ERROR`), never dropped; same for its own step-listing failure. A symlink where a
  feature is expected is marked, never followed — matches `Status`, resolved consistently
  across both. `C8`/`C9` (dangling/self dependency) apply on a **done** step too — `check`
  walks `fm.DependsOn` directly rather than reusing `idx.FirstUnmet`, which is a *refusal*
  predicate (stops at the first fault, exempts a done step) and must not gain a caller that
  needs enumeration. Severity is feature-wide, computed once after the step walk and
  back-filled; a feature-level finding is always `ERROR` (doneness unmeasurable).
- Findings render `<SEVERITY>  <path>[:<line>]  <detail>` (field renamed `Problem`→`Detail`:
  `assemble.Problem` is a distinct type; `scaffold.RefusalError.Problem` is unrelated too,
  untouched). `check` groups rows under a `<feature>  (in flight|complete)` header; `finish`
  prints them bare. `check` exits 1 on any `ERROR`, 0 otherwise.
- A cap finding's `Line` is `cap + 1`, set by the caller (`conform.OverCap` stays line-less).
- `assemble.Server` carries two unexported, test-only seams — `openRoot`/`readDir`, set via
  `export_test.go`'s `SetOpenRootForTest`/`SetReadDirForTest` — because root bypasses POSIX
  permission checks (a chmod test skips silently under root) and a chmod mode reproduces a
  *different* branch per OS. Both `Check` and `Status` now thread `s.readDir` into their
  per-feature listing call (`featureStatus` takes `readDir`, mirroring `openRoot`), so the
  "opens but can't be listed" branch is independently seam-testable on both paths, not just
  `Check`'s. Use the seam, or a self-referential symlink for a uid-independent ELOOP on the
  top-level `os.OpenRoot` call (no seam there); never gate a test on `os.Geteuid()`.
- `validFeatureArgument` rejects a path separator via `os.IsPathSeparator` per byte, not
  `strings.ContainsAny(feature, "/\\")` — `\` is an ordinary POSIX filename character, so the
  old check over-rejected a legal feature name on POSIX while staying correct on Windows.

## Left unbuilt

Every scenario is complete — everything below is unowned, with none left to claim it.

- R4: `DependencyIndex` transitive/cycle traversal; a dependency-cycle finding in `check`;
  `assemble.Server.Next` ordering by dependency.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`; `status --json`; `check --json`;
  `check` over a single step; `--force`/`--if-state-matches` and diff output on divergence.
- `markdown.Headings`, `markdown.ChecklistItems` (all-items listing), `HandoffPattern.Number`.
- `check` never reports: an id/filename mismatch, a leftover `## Handoff` section, a done
  step with no handoff file (R16 exempts it), or a missing progress-list entry.
- `cfg.OptionalConventions` unconsumed; no JSON error/refusal envelope, `"feature"` key, or
  schema-version key. `FeatureStatus.Blocked` is a count with no ids attached.
- Splitting the six oversized `_test.go` files in `assemble`/`scaffold`; extracting
  `Finish`'s resolve/commit phases — both MINOR debt, not attempted (mechanical risk
  outweighed benefit for a single fix-mode pass).

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish`.
  `-HANDOFF.md` sorts *before* its step file (`-` < `.`).
- `atomicfile.Create`'s temp-sibling mode can wedge a later write until removed.
- `assemble.RefusalError` duplicates `scaffold.RefusalError` knowingly (`assemble` must not
  import `scaffold`); `conform.Violation` is the piece that *isn't* duplicated.
- **A plan's fixture-update list can under-count — enumerate by running the suite.**
- `markdown.Section(body, "")` returns `found == true` on the first blank line.
- `!idx.recorded[dep]` is true for both an absent key and a recorded `false` — `Known` is
  the only way to tell a typo'd id from an open dependency; an unparseable depended-on step
  must be Recorded, not skipped. `idx.FirstUnmet` short-circuits on `fm.Done()` and stops at
  the first unmet id — correct for `Finish`'s refusal and `Status`'s Blocked count, wrong
  for a report; `check` never calls it (see above).
- This repository's own tree fails its own `check` (exit 1) — confirmed by the real binary,
  not adjusted away. Don't pin an exact finding count anywhere here — editing this file
  changes its own line count, which `check` measures.

## Open debts

- Heading/cap **value** validation (empty/duplicate heading, non-positive cap) — unowned.
- Invalid `step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too — unowned MINOR.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename — pre-existing, real, unowned, dies unless re-opened.
- `config.InvalidConfigError`'s refusal keeps the `(no files changed)` tail on a read
  command (`start`), contradicting R14a — pre-existing, unowned, dies.
- `cfg.DefaultOutputBudgetBytes` (R13's output budget/truncation) unconsumed — unowned, dies.
- A mid-write I/O failure leaves a half-applied, same-argument-retryable result; nothing
  proactively detects that state (self-repairs on retry) — unowned, dies.
- `conform.Violation.Fix` unused by `check`; the four `checkArgument*` wrappers in
  `scaffold/finish.go` forwarding to `conform` (left alone — inlining touches the check
  band several scenarios' ordering tests pin); `newProblem` naming the directory rather
  than the offending step file — all NIT/MINOR, unowned, dies unless re-opened.
- `validFeatureArgument` (`assemble/check.go`) isn't shared with `Start`/`Finish`/`NewStep`,
  which each answer a bare `.` argument differently — confirmed inconsistent copy, not a
  data-loss path (nothing escapes the feature root); whether it's a contract or Check-local
  hardening is a product question, unowned.
- Zero-value `assemble.Server` (nil seams, only reachable by skipping `NewServer`) and
  `tickMarker` indexing `loc[1]` with no nil check (sole caller already guards with the same
  regex) — both NIT/MINOR, not constructible today, unowned.
