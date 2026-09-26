# Planning

For `architect` (writes plans) and `developer` (executes and ticks them). Orchestrator reads *Coverage* when writing fix-pass brief.

## Scenario plan files are brief step files

`docs/specifications/<feature>/SCENARIO-XX.md` read by `brief` itself (`brief status`, `brief check`). Architect writes it starting with frontmatter, then heading, checklist under `## Implementation Plan`, grouped under `### Red`, `### Green`, `### Sweep`, `### Verify` subheadings (see architect plan format):

```markdown
---
id: SCENARIO-XX
status: open
---

# SCENARIO-XX: <title>
```

Developer sets `status: done` when scenario complete, plus tick in `specification.md`. Every `- [ ]` under `## Implementation Plan` must be ticked by then — `brief check` reports unticked item on done step.

## TDD shape of a plan

Phases batch *when* tests run, not *whether* tests lead. Every behaviour change in Green has its test in Red, and Red runs — failing at assertion — before any Green edit. Sweep holds only behaviour-neutral chores (docs, lint-listed cases, count bumps); production branch planned in Sweep is TDD bypass. FOLDed scenario's steps land in absorbing plan's Red and Green like any other — merge changes scenario boundaries, not test-first order.

## Coverage the gate will demand

Commonest blocking findings share one shape: fallible call in new code with no fault test, or numeric bound with no outside-the-bound test. Each costs fix pass plus re-gate for test architect could have listed up front. Every architect checklist for new or changed command, feature-package method or adapter carries, as named **Red** steps:

- **One fault test per fallible call** — each `Store` call, file read/write/rename, `exec`, and parse the code makes. Include call that re-reads on resume or retry branch, not just first one.
- **Every numeric bound tested just outside it**, in-bound case as control (line caps, count limits, depth limits).
- **Every fallback branch of error → exit-code mapper** — the `default:` arm, not just named sentinels.
- **One decode-fault test per decoded record kind** for adapter or parser reading files — frontmatter as well as body items. Corrupt-child-item test on a read does not cover corrupt root on same read.
