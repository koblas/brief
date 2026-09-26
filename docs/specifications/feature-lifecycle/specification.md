# Specification: feature lifecycle onboarding

## Intent & Goal

**Primary Goal**: An agent in a repository that ran `brief init` learns brief's whole feature
lifecycle — when to open a feature, what a specification and a step must hold, one step at a
time, review before finish — not just the start/tick/finish step loop; and brief refuses to
finish a step that was never planned.

**Secondary Goals**:
- `brief new step` scaffolds a step `brief start` accepts without a hand edit (the acceptance
  heading, deferred by agent-workflow-skill as a "separate fix").
- Every host sees the loop through brief's own output (scaffolds, stderr hints, `--help`); the
  Claude Code skill and CLAUDE.md snippet carry the fuller lifecycle.

**Out of Scope**:
- New headings or guidance comments in the specification scaffold ("Goal", "Scenarios" are not
  configured names brief computes over).
- `brief status` hints inferred from specification prose ("spec has no scenarios").
- Naming roles in CLI hints (roles may be unbound).
- Byte changes to the shipped planner/implementer/reviewer agents.
- A new host-neutral artifact (AGENTS.md or similar).
- A CLI review-loop contract; review policy stays the adopter's.
- Dogfooding: binding this repository's own `architect`/`developer` via `.brief.yaml` and having
  them preload `brief-workflow` instead of duplicating step-format rules (follow-up chore).
- Older-template recognition for the skill and snippet: no release has shipped them to any
  install, so their bytes change in place and their older lists stay empty (user ruling at
  scenario approval).

## Business Rules & Invariants

- Rule 1: `brief new step` writes the configured acceptance heading, then one blank line, then
  the configured checklist heading, file ending in a single trailing newline. Specification and
  state scaffolds are byte-unchanged.
- Rule 2: `brief start` on an open step names, as shortfalls (exit 0, brief still printed): an
  acceptance section present but whitespace-only; a checklist heading present with zero items.
  Order: acceptance, checklist, then existing state-heading rows. `--json` writes these as
  existing `shortfalls[]` rows and zero stderr bytes.
- Rule 3: `brief finish` refuses (exit 1, no files changed) an **open** step whose checklist
  heading is absent or holds zero items. Evaluated after the done/identity handling, in the
  slot of today's unticked-item check. A done step's identical re-finish stays an idempotent
  success; a done step's divergent re-finish stays the existing divergence refusal.
- Rule 4: `start`'s zero-item row and `finish`'s zero-item refusal count items with one shared
  function in `conform`, recognizing items exactly as `markdown.FirstUnchecked` does (fences,
  nesting). One decision point.
- Rule 5: The acceptance section stays optional for `finish` — empty or absent never refuses.
- Rule 6: `brief check` (including `--hook`) reports nothing new: an open or done step with no
  checklist heading or zero items produces no row.
- Rule 7: Contract amendment to `docs/specifications/brief/specification.md`: R11 gains
  "Finishing an open step whose checklist heading is absent or holds no items is refused."; the
  `new step` command-table row and the approved scenario (~line 393) say the scaffold carries an
  empty acceptance section and an empty checklist. `internal/scaffold/finish.go`'s doc sentence
  "A checklist with no items, or an absent heading, is never refused" is reversed.
- Rule 8: The shipped skill and CLAUDE.md snippet change bytes in place; the shipped-digest test
  pins the new bytes; `olderSnippetTemplates` and the skill's older list stay empty.

## Triage Brief

- Channels today: CLAUDE.md snippet (`internal/platform/artifact/snippet.go`, start/finish only);
  `brief-workflow` skill (`internal/platform/artifact/files/skills/brief-workflow/SKILL.md`, step
  loop only); one-line planner/implementer/reviewer agents with no sequence; `.brief.yaml roles:`
  read only by doctor and `--edit-agents`; CLI help describing on-disk effects only.
- `internal/scaffold/render.go` `specificationSkeleton` = title + progress heading;
  `stepSkeleton` omits `cfg.AcceptanceHeading`, so `brief start` on a fresh step prints
  `no "## Scenario" heading found` (`internal/assemble/assemble.go` requires it).
- Reproduced by product-vision: `new feature` → `new step` → `finish --handoff --state` exits 0
  on a step with no acceptance and no checklist items ("demo is complete"); `check` says no
  findings.
- Prior art: `docs/specifications/agent-workflow-skill/` (shipped skill, agents, doctor checks;
  deferred the scaffold heading). This repo's `.claude/agents/architect` and `developer` are a
  repo-specific planner/implementer (TDD phases, mutation guards, test-stats) — the generic slice
  (step contract, rolling STATE, one step at a time, review before finish) belongs in the skill;
  the method stays repo-local.

## Product Verdict

**SHIP WITH CHANGES** (Phase 1). Skill copy alone cannot fix the unplanned-step hole, so the
smallest CLI change that closes it is in scope. Hints name the next action, never a role. "Any
host" = brief's CLI output, not a new artifact. Review loop = skill copy only; the only brief
fact it states is that a finished step's handoff and state are fixed. Agents byte-unchanged.
Skill keeps the name `brief-workflow` (bound agents' `skills:` lines reference it).

## Surface & Copy

### Step scaffold (`brief new step`, default config)

```
---
id: SCENARIO-01
status: open
depends-on: []
---

# SCENARIO-01

## Scenario

## Implementation Plan
```

### `brief new feature` success line (stderr, exit 0)

`brief new feature: created demo (docs/specifications/demo/specification.md, docs/specifications/demo/STATE.md); write its specification, then add each step with 'brief new step demo'`

`brief new step`'s line is unchanged.

### `brief start` shortfalls (stderr in text mode, exit 0)

| Input | Line | JSON |
|---|---|---|
| acceptance heading absent | unchanged: `brief start: <path>: no "## Scenario" heading found; add a "## Scenario" heading to the step file` | unchanged |
| acceptance present, body whitespace-only | `brief start: <path>: "## Scenario" is empty; write the step's acceptance criteria under it` | row, detail `"## Scenario" is empty` |
| acceptance has any non-whitespace text (an HTML comment counts) | none | none |
| checklist heading absent | unchanged refusal, exit 1 | unchanged |
| checklist present, 0 items | `brief start: <path>: "## Implementation Plan" has no checklist items; add them as "- [ ]" lines before implementing, since brief finish refuses a step with none` | row |
| checklist ≥1 item, ticked or not | none | none |
| no open step | unchanged | unchanged |

Headings in the copy are the configured ones. Fix text carries no second `;`.

### `brief finish` new refusals (exit 1, stderr; `--json` common refusal document, `error.kind: "refusal"`)

| Input | Line | Exit |
|---|---|---|
| open step, checklist heading absent | `brief finish: <path>: no "## Implementation Plan" heading found; add it with the step's checklist items, tick them, and retry (no files changed)` (JSON `line: null`) | 1 |
| open step, heading present, 0 items | `brief finish: <path>:<heading line>: "## Implementation Plan" has 0 checklist items, needs at least 1; add the step's items as "- [x]" lines once done, and retry (no files changed)` | 1 |
| open step, ≥1 unticked | unchanged | 1 |
| open step, all ticked | unchanged success | 0 |
| done step, identical inputs, 0 items or no heading | unchanged idempotent success, no write | 0 |
| done step, different inputs | unchanged divergence refusal | 1 |
| acceptance section empty or absent | no refusal | unchanged |

### `brief check`

No change. Open or done step with no checklist heading or zero items: no row, including `--hook`.

### Help text

- `startLong`: replace "A missing optional convention — the step's acceptance heading, or a
  state file heading — is named on stderr instead" with `A shortfall — the step's acceptance
  heading missing or empty, a checklist with no items yet, or a state file heading missing — is
  named on stderr instead`; rest of the sentence unchanged.
- `finishLong`: append after its first paragraph: `brief finish refuses, changing no files,
  while the step's checklist heading is missing, has no items, or has an item not ticked.`
- Root `--help`: insert after the first line, as its own paragraph (text-only if
  `help_json.go` renders root prose):

```
A feature is a specification, ordered step files and one state file. Open one
with 'brief new feature', write its specification, add steps with 'brief new
step', then take each step from 'brief start' to 'brief finish'.
```

No flag help strings change.

### CLAUDE.md snippet (full block, `{dir}` as rendered today)

```
<!-- brief:begin -->
## brief
Features under `{dir}/` are tracked by `brief`. Multi-step work gets a feature: run
`brief new feature <name>`, write its specification, then add each step with
`brief new step <feature>`. To work on a step, run
`brief start <feature>` and work from its output rather than reading the specification
or earlier steps whole. Close the step with
`brief finish <feature> <step> --handoff <path> --state <path>` — never write a handoff
or tick the progress list by hand. `brief --help` for the rest.
<!-- brief:end -->
```

### Skill (`.claude/skills/brief-workflow/SKILL.md`)

Frontmatter:

```
---
description: brief's feature workflow — open a feature with brief new feature, add steps with brief new step, pick up the next open step with brief start, tick its checklist, close it with brief finish. Use when starting multi-step work, or when planning, implementing or reviewing a step of a brief-tracked feature.
user-invocable: false
allowed-tools:
  - Bash(brief new feature *)
  - Bash(brief new step *)
  - Bash(brief start *)
  - Bash(brief finish *)
  - Bash(brief status *)
  - Bash(brief check *)
---
```

Title becomes `# brief workflow`; existing intro paragraph kept; the new section below goes
before the protocol; the existing numbered protocol follows under `## Step protocol` (item 4
"Add a step" kept); the closing "Never tick…" paragraph unchanged.

```
## Lifecycle

1. **Open.** Multi-step work gets a feature: `brief new feature <name>` writes its
   skeleton. Write the specification — what the feature is for and the acceptance
   criteria that say it is done — before adding steps; brief never writes it for you.
2. **Plan.** `brief new step <feature>` scaffolds each step in order with an
   acceptance heading and a checklist heading; fill both. brief does not decide what
   the steps are: have the list approved by whoever owns the feature before the first
   `brief start`. A step with no checklist items cannot be finished.
3. **One step at a time.** Only one step per feature is open for work: take it from
   `brief start` through `brief finish` before starting the next.
4. **Review before finish.** brief does not decide whether a step is reviewed. If it
   is, review after the checklist is ticked and before `brief finish`, and send
   findings back to whoever implements it. A finished step's handoff and state are
   fixed — `brief finish` refuses different content for it later — so a finding after
   finish becomes a new step.
5. **Roles.** Where `.brief.yaml` binds `roles:`, the planner does step 2, the
   implementer runs the step protocol below, and the reviewer reads with `brief start`
   and `brief check` without editing.
```

### Exit codes

Unchanged except `brief finish`: an open step with the checklist heading absent, or with zero
items, goes from exit 0 to exit 1 (Rule 7 amendment).

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 new step scaffolds the acceptance heading
  Given a feature
  When I run "brief new step <feature>"
  Then the step file carries the configured acceptance heading, a blank line, then the configured checklist heading
  And "brief start" reports no missing-acceptance-heading shortfall for it
  And the specification and state scaffolds are byte-unchanged

Scenario: SCENARIO-02 start names an empty acceptance section and a zero-item checklist
  Given an open step whose acceptance section is whitespace-only, or whose checklist heading holds zero items
  When I run "brief start <feature>" in text mode or with --json
  Then each is named as a shortfall row with the ruled copy, in acceptance-then-checklist order before state rows
  And the exit code is 0 and the brief still prints
  And an acceptance section with any text, or a checklist with at least one item, names nothing

Scenario: SCENARIO-03 finish refuses an unplanned open step
  Given an open step whose checklist heading is absent, or present with zero items
  When I run "brief finish <feature> <step> --handoff <path> --state <path>"
  Then it refuses with the ruled line, exit 1, and changes no files
  And a done step's identical re-finish still succeeds without writing
  And "brief check", including "--hook claude-code", reports no row for such a step
  And start's zero-item hint and finish's refusal count items with the same function

Scenario: SCENARIO-04 CLI copy teaches the loop
  Given the ruled copy
  Then "brief new feature"'s success line names writing the specification before adding steps
  And root "--help" carries the feature-lifecycle paragraph
  And "brief start --help" and "brief finish --help" carry the amended sentences
  And docs/specifications/brief/specification.md records the R11 and new-step amendments

Scenario: SCENARIO-05 skill gains a Lifecycle section
  Given "brief init" for claude-code
  Then ".claude/skills/brief-workflow/SKILL.md" carries the ruled frontmatter, title, Lifecycle section and "## Step protocol"
  And the shipped-digest test pins the new bytes

Scenario: SCENARIO-06 CLAUDE.md snippet names when to open a feature
  Given "brief init"
  Then the CLAUDE.md block carries the ruled "Multi-step work gets a feature" sentence
  And RecognizeSnippet recognizes it as current for any feature directory
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: new step scaffolds the acceptance heading
- [x] SCENARIO-02: start names an empty acceptance section and a zero-item checklist
- [x] SCENARIO-03: finish refuses an unplanned open step
- [x] SCENARIO-04: CLI copy teaches the loop
- [ ] SCENARIO-05: skill gains a Lifecycle section
- [ ] SCENARIO-06: CLAUDE.md snippet names when to open a feature
