# brief — current state

Scenarios complete: SCENARIO-01..06. Last updated by a second post-06 reviewer fix pass (1
BLOCKER — unterminated-fence handoff dropping trailing sections; 4 MAJOR — CRLF false-negative
refusals, assemble's control-arm probe, the fence/heading 3-vs-4-space boundary, duplicate-heading
contract) that runs before the crossover; the crossover reads this file next.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus` edits `status:`
  **textually**. `frontmatterOpenLen` accepts `"---\n"` or `"---\r\n"`; `SetStatus` preserves both
  the opening delimiter's and the replaced status line's own ending, never downgrading CRLF to
  LF. No `status:` key → `ErrNoStatusField`. (03-05, fix pass ×2)
- `markdown.SectionRange`/`Section` is the one scanner for read and write, fence-aware both ends
  via a **state machine**: a fence closes only on a run of the opener's own character, ≥ its own
  length, **carrying no info string** (CommonMark §4.5), tolerating ≤3 leading spaces (`> 3`,
  never `>= 3`; pinned at exactly-3-accepted/4-rejected) like `headingLevelOf`. `findHeading` is
  first-occurrence-wins by contract and by test. `UnterminatedFence` refuses (R12, before any
  write) a handoff whose fence never closes — spliced in as-is it would make the next `Finish`'s
  end-scan run to EOF and silently drop every section past the anchor. Every heading/fence
  comparison in this package **and** its callers (`stepfile.frontmatterOpenLen`, `render.go`'s
  progress-heading match) now trims `"\r"` — no remaining read-path gap, only write-path ones
  (Open debts). `Section` trims blank lines; `spliceHandoff`'s fixed point trims only `"\n"` —
  SCENARIO-06/16 compare **computed write bodies against current file bytes**, never supplied
  input against an extracted section. (04-06, fix pass ×2)
- `assemble.Section.Found` distinguishes "heading absent" from "heading present but empty".
  `stepFromEntry` reads only `e.rest` (post-`ParseFrontmatter`) — its test's control arm now
  probes the same way, so it can't diverge from what production reads. (fix pass ×2)
- `stepfile.Compile` accepts `%d` and any zero-padded `%0Nd` verb including `%0d`; `%3d`/`%-4d`
  are refused — `Number`'s digits-only scan can never read a space back. (fix pass ×2)
- `scaffold` owns scaffolding and `finish`. `Finish` runs every validation and computes every
  write body before any write, in R14a order: pattern → feature dir → step found → frontmatter →
  handoff anchor → handoff's fence closes (`ErrUnterminatedFence`) → spec readable → progress
  entry → state file present. Writes land **state → step file → spec**. A write failure after
  validation returns as-is, never `*RefusalError`. (05-06, fix pass 2)
- **Identity/no-op (R11):** re-finishing a done step with identical inputs writes nothing, mtime
  preserved. Gate: `fm.Done() && identical`, taken **before `SetStatus`**. SCENARIO-16 inverts
  this into a refusal over the same three conjuncts and gate. (06)
- `cli.Run` takes `stdin` before `stdout`. `finish` prints nothing to stdout, one stderr line
  `brief finish: <step> is done`. (05-06)

## Left unbuilt

- Differing-inputs refusal (SCENARIO-16). Today divergence overwrites and reports success; git +
  branch is the mitigation.
- Caps (17/18), state-body required-heading check (19, deferred), open-checklist refusal (20),
  unfinished-`depends-on` refusal (21).
- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), unknown-feature enumeration (09), `markdown.Headings` + checklist parser
  (20/22), R13 output truncation, R9's diff/finding output, `FinishResult`.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt; `RefusalError`'s eventual platform move (SCENARIO-13) touches every site here.
- A handoff beginning with the configured heading duplicates it when spliced; an unfenced `##`
  heading inside a handoff breaks the identity fixed point. Unowned, SCENARIO-19's family. (An
  *unterminated* fence is now refused, not silent — R12.)
- `tickProgressEntry` mutates a whole line on `[ ]`→`[x]`: an already-ticked title containing
  `[ ]` gets mutated too. Pre-existing, unowned.
- Stat-before/stat-after `ModTime` is unfalsifiable at filesystem granularity; tests pin via
  `os.Chtimes` to a fixed past timestamp.
- `root = wd` unless `.brief.yaml` found — avoid `t.Chdir`/`os.Getwd` in tests.
- gosec's `G703` flags a straight-line read→transform→write of one path in one test as path
  traversal even at a fixed `t.TempDir()` path. Route through a helper function-call boundary.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result. Unowned; `check` (22) is the detector.
- Invalid-`step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file even though their
  heading match is now CRLF-safe — leaves the file with one mixed ending. Unowned.
- `%3d`/`%-4d` refusal doesn't name the broken rule and differs between `start` and `finish` —
  unowned MINOR (R14a wants the problem stated).
- `ParseFrontmatter`'s `rest` keeps a leading `"\r"` after a CRLF close when a blank line
  follows — cosmetic; every consumer trims `"\r"` per line already. Unowned.
- `assemble`'s sentinels duplicate `scaffold`'s: e.g. `ErrNoSuchFeature` is a bare sentinel, never
  a `*RefusalError` — SCENARIO-13 closes this.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `finish` reads the state file on every call now; once the
crossover lands, `STATE.md` (this file) is that state file.
