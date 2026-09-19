**Binding decisions** — a later scenario must not contradict these without saying so:
- `finish` is `(*scaffold.Server).Finish`, not a new package — `scaffold` owns `RefusalError`, the
  three sentinels, `insertProgressEntry`'s section scan and the `atomicfile` discipline, and a
  feature package may not import another. Every later **write** command joins `scaffold`; every
  later **read** command joins `assemble`.
- Write order is **state file → step file → specification**, validation-complete before the first
  write — under step-file-first, a crash leaves `status: done` beside the old state body and
  SCENARIO-16's differing-inputs refusal then makes the feature unrepairable through the tool.
  06 and 16 must therefore consult identity **only** when frontmatter already says done.
- Validation order is pinned in §3 and is R14a's "first thing wrong" contract: 17/18/19 insert
  **before** step 1 (no disk reads), 20/21 **after** step 4, 06/16 **after** step 5.
- `markdown.SectionRange` is the single scanner for the handoff anchor — fence-aware for the
  anchor search as well as the end scan, because a step file's own plan body contains fenced
  `## Handoff` examples. `Section` is now implemented on top of it; they cannot drift.
- The handoff splice is a **fixed point**: `"\n" + Trim(handoff,"\n") + "\n"` (plus one `"\n"`
  when a heading follows) — for a body whose lines are not unfenced `##`-or-higher headings; the
  profile's three labelled groups are `**bold**`, not headings. SCENARIO-06 and 16 compare
  recorded-vs-supplied against this shape.
- Frontmatter is edited **textually** by `stepfile.SetStatus`, never decode-and-re-marshal —
  `Frontmatter` has no `KnownFields`, so a round trip drops unknown keys. No `status:` key →
  `ErrNoStatusField`, refused, never inserted.
- The progress tick keeps `insertProgressEntry`'s non-fence-aware scan **on purpose**, so `new
  step` and `finish` agree about the progress section; the handoff splice is fence-aware. Two
  rules, two reasons.
- `cli.Run` now takes `stdin io.Reader` before `stdout`.
- `finish` prints **nothing to stdout** (reserved for R9 findings) and one **state-describing**
  stderr line, `brief finish: <step> is done`, chosen so SCENARIO-06's no-op path prints it
  without lying.
- The caller supplies the handoff **body**, never the heading; the heading is configuration.

**Left unbuilt** — named so nobody assumes it exists:
- Cap checks on both inputs — SCENARIO-17/18. State-body heading validation, both detection and
  copy — SCENARIO-19; `## Phasing` explicitly weighed and rejected pulling these forward.
- `Finish` open-checklist and `depends-on` refusals — SCENARIO-20/21. Today `Finish` writes.
- Identity detection, the skipped write and mtime preservation — SCENARIO-06; the differing
  re-finish refusal — SCENARIO-16. Today a second `finish` overwrites and reports success.
- R9's dropped-entry diff and the `[SEVERITY] path:line — finding` output — later phase; stdout
  is already reserved for it.
- An empty or zero-byte handoff body is written verbatim; nothing refuses it.
- `Finish` accepts the step **id** only (`STEP-02`), not the filename (`STEP-02.md`).
- No `FinishResult` type; `Finish` returns `error` alone. R9 will need one.

**Traps** — things that look right and are not:
- Until SCENARIO-16 ships, a second `finish` with a **different** handoff silently overwrites the
  recorded one and reports success — on this repo's own files, across the crossover window. Git
  and a branch are the mitigation, per *Bootstrap hazards*.
- A handoff file that begins with the configured handoff heading is spliced verbatim under the
  anchor, producing **two** headings. The developer implementing this plan is the first person
  who will hit it. Strip the heading from the file; nothing refuses it. Unowned debt, SCENARIO-19's
  family.
- A handoff body containing an **unfenced `##`-or-higher heading** breaks the fixed point:
  `SectionRange` terminates the recorded section at that heading on the next read, so 06 sees a
  phantom difference, 16 refuses, and a re-splice leaves the old tail orphaned below the new body.
  Keep handoff bodies to `**bold**` labels and fenced examples. Nothing refuses it — same family
  and same owner as the duplicated-heading trap above.
- A mid-sequence write failure leaves a half-applied `finish`. Only the **first** write's failure
  is testable (Step 21); there is no seam between writes without a filesystem port, which
  `scaffold`'s `doc.go` deliberately rejected. Unowned debt; `check` (SCENARIO-22) is the detector.
- The target step's checklist must be fully `- [x]` and its `depends-on` must name a done step in
  the fixture, or SCENARIO-20/21 retroactively redden this scenario's happy path.
- The package is still called `scaffold` while owning the whole write path. Mechanical rename
  debt, unowned.
- STATE.md's standing traps still hold: `assemble`/`scaffold` sentinels stay duplicated until
  SCENARIO-13 moves `RefusalError` down; avoid `t.Chdir`/`os.Getwd` in tests; a sweep with no
  control arm proves nothing.
