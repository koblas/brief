# Clean Architecture & TDD Playbook

`brief` is a **single Go binary**. One module at the repo root, no frontend, no protos, no
generated clients, no separate services. Anything in this tree that implies otherwise is a
bug — fix it rather than working around it.

## Workflow rules

- **Step 0 — fresh worktree (MANDATORY, before any feature work):** every pipeline run
  starts from a clean, current default branch in an isolated worktree — never the shared
  checkout's current branch.
  1. Already in a worktree → skip.
  2. Else call **`EnterWorktree`**, `name` = short feature slug. `worktree.baseRef`
     defaults to `fresh`, which branches off `origin/<default-branch>` after a fetch, so
     the worktree starts from current master on its own branch regardless of the main
     checkout's branch. All pipeline artifacts live inside it.
  3. **No dependency warming step.** Go's module cache is global and shared across
     worktrees. Do not wait on one, do not look for a status file.

- **Pipeline is default path, not command user must remember.** User asks for behavior
  that does not exist, or change to behavior that does → enter pipeline **without being
  asked**, starting Step 0 then scoping below. `/intent-and-goal` still typable to force
  it, but it is the procedure, not the trigger.

  **Triggers pipeline:**
  - New user-visible behavior — command, subcommand, flag, output format, exit-code
    contract.
  - New public API surface — exported package API, config key, on-disk or wire format.
  - Change to an existing contract or existing user-visible behavior.

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
  findings, one pass. 9. Run **`/run-reviewers`** again. Repeat until PASS or PASS WITH FOLLOW-UPS. 10. Run **`product-vision`** once more on finished surface — command names, flags, help
  text, output, error copy, exit codes. It reviews design that shipped, not design
  proposed. Verdict has consequences: - **SHIP** — done. - **SHIP WITH CHANGES** — hand changes to **`developer`** in fix mode as MAJOR
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
  - **A gate round is the expensive unit.** Wait for every reviewer before dispatching the
    fix pass, and hand the developer one consolidated list — blocking findings plus the
    cheap MINOR/NIT folds. A fix pass sent the moment the first reviewer reports guarantees
    a second round for findings that were already in flight. On one feature three rounds
    cost ~1.3M tokens; one would have cost roughly a third of that.
  - **Scope every reviewer prompt to the diff.** Pass the commit range and the matched file
    list, and on a re-gate say which of that reviewer's own findings are being re-checked
    and which STATE.md already records as deferred. `/run-reviewers` does this; a
    hand-spawned reviewer must too.
  - **Copy is ruled at scoping, not at the final gate.** Command and flag names, help
    strings, success/refusal/fix lines, exit codes and `--json` field names come from
    `product-vision`'s Phase 1 pass and live in the specification's `## Surface & Copy`.
    The developer implements them verbatim. A string invented at the keyboard is a string
    nobody ruled on, and the final pass sends it back at ten times the cost.
  - `/run-reviewers` reports `REVIEWER DISCOVERY FAILED` → gate did not run. Fix
    discovery; do not treat as PASS.
  - **Two gates, only two.** Pipeline pauses for user at scenario approval (before
    `specification.md` written) and at RETHINK / DON'T BUILD verdict. Everything after
    scenario approval auto-continues — do not ask permission between steps.
  - Scenario-approval gate is load-bearing precisely because pipeline now starts
    implicitly. Never write spec file from offhand request without it.

## Agent roster

| Agent                  | Stage               | Owns                                                                  |
| ---------------------- | ------------------- | --------------------------------------------------------------------- |
| `triage`               | scoping             | What exists, what's affected, reproduction. Read-only                 |
| `product-vision`       | scoping + final     | Whether surface should exist, what it's called. CLI surface + Go API  |
| `architect`            | per scenario        | Ordered TDD checklist. Writes no code                                 |
| `developer`            | per scenario        | TDD implementation, fix mode                                          |
| `arch-reviewer`        | review              | Structure: layout, dependency rule, port interfaces + adapters, wiring |
| `correctness-reviewer` | review              | Wrong behavior: context, goroutines, races, errors, nil handling      |
| `api-reviewer`         | review              | HTTP boundary — only if and when `brief` grows a server               |
| `test-reviewer`        | review              | Coverage, corner cases, test quality                                  |
| `refactor-advisor`     | review (post-green) | Quality. MINOR/NIT only — never blocks                                |

`api-reviewer` and the `api-conventions` skill are kept HTTP-generic against the day
`brief` serves HTTP. Their triggers do not match a CLI-only diff, so `/run-reviewers`
skips them until an HTTP surface exists.

Reviewers share one severity contract: **BLOCKER** (data loss, reachable panic, silently
wrong result, untested change) · **MAJOR** (defect with constructible failure) · **MINOR**
(smell with no constructible failure) · **NIT** (preference, never blocks). Finding with no
concrete failure is downgraded to MINOR and labeled as such.

## VERY IMPORTANT: TDD applies to every production change

Every production change (new code, bug fix, refactor) is preceded by a failing test.
Red-green-refactor methodology, naming conventions, black-box/white-box rules and test
conventions live in the `tdd` + `go-testing` skills (enforced by `test-reviewer`) — invoke
the matching skill when writing or modifying tests.

## Code quality conventions

`refactor-advisor` enforces patterns from `.claude/refactor-catalog.md` — _Comment as a
missing name_, _Compose method_, _Feature envy → Move method_, others.

## Toolchain

Bash tool calls already run inside the pinned nix/devenv environment, so `go` and
`golangci-lint` resolve to the pinned versions. Run them directly:

```bash
go test ./...
```

Always confirm `go version` matches the pin in `devenv.nix` before trusting a result.

**Never write a `/nix/store/...` path into a checklist, an agent prompt, or a command.**
They go stale on every rebuild. A Bash call failing with `operation not permitted` means
the shell was **sandboxed** — re-run with `dangerouslyDisableSandbox: true`.

This applies to `architect` especially: its checklists tell `developer` what to run, so
a stale path in a plan propagates into every step of that scenario.

Verification: `go build ./...`, `go test ./...`, `go test -race` on touched packages,
`golangci-lint run ./...`. Details in `.claude/rules/go-code.md`.
