---
id: SCENARIO-10
status: done
depends-on: []
---

# SCENARIO-10: Status on a repository with no features succeeds silently

## Scenario

```
Scenario: SCENARIO-10 Status on a repository with no features succeeds silently  [orig: 10a]
  Given a project with no feature directories
  When I ask for status
  Then stdout is empty and the command succeeds
  And one line on stderr says no features were found
```

## What is already green (measured, not inherited)

Built `cmd/brief` at HEAD (5370c62) and ran `brief status` in four fixtures:

| fixture                                                   | exit | stdout   | stderr                                                         |
| --------------------------------------------------------- | ---- | -------- | -------------------------------------------------------------- |
| A: `.brief.yaml` present, **no** `docs/specifications`      | 1    | 0 bytes  | `brief status: assemble: open <abs>/docs/specifications: no such file or directory` |
| B: `docs/specifications` present, **empty**                 | 0    | 0 bytes  | empty                                                            |
| C: fresh dir, no config, no feature root                    | 1    | 0 bytes  | same ENOENT line as A                                            |
| D: feature root holds only a regular file                   | 0    | 0 bytes  | empty                                                            |
| E: feature root **is** a regular file (`ENOTDIR`)           | 1    | 0 bytes  | `brief status: assemble: open <abs>/docs/specifications: not a directory` |

So: B and D are green on arrival for "stdout is empty and the command succeeds" — `Status`
returns no rows and `RenderStatusText` writes nothing for an empty slice. **A and C are red**
(exit 1), and the stderr notice is missing in every case. E is the control that must stay
exit 1.

## Decisions this scenario takes

1. **A missing feature root becomes exit 0**, the same shape as an empty one. The Gherkin says
   "no feature **directories**", R14 says "Nothing to return is not an error: empty stdout, one
   line on stderr, exit 0 … a repository with no features … `brief status | wc -l` of 0 means no
   features", and STATE.md's *Left unbuilt* assigns "status on a missing/empty root →
   stderr+exit-0" to SCENARIO-10 by name. Fixture C — a repository that has never run `brief` —
   is the canonical instance; exiting 1 there makes R14's `wc -l` discriminator unreachable.
2. **Only `fs.ErrNotExist` is swallowed**, via `errors.Is`, never a string match. `ENOTDIR`
   (fixture E), permission denied and every other open failure stay exit 1 — a path component
   that is not a directory is a misconfiguration, not an empty repository.
3. **`assemble.Status` decides "the tree has no features"; `internal/cli` decides "say so".**
   `Status` returning `(nil, nil)` for an absent root keeps the filesystem rule in the package
   that owns reading, and gives `runStatus` a single condition — `len(rows) == 0`. The notice is
   presentation and goes to stderr, so it is written in `cli`, never in `RenderStatusText`,
   which produces stdout bytes only.
4. **Exact copy**, one line, written to stderr, followed by `return nil`:

   ```
   brief status: no features found in <cfg.FeatureDirectory>; run 'brief new feature <name>' to create one
   ```

   `<cfg.FeatureDirectory>` is the config value (`docs/specifications`), not the absolute joined
   path — it is the knob the user turns and it is stable in an assertion. The
   `<problem>; <imperative fix>` shape and the single-quoted command match
   `scaffold`'s `run 'brief new feature %s' to create it` and `cli`'s usage lines. R14a's refusal
   template does **not** govern this line: R14a scopes to write refusals, and this is neither a
   write nor a refusal — so there is deliberately no `<path>:` colon form and no
   `(no files changed)` tail.

## Implementation Plan

- [x] Step 1: `internal/cli/status_test.go` `Test_status_says_no_features_were_found_when_the_feature_root_is_absent` — `cli.Run` slice test in a bare `t.TempDir()` (no feature root at all): no error, exit code 0, stdout exactly `""`, stderr exactly the Decision-4 line, exactly one `\n` (red — exit 1 today)
- [x] Step 2: `internal/cli/status_test.go` `Test_status_says_no_features_were_found_when_the_feature_root_is_empty` — same assertions with `docs/specifications` present and empty (red only on the stderr line; stdout/exit already green, say so in the report) (red)
- [x] Step 3: `internal/assemble/status_test.go` `Test_status_reports_no_features_when_the_feature_root_does_not_exist` — `(*Server).Status` against a `t.TempDir()` with no feature root returns zero rows and no error (red)
- [x] Step 4: `internal/assemble/status_test.go` `Test_status_propagates_a_feature_root_that_is_not_a_directory` — control arm for Step 3: the feature root path exists as a **regular file**, `Status` returns an error (green on arrival; it is what keeps Step 5's guard from widening to "swallow every open failure") (new)
- [x] Step 5: `internal/assemble/status.go` `(*Server).Status` — treat an `errors.Is(err, fs.ErrNotExist)` failure from `os.OpenRoot` as zero features: return `nil, nil`; propagate every other error unchanged (green)
- [x] Step 6: `internal/assemble/status.go` `(*Server).Status` doc comment — its last sentence ("propagates an error from a missing or unreadable feature root") is now false. State the rule as it is: an absent feature root is zero features; an unreadable one, and a step file whose frontmatter does not parse, still propagate. No change narrative. Check with `go doc ./internal/assemble Server.Status` (update)
- [x] Step 7: `internal/cli/status.go` `runStatus` — after `srv.Status`, when `len(rows) == 0` write the Decision-4 line to stderr and `return nil` without calling `RenderStatusText` (green)
- [x] Step 8: `internal/cli/status.go` `statusUsage` — one sentence that a repository with no features prints nothing and succeeds, so `--help` matches the shipped behaviour (update)
- [x] Step 9: mutation-verify guard 1 **alone** — widen Step 5's condition to swallow every `os.OpenRoot` error. Step 4's test must go red. Restore, `git diff` clean (verify)
- [x] Step 10: mutation-verify guard 2 **alone** — make Step 7's notice unconditional (drop the `len(rows) == 0` branch). `Test_status_prints_one_line_per_feature_and_nothing_else` and `Test_status_prints_one_line_for_a_single_feature` must both go red on `assert.Empty(t, stderr.String())`. Restore, `git diff` clean (verify)
- [x] Step 11: full verification per the standing brief, then mark SCENARIO-10 done in `specification.md` (line 826)

## Notes for the developer

- **The absence claims already have their control arm — do not duplicate it.** SCENARIO-09's
  `Test_status_prints_one_line_per_feature_and_nothing_else` (three features) and
  `Test_status_prints_one_line_for_a_single_feature` (one feature) assert exact stdout bytes and
  `assert.Empty(stderr)`. They are the arm showing the same probe sees bytes on stdout and
  nothing on stderr when features exist. Keep both as `assert.Equal` on exact bytes — relaxing
  either to `Contains` destroys Step 10's mutation.
- The three bare-`t.TempDir()` tests already in `internal/cli/status_test.go` (extra argument,
  undefined flag, `--help`) run without a feature root but return before `config.Resolve` and
  `Status`, so Step 5 and Step 7 cannot reach them. They need no change.
- Fixture helper `writeStatusStep` creates `docs/specifications/<feature>/…`; Steps 1-3 must
  deliberately *not* call it.
- `assemble.Start` maps any `os.OpenRoot` failure on the top root to `ErrNoSuchFeature`
  (`assemble.go:52-54`). That is `start`'s path and SCENARIO-13's business — do not touch it.

## Out of scope — SCENARIO-10 must not build these

- The `!` field, per-feature malformed tolerance, or any stderr reason line for a broken
  feature — SCENARIO-11.
- `brief start`'s complete-feature "nothing to return" notice — SCENARIO-12, even though it is
  the same R14 shape.
- `--json` / `"step": null` — SCENARIO-15.
- Any change to `assemble.Start`, `ErrNoSuchFeature`, or `RenderStatusText`'s output format.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **An absent feature root is zero features, not an error** — `Status` returns `(nil, nil)` when
  `errors.Is(err, fs.ErrNotExist)` on `os.OpenRoot`; every other open failure (notably `ENOTDIR`
  when the root is a regular file) still exits 1. R14's `brief status | wc -l` of 0
  discriminator requires the exit-0 half; fixture E requires the narrowness.
- **The "no features" notice is keyed on `len(rows) == 0` in `cli`, not on an error value** —
  therefore **SCENARIO-11 must emit a row for a malformed feature**, not drop it. A repository
  whose only feature is malformed would otherwise print "no features found" and exit 0, swallowing
  exactly what 11 exists to surface.
- **Exact stderr copy, one line, exit 0:**
  `brief status: no features found in <cfg.FeatureDirectory>; run 'brief new feature <name>' to create one`.
  Interpolates the **config value**, not the absolute path. R14a's refusal template does not
  govern it — not a write, not a refusal, so no `<path>:` form and no `(no files changed)` tail.
  SCENARIO-12's complete-feature line is the same R14 "nothing to return" shape and should match
  this tone.
- **`Status` returns `nil`, not `[]FeatureStatus{}`, for zero features** — SCENARIO-15's `--json`
  marshals that to `null`, not `[]`. 15 owns that call; it should not reshape `Status`'s return
  to discover it late.

**Left unbuilt** — named so nobody assumes it exists:

- Malformed-feature tolerance, the `!` field, per-feature stderr reasons — SCENARIO-11.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), `start`'s
  complete-feature stderr+exit-0 (12) — all still unbuilt.
- No `cli` helper for "notice" lines exists; SCENARIO-10 writes the line inline in `runStatus`.
  SCENARIO-12 may extract one — there is no `renderNotice` to reuse today.

**Traps** — things that look right and are not:

- **A typo'd `feature-directory` now exits 0**, printing "no features found in `<typo>`". That is
  the R14 shape and the notice naming the directory is the diagnostic — do not "fix" it into a
  refusal without reopening this decision.
- `errors.Is(err, fs.ErrNotExist)` — never a string match on "no such file or directory"; the
  `ENOTDIR` message differs ("not a directory") and would slip through a substring guard.
- A feature directory containing **zero step files** is conforming and prints `0/0 - 0` (STATE.md,
  SCENARIO-09). It is not "no features" — only a root with zero *directories* triggers the notice.
  Fixture D (root holding only a regular file) also triggers it, correctly: `Status` skips
  non-directories.
- `Status`'s doc comment claims a missing root propagates. It must be rewritten in Step 6 or
  `go doc` ships a lie.
- Mutation-verify the two guards **individually** (Steps 9 and 10). Disabling both at once leaves
  every empty-root test passing for the wrong reason.
