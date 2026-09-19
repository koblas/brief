# brief — current state

Scenarios complete: SCENARIO-01..06. Last updated by a third post-06 fix pass (2 BLOCKER,
2 MAJOR); the first two passes guarded individual inputs instead of the shared mechanism, this
one replaced the mechanism. The crossover reads this file next.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus` edits `status:`
  **textually**, preserving CRLF. No `status:` key → `ErrNoStatusField`. (03-05)
- `markdown.SectionRange`/`Section` is the one scanner for read sections (checklist, acceptance,
  state headings), fence-aware both ends via a **state machine**: a fence closes only on a run of
  the opener's own character, ≥ its own length, carrying no info string (CommonMark §4.5),
  tolerating ≤3 leading spaces. `findHeading` is first-occurrence-wins. `UnterminatedFence` shares
  that same state machine and is the sole fence-open detector for both write-path arguments
  (handoff, state) and the on-disk state file. (04-06, fix ×2)
- **`spliceHandoff`'s end is `len(body)` unconditionally — never a terminator scan.** `## Handoff`
  is the last section of every step file by contract (default profile, `stepSkeleton`, R8's
  "replace wholesale"), so nothing after the anchor is ever a real terminator — the fixed point
  is structural, not luck. `markdown.HeadingStart` replaces `SectionRange` on this one write path;
  `SectionRange` is unchanged and still owns every other section. (fix ×3)
- **The cost of that contract is enforced, not assumed, and gated on `!fm.Done()`:** `Finish`
  refuses (`ErrHandoffNotLast`) when a step file not yet done already has a heading after the
  anchor. Once done, everything after the anchor is `Finish`'s own prior output and may
  legitimately contain a heading as ordinary handoff prose (R11 requires an identical re-finish
  to succeed regardless) — deleting the gate passes every test but one
  (`Test_finish_does_not_refuse_a_re_finish_...`), which exists to catch exactly that deletion.
  (fix ×3)
- `RefusalError` carries `Line int` (0 = whole-path refusal); `cli/refusal.go` renders
  `<path>:<line>`. `scaffold.HandoffSource`/`StateSource` are placeholder `Path` values for a
  refusal about an argument's bytes rather than a file `Finish` opened — `cli/finish.go` upgrades
  them to the real `--handoff`/`--state` source (or `"<stdin>"`) before rendering. (fix ×3)
- `Finish`'s check order: pattern → feature dir → step found → frontmatter → handoff anchor
  present (fence-fallback via `validateHandoffAnchor`) → anchor last (gated) → handoff's fence →
  state's fence → spec readable → progress entry → state file present → splice (now infallible)
  → identity → write. State's fence is checked twice: write (`Finish`, the argument) and read
  (`assemble.Start`, the on-disk file) — a hand-written `STATE.md` never passes through `Finish`.
  (fix ×3)
- **Identity/no-op (R11):** re-finishing a done step with identical inputs writes nothing, mtime
  preserved. Gate: `fm.Done() && identical`, taken **before `SetStatus`**. SCENARIO-16 inverts
  this into a refusal over the same three conjuncts and gate. (06)
- `cli.Run` takes `stdin` before `stdout`. `finish` prints nothing to stdout, one stderr line
  `brief finish: <step> is done`. (05-06)

## Left unbuilt

- Differing-inputs refusal (SCENARIO-16, today overwrites and reports success; git + branch is
  the mitigation), caps (17/18), state-body required-heading check (19, deferred), open-checklist
  refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), unknown-feature enumeration (09), `markdown.Headings` + checklist parser
  (20/22), R13 output truncation, R9's diff/finding output, `FinishResult`.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt; `RefusalError`'s eventual platform move (SCENARIO-13) touches every site here.
- **A hand-set `status: done` plus a hand-added trailing section escapes `ErrHandoffNotLast`**
  (the gate only runs while `!fm.Done()`) and gets silently clobbered on the next finish. Already
  an R10 protocol violation before `Finish` runs (hand-setting `status:` bypasses the tool
  entirely); unowned — flag for final review rather than solve.
- `tickProgressEntry` mutates a whole line on `[ ]`→`[x]`: an already-ticked title containing
  `[ ]` gets mutated too. Pre-existing, unowned.
- `root = wd` unless `.brief.yaml` found — avoid `t.Chdir`/`os.Getwd` in tests. gosec's `G703`
  flags a straight-line read→transform→write of one path in one test as path traversal even at a
  fixed `t.TempDir()` path — route through a helper function-call boundary.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result. Unowned; `check` (22) is the detector.
- Invalid-`step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file (mixed endings);
  `ParseFrontmatter`'s `rest` keeps a leading `"\r"` after a CRLF close when a blank line follows
  (cosmetic, every consumer trims `"\r"` per line already); `brief start` on a CRLF feature emits
  mixed line endings (LF frame, CRLF bodies). All three unowned MINOR.
- `SetStatus` returns `ErrNoStatusField` for a missing frontmatter *delimiter* too, not only a
  missing key — would render an untrue refusal if a future caller skipped `ParseFrontmatter`
  first. Unowned MINOR.
- `assemble`'s sentinels duplicate `scaffold`'s: e.g. `ErrNoSuchFeature` is a bare sentinel, never
  a `*RefusalError` — SCENARIO-13 closes this.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `finish` reads the state file on every call now; once the
crossover lands, `STATE.md` (this file) is that state file.
