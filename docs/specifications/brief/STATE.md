# brief — current state

Scenarios complete: SCENARIO-01..06. Last updated by a post-06 reviewer fix pass (1 BLOCKER,
4 MAJOR: fence matcher, `stepfile.Compile`, CRLF handling, `atomicfile` coverage) that runs
before the crossover; the crossover reads this file next.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus` edits the `status:`
  line **textually**, never decode-and-re-marshal (`Frontmatter` has no `KnownFields`; fixture
  proof `owner: planner`). No `status:` key → `ErrNoStatusField`, refused. (03-05)
- `markdown.SectionRange`/`Section` is the one scanner for read and write, fence-aware both ends
  via a **state machine** (`fenceState`+`fenceDelim`), not a boolean toggle: a fence closes only
  on a run of the opener's own character, ≥ its own run length (CommonMark §4.5), tolerating
  ≤3 leading spaces like `headingLevelOf`. Every heading/fence comparison trims `"\r"` too
  (`trimEOL`), so CRLF files read like LF ones. `SectionRange`, `lines`, `findHeading` all route
  through both — never reintroduce a boolean toggle or a `" \t"`-only trim here; `spliceHandoff`
  and every later read (`start`, `finish`, `check`) inherit the contract for free. `Section`
  trims blank lines; `spliceHandoff`'s fixed point trims only `"\n"` — the two normalise
  differently, so SCENARIO-06/16 compare **computed write bodies against current file bytes**,
  never supplied input against an extracted section. (04-06, fix pass)
- `assemble.Section.Found` (new) distinguishes "heading absent" from "heading present but
  empty" — both used to render as an empty `Body` with the `ok` silently discarded.
  `RenderText` still ignores `Found`; SCENARIO-14's required-heading check is the first
  consumer that needs it. (fix pass)
- `stepfile.Compile` accepts only `%d` and zero-padded `%0Nd` verbs; `%3d`/`%-4d`-style
  space-padded or left-justified verbs are refused, because `Number`'s digits-only scan can
  never read a space back out of what `Name` renders. (fix pass)
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
  state bytes, `newSpec` vs. on-disk spec bytes. Taken **before `SetStatus`**, or the `fm.Done()`
  gate would be dead code. Any single divergent conjunct falls through and overwrites —
  SCENARIO-16 inverts that into a refusal over the **same three conjuncts and gate**. `finish`
  is not a status-line normaliser: `status: DONE` is skipped with its spelling intact. (06)
- `cli.Run` takes `stdin` before `stdout`. `finish` prints nothing to stdout (reserved for R9),
  one stderr line `brief finish: <step> is done` — an overwrite and a no-op are indistinguishable
  at the command surface, by design. (05-06)

## Left unbuilt

- Differing-inputs refusal (SCENARIO-16), over SCENARIO-06's three conjuncts and its `fm.Done()`
  gate (an open step's first `finish` always diverges from the scaffolded empty handoff, so an
  ungated refusal would refuse every normal `finish`). Today divergence overwrites and reports
  success; git + branch is the mitigation.
- Caps (17/18), state-body required-heading check (19, deliberately deferred — `Section.Found`
  now exists to support it), open-checklist refusal (20), unfinished-`depends-on` refusal (21).
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
  identity falsely fail into a gratuitous rewrite. Pre-existing, unowned. Not fence-aware or
  CRLF-safe — don't unify with `markdown.Section` opportunistically without re-checking both.
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
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file and trim headings
  with `" \t"` only, not `trimEOL` — unowned, doesn't inherit the markdown fence/CRLF fix.
- `assemble`'s sentinels duplicating `scaffold`'s: e.g. `ErrNoSuchFeature` is a bare sentinel,
  never a `*RefusalError`, so `start` on a missing feature prints a bare line where `new step`
  prints the full templated refusal — SCENARIO-13 closes this.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable — resume a partial crossover by replaying the same commands, no
manual cleanup, no mtime churn on this 47 KB specification. `finish` reads the state file on
every call now; once the crossover lands, `STATE.md` (this file) is that state file.
