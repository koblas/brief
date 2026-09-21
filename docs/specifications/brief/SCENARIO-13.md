# SCENARIO-13: Start refuses a malformed feature rather than assembling half a brief

## Scenario

```gherkin
Scenario: SCENARIO-13 Start refuses a malformed feature rather than assembling half a brief  [orig: 03c]
  Given a feature directory whose specification is missing its progress list
  When I start that feature
  Then the command refuses, names what is missing and the command that fixes it
  And no partial brief is returned
  # A missing handoff file is deliberately NOT this scenario: under the model it means
  # "not done", which is an ordinary state, not a malformed feature.
```

## Measured behaviour before this scenario

Built `./cmd/brief` and ran `brief start demo` against a `brief new feature demo` +
`brief new step demo` tree, one malformation at a time. Baseline (well-formed): exit 0,
46 stdout bytes, stderr empty.

| fixture | exit | stdout | stderr |
| --- | --- | --- | --- |
| specification file absent | **0** | **full brief (46 B)** | — |
| specification present, progress heading absent | **0** | **full brief (46 B)** | — |
| specification unreadable (chmod 000) | **0** | **full brief (46 B)** | — |
| progress heading present, list empty | 0 | full brief | — |
| step frontmatter unparseable YAML | 1 | 0 B | `brief start: assemble: stepfile: parse frontmatter: yaml: line 1: …` |
| step file with no `---` delimiter | 1 | 0 B | `brief start: assemble: no frontmatter found` |
| frontmatter without `id:`/`status:` | **0** | **brief with an empty id: ` — 0 done, 1 open`** | — |
| state file absent | 1 | 0 B | `brief start: malformed feature` |
| state file unreadable | 1 | 0 B | `brief start: malformed feature` |
| state file present, all four headings absent | 0 | full brief | — |
| no step files at all | 0 | 0 B | 12's `no step files yet` notice |
| unknown feature | 1 | 0 B | `brief start: no such feature` |

Genuine reds: the three specification rows and the missing-`id` row. The frontmatter and
state-file rows already exit 1 with empty stdout, but their stderr names **no path and no
fix** — the Gherkin's "names what is missing and the command that fixes it" is unmet, so
they are reshaped, not re-refused. The empty-progress-list and absent-state-heading rows
stay exit 0 (see the boundary below).

## The 13/14 boundary (pin this before writing code)

`## Scope` names the model: a feature is *one specification, an ordered set of steps, a
state file*; a step is *a file with an id and a checklist*; progress is *a list in the
specification*. `## Configuration` adds: "The structural requirements above are not among
[the optional conventions] and cannot be switched off." R2 repeats the list verbatim.

**Structural — 13 refuses, exit 1, stdout byte-empty:**

1. specification file absent or unreadable
2. specification carrying an unclosed fence (its progress heading is unreadable past it)
3. specification without the configured progress heading
4. state file absent or unreadable *(exists today, reshaped)*
5. state file carrying an unclosed fence *(exists today, reshaped)*
6. a step file whose frontmatter is absent or does not parse *(exists today, reshaped)*
7. the briefed step's frontmatter carrying no `id:`
8. the briefed step carrying no checklist heading

**Optional — 14 degrades, exit 0, brief on stdout, shortfall on stderr:** the
`cfg.AcceptanceHeading` section absent from the briefed step; anything named in
`cfg.OptionalConventions`; one of the four state headings absent from an otherwise
readable state file.

**Neither:** a missing handoff file (= "not done"); zero step files and every-step-done
(12's exit-0 notices); an empty-but-present progress list or checklist; an `id:` whose
value does not match the filename (11's decision, a `check` finding).

One-line justification to carry into review: **13 refuses where the omission is
undetectable or the brief unassemblable; 14 degrades where the shortfall is nameable.**
That is why an unclosed fence is a refusal and an absent heading is a degrade — after a
fence opens, "heading absent" and "heading swallowed" are indistinguishable, so no honest
notice can be written.

R14a's template requires `<imperative next action>`, not a command. The Gherkin's "the
command that fixes it" is satisfied by a command where one exists (the step-file case) and
by an imperative where none does (the progress-heading case) — reviewers should read it
against R14a, not against the Gherkin's shorthand.

## Refusal shape

R14a: "Read refusals drop the `(no files changed)` tail." `start`'s existing read refusals
(`ErrNoSuchFeature`, `ErrMalformedFeature`) already render tail-free through
`renderRefusal`'s generic branch. Follow that precedent; invent no second form:

```
brief start: <absolute path>[:<line>]: <problem>; <imperative next action>
```

Wording for the progress-heading case is already written on the write path —
`scaffold.progressRefusal` renders `no "## BDD Acceptance Progress" heading found` /
`add a "## BDD Acceptance Progress" heading to the specification`. Reuse it verbatim
(re-typed, since `assemble` must not import `scaffold`) so `start` and `new step` say the
same thing about the same file. Wording for the step-file case is already in
`assemble.newProblem`; reuse it as-is.

**Path form** (12's rule: a specific on-disk object → absolute; a configured location →
config-relative). Every one of 13's refusals names an on-disk object, so all eight are
**absolute**: refusals 1–3 name `<root>/<feature-dir>/<feature>/specification.md`, 4–5 name
`…/STATE.md`, 6–8 name `…/SCENARIO-NN.md`. `Line` is set only for the two unclosed-fence
refusals; everything else is whole-path (`Line == 0`). Nothing in 13 prints a
config-relative path.

Not fixed here: `config.InvalidConfigError` still renders **with** the `(no files changed)`
tail on `start`, contradicting R14a's read-refusal sentence. It is shared with every write
command, so dropping the tail for reads alone needs `renderRefusal` to know read from write.
Pre-existing, out of 13's scope, unowned.

`errname` is enabled (`.golangci.yaml` sets `default: all` and does not disable it), so the
error type must be named `…Error`. `assemble.Problem` cannot become the error type; Step 5
adds `assemble.RefusalError` instead.

## R1 bootstrap interaction

`brief start brief` in this repo fails **today**: exit 1, `brief start: assemble: no
frontmatter found`, because `SCENARIO-01.md`…`-12.md` carry no frontmatter. Measured:
`specification.md` has `## BDD Acceptance Progress` at line 808 and 8 column-0 fences (even,
balanced); `STATE.md` has none. So checks 1–5 all **pass** on this repo's own tree and 13
adds **no new lockout** — the same refusal fires, for the same file, with a better message
(names the absolute step-file path and the fix). Net: unchanged in outcome, better
diagnosis. Do not touch this repo's own feature tree here — the crossover owns adding
frontmatter, and check 7 makes that crossover's job stricter (see Handoff).

## Implementation Plan

- [x] Step 1: `internal/cli/start_test.go` `newStartFixture` — write a conforming
      `specification.md` (progress heading + one `- [ ] SCENARIO-01` entry) into the
      fixture, so `Test_prints_the_brief_and_writes_nothing_to_stderr` becomes 13's control
      arm (update, green on arrival)
- [x] Step 2: `internal/cli/start_test.go` — add the same conforming specification to the
      two inline fixtures in `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature`
      and `Test_start_still_refuses_a_feature_with_no_state_file`, so 12's exit-0 notice is
      not regressed and the latter still isolates the *state* cause; tighten its stderr
      assertion to name `STATE.md` (update)
- [x] Step 3: `internal/assemble/assemble_test.go` — add a conforming `SPEC.md` to the four
      inline fixtures that lack one (`Test_returns_an_error_when_the_state_file_is_missing`,
      `Test_refuses_a_state_file_whose_fence_is_unterminated`,
      `Test_returns_an_error_when_a_step_file_has_no_frontmatter`,
      `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`); make the
      second frontmatter test use unparseable YAML rather than duplicating the first's
      fixture byte-for-byte (update)
- [x] Step 4: `internal/assemble/assemble_test.go`
      `Test_refuses_a_specification_with_no_progress_heading` — `Start` against a fixture
      identical to `newFixture` except the progress heading; asserts `ErrMalformedFeature`,
      an absolute `SPEC.md` path, the configured heading text, and a zero `Brief` (red)
- [x] Step 5: `internal/assemble/errors.go` — add `RefusalError` (embedding the existing
      `Problem`, plus `Line int` and `Err error`, with `Error()` and `Unwrap()`), the typed
      read refusal mirroring `scaffold.RefusalError`; move `Problem`/`newProblem` here from
      `status.go` and reuse `newProblem` to build both. `FeatureStatus.Problem` keeps its
      `*Problem` type and 11's shape. **Each refusal's `Err` is the sentinel its own test
      asserts** — `ErrMalformedFeature` for the specification and state-file refusals, the
      `stepfile` sentinel for the frontmatter one; `Unwrap` is singular, so do not unify
      them. Closes STATE's "assemble's sentinels duplicate scaffold's" debt (new)
- [x] Step 6: `internal/assemble/assemble.go` `(*Server).Start` — read
      `cfg.SpecificationFile` through the already-opened feature `*os.Root`, the same way
      `StateFile` is read; refuse absent (distinct fix from unreadable), unclosed fence
      (with its line), and absent progress heading via `markdown.Section`, all before any
      `Brief` field is populated (green)
- [x] Step 7: `internal/cli/refusal.go` `renderRefusal` — add an `*assemble.RefusalError`
      branch rendering `brief <cmd>: <path>[:<line>]: <detail>; <fix>` with **no**
      `(no files changed)` tail, ordered beside the existing `*scaffold.RefusalError`
      branch; extend the doc comment to say why the read form drops the tail (new)
- [x] Step 8: `internal/cli/start_test.go`
      `Test_start_refuses_a_specification_with_no_progress_heading` — builds
      `newStartFixture(t, "open")` and overwrites **only** `specification.md` with a body
      lacking the progress heading, so it differs from Step 1's control arm in exactly one
      variable; through `cli.Run`: exit 1, `stdout.Len() == 0`, exactly one stderr line, no
      `(no files changed)` suffix, line contains the absolute specification path and the
      configured heading (red)
- [x] Step 9: `internal/assemble/assemble.go` + `assemble_test.go` — reshape the existing
      state-file refusals (absent, unreadable, unclosed fence) into `*Problem` naming the
      absolute `STATE.md` path, `Line` set only for the fence; keep
      `errors.Is(err, ErrMalformedFeature)` true for both (update)
- [x] Step 10: `internal/assemble/assemble.go` + `assemble_test.go` — reshape the step-file
      frontmatter refusal into `*Problem` naming the absolute step-file path, reusing
      `newProblem`'s existing detail and fix wording so `start` and `status` agree on the
      same file; keep `errors.Is(err, stepfile.ErrNoFrontmatter)` true (update)
- [x] Step 11: `internal/assemble/assemble_test.go`
      `Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id` — a step file with
      `status:`/`depends-on:` but no `id:`; today this renders a brief whose id is the empty
      string (red)
- [x] Step 12: `internal/assemble/assemble.go` — refuse an empty `fm.ID` on the **briefed**
      step only; presence, never format, and never compared against `pattern.ID(n)` (green)
- [x] Step 13: `internal/assemble/assemble_test.go`
      `Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent` — the briefed
      step carries the acceptance section but no `cfg.ChecklistHeading` (red)
- [x] Step 14: `internal/assemble/assemble.go` — refuse on `Checklist.Found == false` for
      the briefed step only; `Test_a_section_distinguishes_present_but_empty_from_not_found_at_all`
      is the standing control that a present-but-empty checklist stays conforming (green)
- [x] Step 15: `internal/assemble/doc.go` and `internal/cli/start.go` `startUsage` — state
      the refusal contract as a rule (what `start` refuses vs what it reports at exit 0);
      no scenario ids, no history (update)
- [x] Step 16: mutation verification, **one check at a time**, each stashed per the standing
      brief. Neuter check 3 → Step 8's test must redden **on its stdout-emptiness
      assertion**: with the check gone the feature is otherwise well-formed, so the same
      probe sees the full 46-byte brief — that is the control proving the assertion is not
      vacuous. Neuter check 7, then check 8 → Steps 11 and 13 redden on their `ErrorIs`
      assertions (those are `assemble`-package tests and have no stdout; do not add cli
      tests for them — one stdout proof is what the scenario needs). Optional second proof
      that the refusal precedes output: return a populated `Brief` alongside the refusal
      **and** hoist `assemble.RenderText` above `runStart`'s error branch — both edits are
      needed, since `RenderText` writes nothing for a nil `Step`
- [x] Step 17: run the standing brief's verification set; the delta for this scenario is
      `-race` on `./internal/assemble/...` and `./internal/cli/...`. Report the exact test
      count and the delta; then tick SCENARIO-13 in `specification.md` and rewrite `STATE.md`
      from this Handoff

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Structural (13 refuses, exit 1, stdout byte-empty) = specification present + readable +
  fence-closed + carrying the progress heading; state file present + readable +
  fence-closed; the **briefed** step's frontmatter parseable with a non-empty `id:`; the
  briefed step's checklist heading present. Everything else about a feature is either
  optional (14) or a `check` finding (22) — SCENARIO-14 is unbuildable if this list grows.
- Optional (14 degrades, exit 0 + stderr notice) = `cfg.AcceptanceHeading` absent, anything
  in `cfg.OptionalConventions`, a state heading absent from a readable state file. 14's red
  is concretely: delete the `## Scenario` line from `newStartFixture`'s step file and assert
  the brief still lands on stdout at exit 0 with one stderr line naming the shortfall.
- Refuse where the omission is undetectable or the brief unassemblable; degrade where the
  shortfall is nameable. This is why an unclosed fence refuses and an absent heading does not.
- Read refusals carry **no** `(no files changed)` tail (R14a, and `start`'s existing
  `ErrNoSuchFeature`/`ErrMalformedFeature` already render tail-free). `*assemble.RefusalError`
  is the read-side `*scaffold.RefusalError`; `cli/refusal.go` renders it tail-free. It must
  be named `…Error`: `errname` is on (`default: all`, not disabled).
- Check order is specification → state file → step files → briefed step, first failure wins.
  Every refusal fires before any exit-0 notice does — 14's degrade notices and 22's findings
  both sit downstream of the whole list.
- All eight refusals name an **absolute** path; `Line` is set only for the two unclosed-fence
  cases. No config-relative path appears in 13's output.
- Present-but-empty stays conforming for both the progress list and the checklist — `new
  step` writes an empty `## Implementation Plan`, so refusing on empty would break
  `new step` → `start` on a freshly scaffolded feature.
- The `id:` check is **presence only**, on the briefed step only. It is never compared to
  `pattern.ID(n)`: 11 ruled an id/filename mismatch conforming and gave it to `check` (22).

**Left unbuilt** — named so nobody assumes it exists:

- A refusal for a missing `status:` key — deliberate: `Frontmatter.Done()` defaults it to
  "open" and `finish` refuses with `ErrNoStatusField` at the write path (R18). Unowned.
- `ErrNoSuchFeature` still does not list the known features (R14 says it should). Unowned.
- `status` does not adopt 13's checks; a feature with no specification is still
  `demo 0/1 SCENARIO-01 0` to `status` and exit 1 to `start`. Owner: `check` (22) — reuse
  the unexported specification-validation helper 13 extracts in `assemble`.
- The `config.InvalidConfigError` tail inconsistency (see *Refusal shape*) — unowned.

**Traps** — things that look right and are not:

- The fix clause must never say `brief new feature <name>` for a feature directory that
  exists — SCENARIO-08 refuses that with `ErrFeatureExists`. A missing specification's fix
  is an imperative, not a command.
- `assemble.RefusalError`'s text field is `Detail` (embedded from `Problem`);
  `scaffold.RefusalError`'s is `Problem`. The two types are knowingly duplicated
  (`assemble` must not import `scaffold`) and the progress-heading wording is duplicated
  with them — keep the strings identical by hand.
- `RenderText(Brief{})` writes **nothing** (it returns early on a nil `Step`). Any
  "no partial output" mutation that only reorders the render proves nothing; see Step 16.
- Six existing tests build feature fixtures with no specification file (two in
  `internal/cli/start_test.go`, four in `internal/assemble/assemble_test.go`). Without
  Steps 1–3 they either regress SCENARIO-12 or keep passing for the wrong reason.
- **The crossover must write `id:` as well as `status:`** into each `SCENARIO-XX.md`
  frontmatter. Check 7 means a crossover that adds frontmatter without `id:` leaves
  `brief start brief` refused for a new reason. 13 itself adds no new lockout: this repo's
  `specification.md` has the progress heading and balanced fences, `STATE.md` has no fences.
- `markdown.Section` (fence-aware) is what finds the progress heading here;
  `scaffold.progressSection` is an exact-line match. A progress heading that appears only
  inside a fence is refused by `start` and accepted by `new step`. Unowned divergence.
