# Clean Architecture & TDD Playbook

## Workflow rules

- **Step 0 — fresh worktree (MANDATORY, before any feature work):** every pipeline run
  starts from clean, current default branch in isolated worktree — never shared
  checkout's current branch.
  1. Already in worktree → skip.
  2. Else call **`EnterWorktree`**, `name` = short feature slug. `worktree.baseRef: fresh`
     (in `settings.json`) branches off `origin/<default-branch>` after fetch, so worktree
     starts from current master on own branch regardless of main checkout's branch. All
     pipeline artifacts live inside it.
  3. **Do not warm deps yourself.** `PostToolUse` hook on `EnterWorktree` runs
     `.claude/warm-deps.sh` detached + silent if present. Gradle/Maven repos need none
     (global cache, no-op). To confirm deps ready read one-line
     `.claude/warm-deps.status` (`ok` = ready), never `.claude/warm-deps.log`.

- **Pipeline is default path, not command user must remember.** User asks for behavior
  that does not exist, or change to behavior that does → enter pipeline **without being
  asked**, starting Step 0 then scoping below. `/intent-and-goal` still typable to force
  it, but it is the procedure, not the trigger.

  **Triggers pipeline:**
  - New user-visible behavior — feature, screen, flow.
  - New API surface — RPC, endpoint, message, field, event.
  - Change to existing contract or existing user-visible behavior.

  **Does NOT trigger pipeline** — do the work:
  - Questions about how existing behavior works.
  - Named bug, known fix, no contract change.
  - Refactors, renames, cleanups with no behavior change.
  - Config, deps, CI, tooling, docs, `.claude/` itself.
  - Already covered by spec under `docs/specifications/` — resume that spec at first
    unchecked scenario instead of starting new one.

  In doubt → run **`triage`** first, decide from what it finds. Triage read-only and
  cheap; spec file nobody asked for is not.

  **Announce, don't block.** On implicit start say in one line what you treat request as
  and that triage runs — then proceed. Cheap redirect, no blocking question.

- **Scenarios first**: triage, refine intent, get product verdict, propose Gherkin
  scenarios, write Source of Truth (SoT) specification file before any code.
  `commands/intent-and-goal.md` holds detailed procedure.

- **Sequential pipeline.**

  **Scoping (the `intent-and-goal` procedure):**
  1. **`triage`** — read-only. What exists, what's affected, what's unknown.
  2. **`product-vision`** — right thing, right shape, named right?
     Verdict: SHIP / SHIP WITH CHANGES / RETHINK / DON'T BUILD. RETHINK and DON'T BUILD
     stop pipeline, go back to user.
  3. Scenarios approved → `specification.md` written, carrying triage brief + product
     verdict.

  **For each scenario in order (top-to-bottom in `## BDD Acceptance Progress`):** 4. Run **`architect`** to plan it (produces `SCENARIO-XX.md`). 5. Run **`developer`** to implement it. 6. Next unchecked scenario.

  **After all scenarios implemented:** 7. Run **`/run-reviewers`** (once, no arguments) on all changed files. 8. **FAIL** (any BLOCKER or MAJOR) → run **`developer`** in fix mode with consolidated
  findings, one pass. 9. Run **`/run-reviewers`** again. Repeat until PASS or PASS WITH FOLLOW-UPS. 10. Run **`product-vision`** once more on finished surface — proto, endpoint, UI states,
  error copy. It reviews design that shipped, not design proposed. Verdict has
  consequences: - **SHIP** — done. - **SHIP WITH CHANGES** — hand changes to **`developer`** in fix mode as MAJOR
  findings, then re-run `/run-reviewers` (step 7). - **RETHINK** / **DON'T BUILD** — stop, put it to user. Surface already shipped, so
  this is a conversation, not a silent revert.

  **Rules:**
  - **Every agent in this pipeline counts as user-requested.** A harness rule may say not
    to spawn subagents unless the user asks. Entering the pipeline IS that ask: `triage`,
    `product-vision`, `architect`, `developer`, and the reviewers `/run-reviewers` drives
    are the procedure, not an optional delegation. Spawn them without asking permission
    first. Do not silently run the pipeline "inline" instead — a pipeline whose reviewer
    gate never ran is not a pipeline, and skipping it quietly is worse than not starting
    one. This does NOT widen to agents outside the roster below; those still need an ask.
  - One scenario at a time. Never multiple architects or developers in parallel.
  - Never batch multiple scenarios in one architect or developer call.
  - Never skip `/run-reviewers` after all scenarios implemented.
  - **Inherited context is `STATE.md`, not the pile of handoffs.** `developer` rewrites
    `docs/specifications/<feature-slug>/STATE.md` at the end of each scenario; `architect`
    and `developer` read that one file. Reading every prior `## Handoff` makes context grow
    with the square of the scenario count and dominated every late agent's budget on the
    first 20-scenario feature. Per-scenario Handoffs remain the audit trail.
  - **Do not retype the standing brief into agent prompts.** Verification commands, the
    stale-diagnostics rule, the mutation-verification protocol and the vacuous-assertion
    bar live in `.claude/rules/agent-briefs.md`. Prompts carry the scenario-specific delta
    only — what this scenario decides, what it inherits, what is deliberately deferred.
  - **Re-gate narrowly.** After a fix pass, re-run the reviewers that blocked plus any
    whose trigger globs match files the fix touched — not the whole set. A reviewer that
    returned PASS on a surface the fix did not touch has nothing new to read.
  - **PASS WITH FOLLOW-UPS is done.** Only BLOCKER and MAJOR block. MINOR/NIT are
    fix-if-cheap — never force another round trip.
  - `/run-reviewers` reports `REVIEWER DISCOVERY FAILED` → gate did not run. Fix
    discovery; do not treat as PASS.
  - **Two gates, only two.** Pipeline pauses for user at scenario approval (before
    `specification.md` written) and at RETHINK / DON'T BUILD verdict. Everything after
    scenario approval auto-continues — do not ask permission between steps.
  - Scenario-approval gate is load-bearing precisely because pipeline now starts
    implicitly. Never write spec file from offhand request without it.

## Agent roster

| Agent                                | Stage               | Owns                                                                                |
| ------------------------------------ | ------------------- | ----------------------------------------------------------------------------------- |
| `triage`                             | scoping             | What exists, what's affected, reproduction. Read-only                               |
| `product-vision`                     | scoping + final     | Whether surface should exist, what it's called. React app _and_ generated TS client |
| `architect`                          | per scenario        | Ordered TDD checklist. Writes no code                                               |
| `developer`                          | per scenario        | TDD implementation, fix mode                                                        |
| `arch-reviewer`                      | review              | Structure: layout, dependency rule, Store + adapters, wiring                        |
| `correctness-reviewer`               | review              | Wrong behavior: context, goroutines, races, Lambda state reuse, errors              |
| `api-reviewer`                       | review              | HTTP/ogen conformance                                                               |
| `test-reviewer` / `ui-test-reviewer` | review              | Coverage, corner cases, test quality                                                |
| `refactor-advisor`                   | review (post-green) | Quality. MINOR/NIT only — never blocks                                              |

Reviewers share one severity contract: **BLOCKER** (data loss, cross-request leakage,
reachable panic, silently wrong result, untested change) · **MAJOR** (defect with
constructible failure) · **MINOR** (smell with no constructible failure) · **NIT**
(preference, never blocks). Finding with no concrete failure is downgraded to MINOR and
labeled as such.

## VERY IMPORTANT: TDD applies to every production change

Every production change (new code, bug fix, refactor) is preceded by a failing test.
Red-green-refactor methodology, naming conventions, black-box/white-box rules, handler
test conventions live in `tdd` + `go-testing` skills (enforced by `test-reviewer`) for
Go, and `ui-testing` skill (enforced by `ui-test-reviewer`) for React — invoke the
matching skill when writing or modifying tests.

## Code quality conventions

`refactor-advisor` enforces patterns from `~/.claude/refactor-catalog.md` —
_Comment as a missing name_, _Compose method_, _Feature envy → Move method_, others.

## Toolchain

Bash tool calls already run inside the pinned nix/devenv environment, so `go`, `heph`,
`golangci-lint`, `buf`, `node`, `npx` resolve to the pinned versions. Run them directly:

```bash
go test ./...
```

Always confirm `go version` matches the pin before trusting a result.

**Never write a `/nix/store/...` path into a checklist, an agent prompt, or a command.**
They go stale on every rebuild. A toolchain that looks wrong (heph `driver not found:
scratch`) means the shell was **sandboxed** — re-run with
`dangerouslyDisableSandbox: true`.

This applies to `architect` especially: its checklists tell `developer` what to run, so
a stale path in a plan propagates into every step of that scenario.

Verification: iterate with `go ...`; run `heph run //go:test-all` once before calling a
scenario done. Details in `.claude/rules/go-code.md`.
