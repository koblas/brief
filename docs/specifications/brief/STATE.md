# brief — current state

Scenarios complete: SCENARIO-01..04. Last updated by SCENARIO-04.

## Binding decisions

- Config (`internal/platform/config`): `.brief.yaml`, upward walk, nearest wins, missing anywhere
  = `config.Default()`. Carries `ChecklistHeading` (`## Implementation Plan`) and
  `AcceptanceHeading` (`## Scenario`, **optional** — read when present, never written by
  `scaffold.stepSkeleton`, deliberately unchanged). `Resolve(startDir) (Config, string, error)`;
  `errors.Is`-comparable to `ErrInvalidConfig`. (01/02/03/04)
- **`stepfile`** naming unchanged. New: `Frontmatter{ID, Status, DependsOn}`,
  `ParseFrontmatter(body) (Frontmatter, rest []byte, error)` (`ErrNoFrontmatter` on no leading
  `---`). `Frontmatter.Done()` is the sole doneness authority: trimmed+case-folded `status ==
  "done"`; everything else (`blocked`, `""`) is open — SCENARIO-09's blocked count must revisit.
  (03/04)
- **`internal/platform/markdown`** (new): `Section(body, heading) (string, bool)`,
  `Title(body) (string, bool)` — hand-rolled scans, no AST. Fence-aware (``` / ~~~); ends a
  section at the next same-or-**higher**-level heading (`##` ends `##`, not `###`).
  `headingLevelOf` tolerates **0–3** leading spaces before `#` (CommonMark's bound, not
  unbounded) — enough for this repo's 2-space-indented Gherkin comment lines to need
  fence-awareness, while a 4+-space continuation line inside a checklist item stays plain text. (04)
- **`internal/assemble`** (new, read-side feature package): `Server`, `NewServer(cfg, root)`,
  `Start(ctx, feature) (Brief, error)`, `RenderText(w, Brief) error`. `status`/`next`/`show`/
  `handoff`/`state get` join it as further methods, never new packages; `finish` writes, picks its
  own home in SCENARIO-05. `Start` parses frontmatter **first**, feeds `markdown` funcs only the
  rest — a `#` in YAML is never a heading. Foreign errors wrapped once
  (`fmt.Errorf("assemble: %w", err)`); own sentinels (`ErrNoSuchFeature`, `ErrMalformedFeature`)
  bare, **duplicating** `scaffold`'s rather than sharing them (Traps). "Next" = lowest step
  **number** (`stepfile.Number`, never directory/lexicographic order) with `Done() == false`;
  `depends-on` parsed, ignored for ordering. `Brief.Step` is a pointer, nil = no next step.
  `Step.Acceptance`/`.Checklist` are `Section{Heading, Body}` so `RenderText` needs no `cfg`.
  `Brief.Inherited` always carries all four state sections, empty body included; `RenderText`
  skips an empty section (rendering rule, not loading) and writes nothing for `Step == nil`
  (SCENARIO-12 owns that case). Refuses a missing state file (`ErrMalformedFeature`, no partial
  `Brief`); reads the state file only, no R6 synthesis. Writes nothing to disk (mutation- and
  sweep-verified). (04)
- `internal/cli`: `brief start <feature>` added, `startUsage` states read-only in six words.
  Errors through `renderRefusal(stderr, "start", err)` — bare sentinels hit the flatten fallback
  (`brief start: <cause>`, exit 1, no `(no files changed)` tail — read refusal). `expected one of:
  new` → `new, start` everywhere. Prior scaffold/atomicfile/RefusalError decisions unchanged.

## Left unbuilt

- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet` — SCENARIO-09/…; `--json`/JSON
  tags — SCENARIO-15; complete-feature stderr + exit-0 — SCENARIO-12.
- `markdown.Headings(body) []string` and a checklist-item (`- [ ]`) parser — SCENARIO-20/22.
- `assemble`'s errors carry **no** `RefusalError` shape, no user-facing copy — SCENARIO-13.
- `start` validates only the state file's existence, not the specification, progress list, or any
  handoff anchor — SCENARIO-13 owns all three.
- R13 output truncation: `Brief` has no budget field, `RenderText` no cap.
- Known-feature enumeration in the unknown-feature error — SCENARIO-09.

## Traps

- **SCENARIO-13's real cost:** the read-refusal template needs `RefusalError` visible to
  `assemble`, which must not import `internal/scaffold` — so `RefusalError` moves to a platform
  package, touching every `scaffold` construction site plus `cli/refusal.go`'s
  `errors.AsType[*scaffold.RefusalError]` branch. Until then `assemble`'s sentinels duplicate
  `scaffold`'s.
- **`scaffold.insertProgressEntry` is not fence-aware and ends at ANY `#` line**, unlike
  `markdown.Section` (fence-aware, indent-tolerant). Two rules by design — it's the write path;
  SCENARIO-05 keeps its own behavior, don't unify opportunistically. Unowned.
- Nothing binds `scaffold.stepSkeleton`'s frontmatter literal to `stepfile.ParseFrontmatter`
  except the SCENARIO-04 fixture writing it byte-for-byte. Unowned round-trip test.
- `root = wd` unless a `.brief.yaml` is found (none in this repo) — `start`, like `new`, works
  from the repo root only.
- Avoid `t.Chdir`/`os.Getwd` in tests (`EvalSymlinks` breaks macOS fixtures).
- `yaml.Decoder.Decode` on a zero-byte file returns `io.EOF`, not nil — `decodeConfig`'s guard
  must stay.
- A disk-unchanged sweep with no control arm proves nothing — any future "writes nothing" claim
  needs its own control or it is vacuous.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- Half-scaffolded feature dir / orphan step file on a mid-write I/O failure — unowned; `check`
  (SCENARIO-22) is the natural detector.
- Invalid-`step-file-pattern` refusal names the feature directory, not the config carrying the
  bad value — unowned.
- `insertProgressEntry` inserts an LF line into a CRLF file unmodified elsewhere — unowned.
- `assemble`'s sentinels duplicating `scaffold`'s — SCENARIO-13 closes this via the `RefusalError`
  move.
