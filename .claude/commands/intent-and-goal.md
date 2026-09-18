---
description: The scoping procedure for any new feature or behavior change — triage the request against the codebase, refine the intent, get a product verdict, generate Gherkin scenarios with IDs, and create a Source of Truth (SoT) specification file. CLAUDE.md enters this automatically whenever the user asks for new or changed behavior; type it to force it.
argument-hint: <optional — brief description of the feature; omit to use the conversation>
allowed-tools: Read, Write, Glob, Grep, Skill, Agent
---

## The request

**$ARGUMENTS** non-empty → that is the request.

Empty → this procedure was entered implicitly (see pipeline trigger in `.claude/CLAUDE.md`)
and **the request is the user's own words in the conversation**. Take it verbatim. Do not ask
the user to restate what they just said. Do not treat the empty argument as missing input.

Either way: state in one line what you treat the request as, note triage is running, continue.
Do not stop for confirmation here — the approval gate is at end of Phase 2.

## Phase 0: Triage (before asking the user anything)

Run **`triage`** on the request. Read-only. Returns affected commands/packages, prior art
already in the repo, a reproduction if this is a bug, what already exists, what must be built,
and the genuine open questions.

Ask the user only what triage could not answer from the code. Starting from facts instead of
guesses is the point of this phase.

## Phase 1: Intent & Goal Refinement

1. Ask the user the open questions triage surfaced — the "Why" and "Who" behind the request.
2. Define **Primary Goal** (main business value).
3. Identify **Secondary Goals** or constraints (security, performance, audit, etc.).
4. Run **`product-vision`** on the refined intent, passing the triage brief. Report its verdict:
   - **DON'T BUILD** or **RETHINK** — stop, put it to the user before going further.
   - **SHIP WITH CHANGES** — fold changes into the intent before Phase 2.
   - **SHIP** — continue.
5. Summarize refined intent, ask: "Does this capture it correctly? I'll move on to proposing
   scenarios."

## Phase 2: Scenario Generation

Intent confirmed → automatically:

1. **Invoke `clean-architecture` skill** for folder structure and conventions.
2. Read existing domain models in the domain source directory.
3. Read existing use cases in the use case source directory.
4. Propose Gherkin scenarios with unique IDs (`SCENARIO-01`, `SCENARIO-02`, …).
5. Ask clarifying questions if business rules ambiguous.
6. Iterate with the user — add, remove, refine scenarios as needed.
7. Wait for explicit user approval before Phase 3. **This is the one blocking gate in the whole
   pipeline** — everything after it auto-continues, and this procedure is usually entered
   implicitly rather than typed. Never write `specification.md` from an offhand request without
   approval here.

### Scenario format

```gherkin
Scenario: <clear description>
  Given <precondition>
  When <action>
  Then <expected outcome>
```

### What to cover

- **Happy path** — primary success case first.
- **Empty state** — no data, no matches, no candidates.
- **Edge cases** — boundaries, thresholds, equal values, min/max limits.
- **Error scenarios** — invalid input, dependency unavailable, malformed data.

### Scenario rules

- Business-domain language; avoid generic CRUD wording.
- One behavior per scenario.
- Reuse existing domain objects where possible.
- No implementation details or architecture in this phase.

## Phase 3: SoT Creation

On approval create `docs/specifications/<feature-slug>/` and write the specification inside it.

### Folder structure

```
docs/specifications/<feature-slug>/
  specification.md          # SoT — intent, rules, scenarios, progress
  SCENARIO-01.md            # Created later by the architect agent
  SCENARIO-02.md            # Created later by the architect agent
```

Only `specification.md` in this phase. Scenario plan files come from the architect agent.

### Specification Template

```markdown
# Specification: <Feature Name>

## Intent & Goal

**Primary Goal**: <main business value>

**Out of Scope**: <explicitly excluded concerns>

**Business Rules**: <rules and constraints identified in Phase 1>

## Business Rules & Invariants
- Rule 1: ...

---

## Triage Brief
<From the `triage` agent — affected surface, prior art to follow, and especially
"Already exists — do not re-plan". The architect treats this as binding.>

## Product Verdict
<The `product-vision` verdict and, for SHIP WITH CHANGES, the accepted changes. The
architect folds these into the scenario plans rather than deferring them.>

---

## Scenarios (Gherkin)
<Approved scenarios from Phase 2>

---

## BDD Acceptance Progress
- [ ] SCENARIO-01: <Title>
- [ ] SCENARIO-02: <Title>
```
