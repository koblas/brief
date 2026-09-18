# brief — current state

Scenarios complete: SCENARIO-01..05. Last updated by SCENARIO-05.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus(body, status)
  ([]byte, error)` edits the `status:` line **textually** inside the delimiters — never
  decode-and-re-marshal, since `Frontmatter` has no `KnownFields` and a round trip drops unknown
  keys (fixture proof: `owner: planner`). No `status:` key → `ErrNoStatusField`, refused, never
  inserted. (03-05)
- `markdown.SectionRange(body, heading) (start, end int, ok bool)` is the one scanner for read
  and write; `Section` is built on it, so they can't drift. Fence-aware for **both** the anchor
  search and the end scan; `end` is the terminating heading line's **start byte**, leading
  whitespace included, so a splice never truncates indentation. (04-05)
- `assemble` (read) and `scaffold` (write, name is a rename debt — Traps) never import each
  other. `scaffold` now owns `finish` too: `(*Server).Finish(ctx, feature, step string, handoff,
  state []byte) error`. Every validation runs and every write body (spliced step, ticked spec,
  `status: done` frontmatter) is **computed before any write**, so a missing `status:` key is a
  refusal, not a half-done write. Order (R14a "first thing wrong"): pattern compiles → feature
  dir opens → step found via `pattern.ID(n) == step` (never string-matched on the filename) →
  frontmatter parses → handoff anchor present → spec readable → progress heading+entry (one
  `tickProgressEntry` scan) → state file present. Writes land **state → step file → spec**:
  state-first lets a crash-then-retry converge; step-file-first would leave `status: done` beside
  the *old* state, so 06/16's identity check would see a false mismatch and refuse — unrepairable.
  **06/16 must consult identity only when frontmatter already says done.** A write failure after
  validation returns as-is, never `*RefusalError` (its `(no files changed)` tail would lie). (05)
- Handoff splice is a **fixed point**: `"\n"+Trim(handoff,"\n")+"\n"`, +one more `"\n"` if a
  heading follows — for a handoff with no unfenced `##`+ heading. 06/16 compare against this
  shape. Caller supplies the handoff **body** only, never the heading. (05)
- `tickProgressEntry` reuses `insertProgressEntry`'s non-fence-aware section scan (both write
  paths must agree) but adds an id-**boundary** match so `STEP-10` never matches `STEP-1`. (05)
- `cli.Run` takes `stdin io.Reader` before `stdout`. `finish` prints nothing to stdout (reserved
  for R9 findings), one stderr line `brief finish: <step> is done` — state-describing, so 06's
  no-op path can print it unchanged. (05)

## Left unbuilt

- Identity/no-op detection — SCENARIO-06; differing-inputs refusal — SCENARIO-16. Today a second
  `finish` overwrites and reports success unconditionally.
- `Finish`'s caps (17/18), state-body required-heading check (19 — both detection and copy,
  deliberately deferred), open-checklist refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet` (09/…), `--json` (15),
  complete-feature stderr+exit-0 (12), unknown-feature enumeration (09), `markdown.Headings` +
  checklist parser (20/22), R13 output truncation.
- R9's dropped-entry diff and finding output — stdout already reserved. No `FinishResult` type.
  `Finish` accepts the step id only, not the filename.

## Traps

- **SCENARIO-13's cost:** moving `RefusalError` to a platform package now also touches `finish`'s
  two new refusals, plus every other `scaffold` site and `cli/refusal.go`.
- Until 16 ships, a second `finish` with a *different* handoff silently overwrites and reports
  success. Git + branch is the mitigation.
- A handoff beginning with the configured heading duplicates it when spliced; an unfenced `##`+
  heading inside a handoff breaks the fixed point. Neither refused — unowned, SCENARIO-19's
  family. Keep handoff bodies to `**bold**` labels and fenced examples.
- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt.
- `flag.FlagSet.Parse` stops at the first non-flag arg — `finish`'s positionals precede its
  flags, so they're split off (`splitLeadingPositionals`) before `Parse` runs. Any future command
  with positionals after flags needs the same treatment.
- `insertProgressEntry`/`tickProgressEntry` are deliberately not fence-aware — don't unify with
  `markdown.Section` opportunistically.
- Nothing binds `stepSkeleton`'s frontmatter literal to `ParseFrontmatter`/`SetStatus` except
  fixtures. `root = wd` unless `.brief.yaml` found — `start`/`new`/`finish` all repo-root only.
  Avoid `t.Chdir`/`os.Getwd` in tests. A "disk unchanged" sweep needs a control arm or it proves
  nothing.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result (`new`'s orphan dir/step file; `finish`'s
  split write landing state-only or state+step). Only the first write's failure is testable
  without a filesystem port (deliberately not built). Unowned; `check` (22) is the detector.
- Invalid-`step-file-pattern` refusal names the feature dir, not the config — unowned, shared by
  `new step` and `finish`.
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file — unowned.
- `assemble`'s sentinels duplicating `scaffold`'s — SCENARIO-13 closes this via the `RefusalError`
  move.
