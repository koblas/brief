# SCENARIO-06: Finishing a finished step with the same inputs changes nothing

## Scenario

```gherkin
Scenario: SCENARIO-06 Finishing a finished step with the same inputs changes nothing
  Given a step already finished
  When I finish it again with the same handoff and the same state body
  Then the command succeeds
  And all three files are byte-identical, mtime included, because the write was skipped
```

Governed by **R11** as amended: *"Finishing a finished step with the same inputs succeeds and
changes nothing — identity is detected and the write skipped outright, rather than re-spliced to
an identical result, so mtime is preserved."*

## What this scenario decides (spec was silent; these are choices)

**D1 — What is compared: the computed write bodies against the current file bytes.** `Finish`
already computes all three write bodies before the first byte reaches disk (SCENARIO-05's
contract), so identity is three comparisons over values already in hand:

- the spliced step body — `stepBody[:frontLen] + splicedRest`, taken **before** `SetStatus` — against `stepBody`
- the supplied `state` against the state file's current bytes
- `newSpec` against `specBytes`

Rejected: comparing the *supplied input* against the *extracted current section*
(`markdown.Section(rest, HandoffHeading)` vs `handoff`). `Section` trims blank lines while
`spliceHandoff` trims only `"\n"`, so the two normalise differently — an input that compares
equal under that check can still splice to different bytes. "Identity detected" would then be a
lie, and the tool would report success while discarding the caller's handoff. That is verbatim
the failure R11 exists to prevent. Comparing computed-against-current cannot disagree with the
re-splice outcome, because it *is* the re-splice outcome.

**D2 — The step comparison is taken before `SetStatus`, deliberately.** The frontmatter half of
the spliced body is copied verbatim from disk, so a pre-`SetStatus` comparison excludes the
`status:` line. Two things follow, both wanted:

- It makes SCENARIO-05's `fm.Done()` gate **constructible and therefore testable** — see D3.
- A step already done but spelled `status: DONE` or `status:   done` is skipped with its
  spelling preserved, rather than rewritten once for normalisation. `finish` is not a
  status-line normaliser; R11's mtime promise outranks tidiness.

**D3 — The gate is `fm.Done()`, and the spec conjunct is what makes a crash converge.**
SCENARIO-05 bound this: identity may be consulted only when frontmatter already says done. Under
D2 the gate is load-bearing and provable — a step whose `status:` is `open` with every other byte
matching must still be written, or it is never marked done and becomes unrepairable through the
tool. The `newSpec == string(specBytes)` conjunct is load-bearing for the other direction: after a
crash between the step write and the spec write, `fm.Done()` is already true and handoff+state
both match, so the gate alone would skip and leave the progress checkbox stale forever.

**D4 — `SetStatus` runs before the skip check.** `fm.Done()` being true does not guarantee
`SetStatus` succeeds: `ParseFrontmatter` YAML-decodes, so `{status: done, id: X}` populates
`fm.Status`, while `SetStatus` needs a line whose prefix at column 0 is `status:`. Running it
first means such a step is refused even on a no-op re-finish. Chosen for validation completeness
(R14a: every validation runs before any write). Not constructible from `stepSkeleton`; recorded
because it is observable behaviour no test here covers.

**D5 — Partial divergence is explicitly NOT this scenario.** Exact match on **both** caller inputs
(plus a spec already ticked) is the skip. Any divergence in any conjunct falls through to the
normal write path and overwrites, exactly as today. SCENARIO-16 inverts that into a refusal;
steps 2 and 3 below ship the two tests it must invert, so 16 arrives with a coded red start.

**D6 — mtime is asserted against a pinned past timestamp, not a stat-before/stat-after delta.**
`os.Stat` ModTime compared before and after is unfalsifiable at coarse filesystem granularity: two
writes inside one tick look unchanged. Instead each test `os.Chtimes` all three files to a fixed
whole-second time an hour in the past, then asserts `ModTime().Equal(pinned)` exactly. Any write,
at any granularity, moves mtime to now — an hour away — so the assertion cannot pass by accident.
The control arm (step 3) pins the same way and asserts the times are **no longer** pinned.

## Implementation Plan

- [x] Step 1: `internal/scaffold/finish_idempotent_test.go` — new file, `package scaffold_test`, reusing `newFinishFixture`/`snapshotTree`/`fixtureConfig` from `finish_test.go` (which is already 574 lines; idempotence is its own behaviour family). Add three local helpers: `newFinishedFixture(t) finishFixture` — builds `newFinishFixture`, runs one `Finish` with `fx.newHandoff`/`fx.newState`, `require.NoError`s it, returns the fixture (steps 2–8 all open with it, so each test body shows only its own divergence — which is what makes the one-variable control-arm claim readable; extract it now rather than retrofitting across seven tests); `pinModTimes(t, dir, names []string, when time.Time)` wrapping `os.Chtimes`; and `modTimes(t, dir, names []string) map[string]time.Time` wrapping `os.Stat`. Pinned constant `time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)` — whole seconds so a 1-second-granularity filesystem represents it exactly (new)
- [x] Step 2: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it` — `newFinishedFixture`, then `Finish` again with a *different* handoff and the same state. Assert `markdown.Section(rest, cfg.HandoffHeading)` equals `strings.Trim(string(differentHandoff), "\n")` — `Section` returns the section **trimmed**, so an assertion against the raw input's trailing newline reds for the wrong reason (see `finish_test.go:219` for the established shape). Green on arrival (today's unconditional overwrite) — say so, do not manufacture a red. This is the arm that reddens if the handoff conjunct is dropped (new)
- [x] Step 3: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_a_different_state_body_replaces_it` — `newFinishedFixture`; `Finish` again with the same handoff and a *different* state body. Assert the state file equals the different body (written verbatim, no trim). Green on arrival. Reddens if the state conjunct is dropped (new)
- [x] Step 4: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_whose_progress_entry_was_un_ticked_re_ticks_it` — `newFinishedFixture`, then rewrite `specification.md` replacing `- [x] STEP-02` with `- [ ] STEP-02`; `Finish` again with identical handoff and state. Assert the spec contains `- [x] STEP-02: Assemble the thing`. Green on arrival. Reddens if the spec conjunct is dropped (new)
- [x] Step 5: `finish_idempotent_test.go` `Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges` — step 4's fixture; `pinModTimes` all three files, `Finish` with identical inputs, assert each `ModTime` is **not** equal to the pinned time. This is step 6's control arm: it differs from the claim in exactly one variable (the checkbox), it proves the probe can observe a write, and unlike a different-handoff control it stays valid after SCENARIO-16 lands. Green on arrival (new)
- [x] Step 6: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_modification_time` — `newFinishedFixture`; `pinModTimes` all three; `Finish` again with the identical handoff and state; `require.NoError`; assert each of the three `ModTime`s equals the pinned time exactly. **RED** — today all three are rewritten. Prove it: `go test ./internal/scaffold/ -run 'Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_modification_time' -v` (red)
- [x] Step 7: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_the_same_inputs_leaves_every_file_byte_identical` — `newFinishedFixture`; `snapshotTree` before, `Finish` again, `snapshotTree` after, `assert.Equal`. Green on arrival — the existing `Test_finishing_with_a_handoff_already_in_place_reproduces_the_file_byte_for_byte` covers only the step file; this extends the content claim to the spec and state file. Say it was green (new)
- [x] Step 8: `finish_idempotent_test.go` `Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk` — `newFinishedFixture`, then rewrite `STEP-02.md` replacing `status: done` with `status: open`, leaving handoff, state file and ticked checkbox untouched — **revert the status line and nothing else**, or the step-body conjunct carries the test and the gate goes untested. `Finish` with the identical inputs; `require.NoError`; assert `stepfile.ParseFrontmatter` reports `Done()`. This is the single-variable form of SCENARIO-05's crash window and the only non-vacuous proof the `fm.Done()` gate is load-bearing. Green on arrival (new)
- [x] Step 9: `internal/scaffold/finish.go` `(*Server).Finish` — bind the frontmatter (`fm, rest, err := stepfile.ParseFrontmatter(...)`, today discarded); after the existing `Lstat` regular-file refusal, read the state file's current bytes (an I/O failure on an existing regular file returns `fmt.Errorf("scaffold: %w", err)`, matching the step-file read at line 88, **not** the "state file is missing" refusal); compute the three-way `identical` over the pre-`SetStatus` step body, the state bytes and `newSpec`; run `SetStatus` as today; then `if fm.Done() && identical { return nil }` before the first `atomicfile.WriteFile`. Step 6 goes green (green)
- [x] Step 10: mutation-verify the five conjuncts **individually**, each stashed per `.claude/rules/agent-briefs.md`, each reddening exactly one named test: (a) drop `fm.Done() &&` → step 8 reddens; (b) drop the handoff `bytes.Equal` → step 2 reddens; (c) drop the state `bytes.Equal` → step 3 reddens; (d) drop the spec comparison → steps 4 and 5 redden; (e) disable the skip → step 6 reddens. **Trap in (e):** deleting the `return nil` leaves `identical` declared-and-unused, which is a Go *compile* error, and a mutation that breaks compilation is not evidence. The valid surgical form is emptying the branch body — `if fm.Done() && identical { }` — which keeps `identical` used and compiles. Report which mutation reddened which test (verify)
- [x] Step 11: `internal/cli/finish_test.go` `Test_finishing_an_already_finished_step_a_second_time_prints_the_same_line_and_succeeds` — reuse the existing `newFinishCLIFixture`/`writeInput` helpers (they already solve the `root = wd`, no-`.brief.yaml` setup; do not build a new fixture and do not use `t.Chdir`). Call `cli.Run` twice with identical argv and fresh buffers for the second call. Assert on the second: `require.NoError`, `assert.Empty(stdout)`, `assert.Equal("brief finish: SCENARIO-01 is done\n", stderr)`. Pins the user-visible contract: exit 0, nothing on stdout (reserved for R9 findings), the same state-describing stderr line as a writing run. **Green on arrival and it cannot redden under any step-10 mutation** — before step 9 an overwriting second `finish` also returns nil and prints the identical line. That indistinguishability *is* the contract being pinned; do not try to manufacture a red, and do not add an mtime assertion here — step 6 owns that claim (new)
- [x] Step 12: `internal/scaffold/finish.go` — update `Finish`'s doc comment: state the identity rule as a contract (skipped outright when frontmatter says done and all three computed bodies equal what is on disk, so mtime is preserved), why the step comparison precedes `SetStatus`, and why the spec conjunct is required for crash convergence. No history, no scenario ids — state the rule (update)
- [x] Step 13: `internal/scaffold/doc.go` — one sentence in the package comment that `Finish` is a true no-op on a re-finish with identical inputs. Verify with `go doc ./internal/scaffold` and `go doc ./internal/scaffold Server.Finish` (update)
- [x] Step 14: verification, unpiped, from the repo root — `go build ./...`; `go test ./...` (report the exact test count and the delta); `go test -race ./internal/scaffold/... ./internal/cli/...`; `golangci-lint run ./...`; `go test -v ./internal/scaffold/... 2>&1 | grep -c -- "--- SKIP"` (verify)
- [x] Step 15: `docs/specifications/brief/specification.md` — tick `- [ ] SCENARIO-06` in `## BDD Acceptance Progress`. **By hand and by hand only**; nothing else in that 47 KB file changes. Then rewrite `docs/specifications/brief/STATE.md` from the Handoff below (update)

## User-visible contract

| Invocation | stdout | stderr | exit |
| --- | --- | --- | --- |
| `brief finish <f> <s> --handoff <p> --state <p>` on a done step, inputs identical | empty | `brief finish: <step> is done` | 0 |
| the same, any input divergent | empty | `brief finish: <step> is done` | 0 (overwrites — SCENARIO-16 turns this into a refusal) |

Unchanged from SCENARIO-05: the no-op is indistinguishable from a writing run at the command
surface, by design, because the line describes state rather than an action.

## Handoff

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

## Crossover note

Flagged for whoever reads STATE.md at the crossover, not planned here:

- A re-finish with identical inputs is now a **true no-op**, so the crossover's "mark SCENARIO-01
  through 06 done with handoffs written from what was built" step is safely re-runnable. A partial
  crossover can be resumed by replaying the same commands — no manual cleanup, no mtime churn on
  the 47 KB specification.
- The two divergence arms (steps 2 and 3) are the first step-files-through-the-tool work item:
  SCENARIO-16 inverts them.
- `finish` reads the state file on every call now. `docs/specifications/brief/STATE.md` is the
  state file once the crossover lands, so that read is against this repository's own file.
