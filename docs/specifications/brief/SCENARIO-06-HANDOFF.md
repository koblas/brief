**Binding decisions** — a later scenario must not contradict these without saying so:

- Identity compares the **computed write bodies against the current file bytes**, never the
  supplied input against an extracted section — `markdown.Section` trims blank lines while
  `spliceHandoff` trims only `"\n"`, so the input-vs-section check can report identity for an
  input that splices to different bytes, silently discarding the caller's handoff. SCENARIO-16's
  divergence detection must reuse the same three computed bodies for the same reason.
- The step-body comparison is taken **before `SetStatus`**, so the `status:` line is excluded from
  it. This is what makes the `fm.Done()` gate constructible — post-`SetStatus`, a done step is the
  only state in which identity can hold, and the gate becomes untestable dead code.
- The identity predicate is a **three-way conjunction**: spliced step body, state bytes, `newSpec`.
  The spec conjunct is not tidiness — after a crash between the step write and the spec write,
  `fm.Done()` is true and handoff+state match, so without it the progress checkbox stays stale
  forever. The handoff and state conjuncts are what stop a divergent input being skipped.
- The gate stays `fm.Done() && identical`, inherited from SCENARIO-05. SCENARIO-16's refusal must
  sit behind the same `fm.Done()` gate: an open step's first `finish` always diverges from the
  scaffolded empty handoff, so an ungated refusal would refuse every normal `finish`.
- `SetStatus` runs **before** the skip check, so a frontmatter whose `status:` YAML decodes but
  whose text `SetStatus` cannot find is refused even on a no-op re-finish. Chosen for R14a
  validation completeness; not constructible from `stepSkeleton`.
- The state file is now **read unconditionally** on every `Finish`, not just `Lstat`ed. That read
  is R9's down payment (the dropped-entry diff needs the outgoing body anyway), not waste.
- `finish` is **not a status-line normaliser**: a done step spelled `status: DONE` is skipped with
  its spelling intact rather than rewritten. `check` (SCENARIO-22) owns normalisation if anyone
  ever wants it.

**Left unbuilt** — named so nobody assumes it exists:

- Differing-inputs refusal — SCENARIO-16. Today any divergence falls through to the normal write
  path and overwrites, returning nil and printing `brief finish: <step> is done`. That is 16's red
  starting point, and `Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it` and
  `Test_re_finishing_a_done_step_with_a_different_state_body_replaces_it` are the two tests 16
  must **invert** into refusal-plus-byte-identity. Do not delete them; repoint them.
- `Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges` is the only
  mtime control arm that survives 16 — it diverges on the spec, not on a caller input. Keep it.
- Caps (17/18), required-heading check (19), open-checklist refusal (20), unfinished-`depends-on`
  refusal (21), R9's diff, `FinishResult`, `status`, `check`, `--json` — all still unbuilt.

**Traps** — things that look right and are not:

- A stat-before/stat-after ModTime comparison is **unfalsifiable** at coarse filesystem
  granularity: a write inside one tick looks unchanged. The pinned-past-timestamp form
  (`os.Chtimes` to a whole second an hour ago, then assert exact equality) is the only version
  that cannot pass by accident. Do not "simplify" it back.
- Mutating away the skip by deleting `return nil` is a **compile error** (`identical` declared and
  not used), and a mutation that breaks compilation proves nothing. Empty the branch body instead.
- `tickProgressEntry` does `strings.Replace(line, "[ ]", "[x]", 1)` on the whole line. An
  already-ticked entry whose *title* contains `[ ]` gets the title mutated, so `newSpec` differs
  from `specBytes` and identity falsely fails into a gratuitous rewrite. Pre-existing; 06's
  comparison is merely what makes it observable. Unowned.
- The identity check inherits its correctness from `spliceHandoff` being a fixed point. Break that
  fixed point and a re-finish silently rewrites forever.
- The `fm.Done()` gate is provable only because its fixture reverts **the status line and nothing
  else**. Revert the handoff too and the step-body conjunct carries the test — it becomes a second
  handoff-conjunct test and the gate goes untested while still looking covered.
- `markdown.Section` returns a section **trimmed** of leading and trailing blank lines; the handoff
  input is not. Asserting a handoff section against the raw input reds for the wrong reason.
