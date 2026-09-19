# brief — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06 — the last pre-crossover
scenario; the crossover itself reads this file next.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus` edits the `status:`
  line **textually**, never decode-and-re-marshal (`Frontmatter` has no `KnownFields`; fixture
  proof `owner: planner`). No `status:` key → `ErrNoStatusField`, refused. (03-05)
- `markdown.SectionRange`/`Section` is the one scanner for read and write, fence-aware both ends.
  `Section` trims blank lines; `spliceHandoff`'s fixed point trims only `"\n"` — the two
  normalise differently, so SCENARIO-06/16 compare **computed write bodies against current file
  bytes**, never the supplied input against an extracted section. (04-06)
- `scaffold` owns scaffolding and `finish`. `(*Server).Finish(ctx, feature, step, handoff, state
  []byte) error` runs every validation and computes every write body before any write. Order
  (R14a): pattern compiles → feature dir opens → step found via `pattern.ID(n) == step` →
  frontmatter parses → handoff anchor present → spec readable → progress entry found → state
  file present (`Lstat`, then read unconditionally — R9's down payment). Writes land **state →
  step file → spec**: state-first lets a crash-then-retry converge; the reverse would leave
  `status: done` beside old state, breaking the identity check below. A write failure after
  validation returns as-is, never `*RefusalError` (`(no files changed)` would lie). (05-06)
- **Identity/no-op (R11):** re-finishing a done step with identical inputs writes nothing, mtime
  preserved on all three files. Gate: `fm.Done() && identical`, `identical` a three-way
  conjunction — pre-`SetStatus` spliced step body vs. on-disk step body, `state` vs. on-disk
  state bytes, `newSpec` vs. on-disk spec bytes. Taken **before `SetStatus`**: post-`SetStatus`
  only an already-done step could ever match, making the `fm.Done()` gate dead code. The spec
  conjunct is crash-convergence, not tidiness: after a crash between the step write and the spec
  write, `fm.Done()` is already true and handoff+state match, so the gate alone would leave the
  checkbox stale forever. `SetStatus` runs before the skip check, so a frontmatter that
  YAML-decodes but whose text `SetStatus` can't find is refused even on a no-op re-finish. Any
  single divergent conjunct falls through and overwrites — SCENARIO-16 inverts that into a
  refusal over the **same three conjuncts and gate**. `finish` is not a status-line normaliser:
  `status: DONE` is skipped with its spelling intact. (06)
- `cli.Run` takes `stdin` before `stdout`. `finish` prints nothing to stdout (reserved for R9),
  one stderr line `brief finish: <step> is done` — an overwrite and a no-op are indistinguishable
  at the command surface, by design. (05-06)

## Left unbuilt

- Differing-inputs refusal (SCENARIO-16), over SCENARIO-06's three conjuncts and its `fm.Done()`
  gate (an open step's first `finish` always diverges from the scaffolded empty handoff, so an
  ungated refusal would refuse every normal `finish`). Today divergence overwrites and reports
  success; git + branch is the mitigation.
- Caps (17/18), state-body required-heading check (19, deliberately deferred), open-checklist
  refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), unknown-feature enumeration (09), `markdown.Headings` + checklist parser
  (20/22), R13 output truncation, R9's diff/finding output, `FinishResult`.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt. `RefusalError`'s eventual platform move (SCENARIO-13) touches every site here plus
  `cli/refusal.go`.
- A handoff beginning with the configured heading duplicates it when spliced; an unfenced `##`+
  heading inside a handoff breaks the fixed point the identity check depends on — break it and a
  re-finish silently rewrites forever. Unowned, SCENARIO-19's family.
- `tickProgressEntry` does `strings.Replace(line, "[ ]", "[x]", 1)` on the whole line: an
  already-ticked entry whose *title* contains `[ ]` gets its title mutated, which would make
  identity falsely fail into a gratuitous rewrite. Pre-existing, unowned; SCENARIO-06 merely makes
  it observable. Not fence-aware — don't unify with `markdown.Section` opportunistically.
- Stat-before/stat-after `ModTime` is unfalsifiable at filesystem granularity; tests pin via
  `os.Chtimes` to a fixed whole-second past timestamp and assert exact (in)equality after.
- `root = wd` unless `.brief.yaml` found — avoid `t.Chdir`/`os.Getwd` in tests. A "disk unchanged"
  sweep needs a control arm or it proves nothing.
- gosec's taint analysis (`G703`) flags a straight-line read→transform→write of the same path in
  one test function as path traversal, even at a fixed `t.TempDir()` path. Route such rewrites
  through a small helper (e.g. `readFileString(t, path) string`); a function-call boundary clears
  it.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result (`new`'s orphan dir/step file; `finish`'s
  split write landing state-only or state+step). Unowned; `check` (22) is the detector.
- Invalid-`step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file — unowned.
- `assemble`'s sentinels duplicating `scaffold`'s — SCENARIO-13 closes this.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable — resume a partial crossover by replaying the same commands, no
manual cleanup, no mtime churn on this 47 KB specification. `finish` reads the state file on
every call now; once the crossover lands, `STATE.md` (this file) is that state file.
