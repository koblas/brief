---
id: SCENARIO-19
status: done
---

# SCENARIO-19: A state body missing a required heading is refused

## Scenario

```gherkin
Scenario: SCENARIO-19 A state body missing a required heading is refused  [orig: 06c]
  Given an open step whose checklist is complete
  When I finish it with a replacement state body missing a required heading
  Then the refusal names the missing heading
  And the refusal says an empty section is valid
  And both files are byte-identical
```

## Measured behaviour today (binary built from eea77bc, probe tree in a temp dir)

`brief finish demo SCENARIO-01 --handoff h.md --state s.md`, five state bodies, five identical
results — exit **0**, stderr `brief finish: SCENARIO-01 is done`, files **CHANGED**:

- missing `## Traps` only
- no headings at all (`line 0` / `line 1` / `line 2`)
- all four headings, shuffled order
- all four headings, every section empty
- all four headings **inside a fenced code block**

**Nothing about the state body's structure is enforced today.** `Finish` checks the state
argument's line cap and its fence terminator and nothing else; the body is written to
`cfg.StateFile` verbatim, the step is marked done and the progress entry ticked. Every
subsequent `brief start` on that feature then degrades with SCENARIO-14 shortfalls it can
never repair from the write side. This scenario closes that loop: after 19, a state-heading
shortfall on `start` can only come from a hand-edited file or a pre-19 tree, never from
`finish`.

## Decisions this scenario takes

1. **Presence only. All four required, any order, an empty section is valid.** The trigger is
   `markdown.Section(state, heading).found == false` — exactly SCENARIO-14's `Section.Found`
   rule, no asymmetry between the write path and the read path. The Gherkin mandates the
   leniency outright ("And the refusal says an empty section is valid"), and
   `internal/cli/finish_test.go` lines 168, 187, 286 and 307 already pass a four-heading body
   whose sections are all empty — four existing tests are load-bearing on it.
2. **Order is not enforced.** `assemble.stateSections` reads by name and is order-independent,
   so an order requirement would be enforcement with no consumer; shuffled order is measured
   accepting today, so leaving it accepting is the no-change branch. `Ordered()` is used only
   to define *which* missing heading is reported first — a missing heading has no document
   position, so configured order is the only available meaning of "first".
3. **One refusal line naming the first missing heading**, not one line per missing heading.
   R14a's template is one line, and the existing refusal band (cap, fence, `verdict()`) already
   reports the first thing wrong. SCENARIO-14's one-line-per-shortfall precedent does not
   transfer: 14 is a degrade path that completes and can report everything it found; this is a
   refusal that stops at the first fault and changes nothing.
4. **The check runs after `checkArgumentFence`, ahead of the specification read and
   `(refinish).verdict()`** — a body whose fence never closes cannot have its headings read
   (this is why 18 put it there), and arguments stay checked before any on-disk state is
   consulted, matching 17/18's cap placement.
5. **Reuse, do not rewrite:** `markdown.Section` per `cfg.StateHeadings.Ordered()` entry — the
   same call shape as `assemble.stateSections`. `markdown.Headings` stays unbuilt (STATE.md
   lists it for 20/22); do not reach for it. Four `Section` calls over a body capped at 80
   lines is not worth a single-pass scanner.
6. **Scope.** 19 validates the state *argument* against the configured headings. It does **not**
   validate the configured heading values themselves (STATE.md's "Heading/cap value validation
   — unowned until SCENARIO-19") — that debt stays open and is re-stated in the Handoff with the
   concrete hole it leaves. `start` is untouched (14 owns the degrade there). No open-checklist
   check (20), no dependency check (21), no `check` (22).

**Refusal copy** (rendered by `cli/refusal.go`, `<state>` upgraded to the `--state` path by the
existing `StateSource` branch in `cli/finish.go`):

```
brief finish: /tmp/x/state.md: state is missing the "## Binding decisions" section; add a "## Binding decisions" heading to the state body — an empty section is valid — and retry (no files changed)
```

Deliberately *not* 14's `no %q heading found` wording: STATE.md's trap records that 13's
refusal and 14's shortfall already share a `Detail` string, so these two must be told apart by
text as well as by exit code. Assert the whole line with `Equal`, never `Contains`, and assert
it against the **fixture's** retitled headings (`## Decisions Fixture`, `## Gotchas`), which is
what proves the text is read from configuration rather than hardcoded. `RefusalError.Line` is
0: a missing heading has no line.

**User-visible contract:** exit 1, empty stdout, one stderr line, `(no files changed)` tail
present, nothing on disk touched.

## Fixture updates this scenario forces — enumerated, not estimated

Six pre-existing tests hand `Finish` a state body with no headings and must be repointed:

- `internal/scaffold/finish_cap_test.go` — `Test_accepts_a_state_body_of_exactly_the_configured_cap`,
  `Test_accepts_a_state_body_of_exactly_the_configured_cap_with_no_trailing_newline`
  (`bodyOfLines(20)`; accepting arms).
- `internal/scaffold/finish_idempotent_test.go` — the four `differentState :=
  []byte("DIFFERENT-STATE-BODY\n")` literals, in
  `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state`,
  `Test_re_finishing_a_done_step_with_a_different_state_body_is_refused`,
  `Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first`,
  `Test_a_refused_re_finish_leaves_every_file_byte_identical`. STATE.md's note named only the
  cap fixtures; these four are the under-count.

Unaffected, confirmed by reading the check order — do **not** edit, but confirm empirically:
`finish_test.go:316` `badState` (fence fires first), `cli/finish_test.go:328` `overCapBody(81)`
(cap first), `:448` unterminated fence (fence first), `:378`/`:473` empty `state.md` (usage
error / unreadable handoff, never reach `Finish`), `:533` `"s"` (the no-such-feature refusal
precedes the whole argument band). Every other cli state fixture already carries the four
default headings.

## Implementation Plan

- [x] Step 1: `internal/scaffold/finish_headings_test.go` `Test_refuses_a_state_body_missing_a_required_heading` — `Finish` with a body missing the fixture's `## Gotchas`, pinned to the refusal copy above (red)
- [x] Step 2: `internal/scaffold/errors.go` `ErrMissingStateHeading` — sentinel + doc comment stating presence-only, empty-valid, fence-aware (new)
- [x] Step 3: `internal/scaffold/finish.go` `checkArgumentHeadings` — `markdown.Section` per `cfg.StateHeadings.Ordered()` entry, first `!found` wins, `*RefusalError` with `Line: 0`, named beside `checkArgumentCap`/`checkArgumentFence` (green)
- [x] Step 4: `internal/scaffold/finish.go` `(*Server).Finish` — call it immediately after `checkArgumentFence`, before the specification read (green)
- [x] Step 5: `go test ./internal/... 2>&1` — record the **full list** of now-failing pre-existing tests and compare it against the six enumerated above; report any test the enumeration missed rather than silently fixing it
- [x] Step 6: `internal/scaffold/finish_test.go` `differentStateBody` — helper beside `newStateBody`: the four configured headings each carrying `DIFFERENT-STATE-ENTRY`, distinct from `NEW-`/`OLD-STATE-ENTRY` (new)
- [x] Step 7: `internal/scaffold/finish_idempotent_test.go` — repoint all four `differentState` literals at `differentStateBody(fx.cfg)` (update)
- [x] Step 8: `internal/scaffold/finish_cap_test.go` `stateBodyOfLines` — helper emitting the four configured headings plus distinct filler to exactly n lines; repoint both at-cap state tests (update)
- [x] Step 9: `internal/scaffold/scaffold_test.go` — re-check `fixtureConfig`'s comment about the "16-line maximum of every state body" against the new helpers; correct it if it now misstates the fixtures (update)
- [x] Step 10: `finish_headings_test.go` `Test_accepts_a_state_body_whose_sections_are_all_empty` — presence, not content, is the rule (pin)
- [x] Step 11: `finish_headings_test.go` `Test_accepts_a_state_body_whose_headings_are_out_of_configured_order` — records decision 2 as a green test, not only as prose (pin)
- [x] Step 12: `finish_headings_test.go` `Test_accepts_the_state_body_new_feature_writes` — `NewFeature` into a bare `t.TempDir()` root, read `cfg.StateFile` **off disk**, hand those bytes to `Finish`; the body must come from production, never re-derived in the test, or it pins nothing (pin)
- [x] Step 13: `finish_headings_test.go` `Test_refuses_a_state_body_whose_headings_are_only_inside_a_fenced_block` — the measured case that succeeds today; proves `Section`, not a substring scan (red on arrival only if Step 3 got it wrong — say which)
- [x] Step 14: `finish_headings_test.go` `Test_refuses_an_empty_state_body_naming_the_first_configured_heading` — the degenerate body (pin)
- [x] Step 15: `finish_headings_test.go` `Test_names_the_first_configured_heading_when_several_are_missing` — body missing the 1st and 3rd configured headings, names the 1st (pin)
- [x] Step 16: `finish_headings_test.go` `Test_refuses_a_state_body_missing_only_the_last_configured_heading` — proves the loop covers every position, not just index 0; Step 15 alone passes with only `Ordered()[0]` checked (pin)
- [x] Step 17: `finish_headings_test.go` `Test_reports_the_state_cap_before_a_missing_heading` — over-cap **and** headingless body reports `ErrOverCap`, `NotErrorIs` the heading sentinel (pin)
- [x] Step 18: `finish_headings_test.go` `Test_reports_the_state_s_unclosed_fence_before_a_missing_heading` — unterminated fence **and** headingless body reports `ErrUnterminatedFence` (pin)
- [x] Step 19: `finish_headings_test.go` `Test_a_state_body_missing_a_heading_on_a_done_step_reports_the_heading_not_the_re_finish_refusal` — `newFinishedFixture`, `NotErrorIs` `ErrAlreadyFinished` (pin)
- [x] Step 20: `finish_headings_test.go` `Test_a_refused_missing_heading_leaves_every_file_byte_identical` — `pinModTimes`/`snapshotTree`/`modTimes` over all four names, both probes, control arms cited from `finish_idempotent_test.go` not duplicated (pin)
- [x] Step 21: `internal/cli/finish_test.go` `Test_refuses_a_state_body_missing_a_heading_and_names_the_state_path` — CLI slice through `cli.Run`: exit 1, empty stdout, whole-line `Equal` on stderr with the real `--state` path and the `(no files changed)` tail (new)
- [x] Step 22: `internal/cli/finish.go` `finishUsage` — one clause on the `--state` line saying it must carry the configured state headings and that a section may be empty; no other cli change, the `StateSource` branch at line 100 already upgrades the path (update)
- [x] Step 23: `internal/scaffold/finish.go` `(*Server).Finish` doc comment — **two** edits: delete the now-false "neither argument is checked against a required-heading schema" clause, and insert the heading step into the ordered check enumeration between the fence sentence and "the specification is readable" (update)
- [x] Step 23a: `internal/scaffold/doc.go` — add `ErrMissingStateHeading` to the package doc's sentinel enumeration, and `ErrOverCap` with it: 17/18 added that sentinel and never listed it, so the contract `go doc ./internal/scaffold` prints is already one short (update)
- [x] Step 24: mutation — delete the `checkArgumentHeadings` call from `Finish`; Steps 1, 13, 14, 15, 16 go red; restore and `diff` byte-identical
- [x] Step 25: mutation — tighten the predicate to `!found || strings.TrimSpace(body) == ""`; Step 10 goes red and Step 1 stays red for the right reason; restore
- [x] Step 26: mutation — replace `markdown.Section` with `strings.Contains(string(state), heading)`; Step 13 goes red; restore
- [x] Step 27: mutation — reverse the iteration over `Ordered()`; Step 15 goes red. Then, separately, check only `Ordered()[0]`; Step 16 goes red. Run them one at a time — Step 15 alone is vacuous, exactly as 17/18's cap-order test was
- [x] Step 28: mutation — move the heading check above `checkArgumentCap`; Step 17 goes red. Separately, move it above `checkArgumentFence`; Step 18 goes red. Separately, move it below the `verdict()` switch; Step 19 goes red. One at a time, restoring between
- [x] Step 29: run the standing brief's verification block from the repo root, unpiped, racing `./internal/scaffold/...` and `./internal/cli/...` — report the exact test count and the delta
- [x] Step 30: `docs/specifications/brief/specification.md` — tick SCENARIO-19 in `## BDD Acceptance Progress`; rewrite `docs/specifications/brief/STATE.md` folding this scenario's Handoff in and dropping what it supersedes

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **The state heading check is presence-only, any order, empty sections valid** — the Gherkin
  requires the refusal to say an empty section is valid, and `cli/finish_test.go`:168/187/286/307
  already finish successfully with four empty sections. Trigger is `markdown.Section(...).found
  == false`, identical to SCENARIO-14's `Section.Found` rule; write path and read path agree.
- **Order is deliberately unenforced** — `assemble.stateSections` reads by name, so an order rule
  would have no consumer. `Ordered()` only decides which missing heading is named first.
  `Test_accepts_a_state_body_whose_headings_are_out_of_configured_order` is the record; 20/21/22
  must not quietly tighten it.
- **One refusal line, naming the first missing heading in `Ordered()` order** — R14a is one line
  and the band already reports the first fault. 14's one-line-per-shortfall shape belongs to the
  degrade path only.
- **`checkArgumentHeadings` sits between `checkArgumentFence` and the specification read** — after
  the fence (an open fence makes headings unreadable) and ahead of `(refinish).verdict()` (17/18's
  rule that arguments are checked before disk). SCENARIO-20's open-checklist refusal reads the
  *step file*, not an argument, so it belongs after this band, not inside it.
- **This narrows R11.** An identical re-finish is a true no-op only when the bytes also carry the
  four headings; a done step whose recorded state predates 19 now refuses rather than no-opping.
  This repository's own `docs/specifications/brief/STATE.md` does carry all four, so 19 adds no
  new self-hosting lockout beyond the 80-line cap one 18 recorded.
- **Refusal wording is `state is missing the %q section`**, deliberately unlike 14's `no %q heading
  found` — those two must stay distinguishable by text, per the 13/14 shared-`Detail` trap.

**Left unbuilt** — named so nobody assumes it exists:

- `markdown.Headings` — still unbuilt; 20/22 own it. 19 uses four `markdown.Section` calls, which
  is also what makes it fence-aware; a `strings.Contains` scan would pass every test here but the
  fenced-headings one.
- Validation of the *configured* heading values (`state-headings:` set to `""`, to duplicate
  strings, or to text with no leading `#`) — unowned, and 19 does not take it. Same for cap values.
- Any heading check on the handoff argument, on the `start` read path, or on the specification —
  not built, not planned here.
- `ErrMissingStateHeading` has no `check` (22) counterpart yet; R18's backstop for pre-19 state
  files is still unwritten.

**Traps** — things that look right and are not:

- **`markdown.Section(body, "")` returns `found == true`.** `findHeading` compares
  `trimEOL(line) == heading`, so an empty configured heading matches the first blank line,
  `headingLevelOf` returns 0, and the end scan (`lvl > 0 && lvl <= 0`) never fires — the "section"
  runs to end of file. A repo configuring `traps: ""` passes this check silently. Not fixed here;
  it is the concrete hole under the unowned heading-value-validation debt.
- **`Test_names_the_first_configured_heading_when_several_are_missing` is vacuous on its own** —
  it also passes when only `Ordered()[0]` is ever checked. Its twin (missing only the last
  heading) plus the reverse-iteration mutation are the real proof. Same failure shape 17/18 paid
  for with the cap-order test.
- **STATE.md's fixture warning under-counted.** It named the cap fixtures; the four
  `differentState := []byte("DIFFERENT-STATE-BODY\n")` literals in `finish_idempotent_test.go`
  were the larger half, and two of them are *refusal* arms that would have started reporting the
  wrong sentinel rather than failing loudly. Enumerate by running the suite, never by reading one
  file.
- **`cli/finish_test.go`'s bare `""` and `"s"` state bodies never reach the argument band** — the
  usage error, the unreadable-handoff error and the no-such-feature refusal all precede it. They
  look like fixtures 19 broke and are not. Do not "fix" them.
