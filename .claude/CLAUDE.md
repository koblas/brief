# Clean Architecture & TDD Playbook

`brief` is **single Go binary**. One module at repo root, no frontend, no protos, no generated clients, no separate services. Anything in tree implying otherwise is bug — fix it, not work around.

## Workflow rules

- **Step 0 — fresh worktree (MANDATORY, before any feature work):** every pipeline run starts from clean, current default branch in isolated worktree — never shared checkout's current branch.
  1. Already in worktree → skip.
  2. Else call **`EnterWorktree`**, `name` = short feature slug. `worktree.baseRef` defaults to `fresh` — branches off `origin/<default-branch>` after fetch, so worktree starts from current master on own branch regardless of main checkout's branch. All pipeline artifacts live inside it.
  3. **No dependency warming step.** Go module cache global, shared across worktrees. Do not wait on one, do not look for status file.

- **Pipeline is default path, not command user must remember.** User asks for behavior that not exist, or change to behavior → enter pipeline **without being asked**, Step 0 then scoping below. `/intent-and-goal` still typable to force it, but it procedure, not trigger.

  **Triggers pipeline:**
  - New user-visible behavior — command, subcommand, flag, output format, exit-code contract.
  - New public API surface — exported package API, config key, on-disk or wire format.
  - Change to existing contract or existing user-visible behavior.

  **Does NOT trigger pipeline** — do work:
  - Questions about how existing behavior works.
  - Named bug, known fix, no contract change.
  - Refactors, renames, cleanups with no behavior change.
  - Config, deps, CI, tooling, docs, `.claude/` itself.
  - Already covered by spec under `docs/specifications/` — resume that spec at first unchecked scenario instead of new one.

  Doubt → run **`triage`** first, decide from what it finds. Triage read-only and cheap; spec file nobody asked for is not.

  **Announce, don't block.** On implicit start say in one line what you treat request as and that triage runs — then proceed. Cheap redirect, no blocking question.

- **Scenarios first**: triage, refine intent, get product verdict, propose Gherkin scenarios, write Source of Truth (SoT) specification file before any code. `commands/intent-and-goal.md` holds detailed procedure.

- **Sequential pipeline.**

  **Scoping (the `intent-and-goal` procedure):**
  1. **`triage`** — read-only. What exists, what affected, what unknown.
  2. **`product-vision`** — right thing, right shape, named right? Verdict: SHIP / SHIP WITH CHANGES / RETHINK / DON'T BUILD. RETHINK and DON'T BUILD stop pipeline, go back to user.
  3. Scenarios approved → `specification.md` written, carrying triage brief + product verdict.

  **For each scenario in order (top-to-bottom in `## BDD Acceptance Progress`):** 4. Run **`architect`** to plan it (produces `SCENARIO-XX.md`). 5. Run **`developer`** to implement it. 6. Next unchecked scenario.

  **After all scenarios implemented:** 7. Run **`/run-reviewers`** (once, no arguments) on all changed files. 8. **FAIL** (any BLOCKER or MAJOR) → run **`developer`** in fix mode with consolidated findings, one pass. 9. Run **`/run-reviewers`** again. Repeat until PASS or PASS WITH FOLLOW-UPS. 10. Run **`product-vision`** once more on finished surface — command names, flags, help text, output, error copy, exit codes. Reviews design that shipped, not design proposed. Verdict has consequences: - **SHIP** — push branch and open PR against `main` (title from spec's Primary Goal; body: summary, final gate verdicts, STATE.md follow-ups, test plan, anything left unverified). Push and PR are only outward-facing steps; ask before running them. Never merge. - **SHIP WITH CHANGES** — hand changes to **`developer`** in fix mode as MAJOR findings, then re-run `/run-reviewers` (step 7). - **RETHINK** / **DON'T BUILD** — stop, put to user. Surface already shipped, so this conversation, not silent revert.

  **Rules:**
  - **Every agent in this pipeline counts as user-requested.** Harness rule may say no subagents unless user asks. Entering pipeline IS that ask: `triage`, `product-vision`, `architect`, `developer`, and reviewers `/run-reviewers` drives are procedure, not optional delegation. Spawn without asking permission. Do not silently run pipeline "inline" instead — pipeline whose reviewer gate never ran is not pipeline, and skipping it quietly worse than not starting one. Does NOT widen to agents outside roster below; those still need ask.
  - One scenario at a time. Never multiple architects or developers in parallel.
  - Never batch multiple scenarios in one architect or developer call.
  - Never skip `/run-reviewers` after all scenarios implemented.
  - **Inherited context is `STATE.md`, not pile of handoffs.** `developer` rewrites `docs/specifications/<feature-slug>/STATE.md` at end of each scenario; `architect` and `developer` read that one file. Reading every prior `## Handoff` makes context grow with square of scenario count and dominates every late agent's budget on long feature. Per-scenario Handoffs remain audit trail.
  - **Do not retype standing brief into agent prompts.** Verification commands, stale-diagnostics rule, mutation-verification protocol, vacuous-assertion bar live in `.claude/rules/agent-briefs.md`. Prompts carry scenario-specific delta only — what this scenario decides, what it inherits, what deliberately deferred.
  - **Re-gate narrowly.** After fix pass, re-run reviewers that blocked plus any whose trigger globs match files fix touched — not whole set. Reviewer that returned PASS on surface fix did not touch has nothing new to read.
  - **PASS WITH FOLLOW-UPS is done.** Only BLOCKER and MAJOR block. MINOR/NIT fix-if-cheap — never force another round trip.
  - **Fix passes capped at 3.** 4th round opens only for BLOCKER, or for MAJOR previous fix pass itself introduced *and* that changes exit code or written file. Every other finding after pass 3 goes to STATE.md `## Open debts` and feature proceeds to final product-vision pass. Two passes in row each reopening same surface is design smell, not to-do list: stop, consolidate that logic behind one decision point before patching again.
  - **Gate round is expensive unit.** Wait for every reviewer before dispatching fix pass, hand developer one consolidated list — blocking findings plus cheap MINOR/NIT folds. Fix pass sent moment first reviewer reports guarantees second round for findings already in flight.
  - **Scope every reviewer prompt to diff.** Pass commit range and matched file list, and on re-gate say which of that reviewer's own findings being re-checked and which STATE.md already records as deferred. `/run-reviewers` does this; hand-spawned reviewer must too.
  - **Copy ruled at scoping, not at final gate.** Command and flag names, help strings, success/refusal/fix lines, exit codes and `--json` field names come from `product-vision`'s Phase 1 pass and live in specification's `## Surface & Copy`. Developer implements them verbatim. String invented at keyboard is string nobody ruled on, and final pass sends it back at ten times cost.
  - `/run-reviewers` reports `REVIEWER DISCOVERY FAILED` → gate did not run. Fix discovery; do not treat as PASS.
  - **Two gates, plus push.** Pipeline pauses for user at scenario approval (before `specification.md` written) and at RETHINK / DON'T BUILD verdict. Everything after scenario approval auto-continues — do not ask permission between steps — up to SHIP push, which outward-facing and asks first.
  - Scenario-approval gate load-bearing precisely because pipeline now starts implicitly. Never write spec file from offhand request without it.

## Delegating work to agents (pipeline or not)

- **Size each agent task to finish without context compaction.** One agent per package is default, but split package past ~100 top-level tests (or one expected to run past ~90 minutes) by command or file group. Agent that compacts loses track of own test counts and drops commit trailers.
- **Brief whole target, name what may stay behind.** Brief letting agent "descope for budget" turns one pass into several. Say which tests must move and which stay (and why) up front; ask for green, committed checkpoint only as fallback.
- **Parallel agents only through Agent tool's `isolation: "worktree"`.** Never create worktree yourself and hand path to agent: session can only write to own worktree, and agent told to work elsewhere is blocked by harness hook. Independent packages (no shared seam) can run parallel this way; merge after.
- **Model per call.** `architect` defaults to Opus; for scenario whose plan small (one package, roughly ≤15 steps) pass `model: "sonnet"` on Agent call. Keep Opus for multi-package or design-heavy scenarios.
- **Measure with repo's scripts, not ad hoc.** Counts come from `.claude/scripts/test-stats.py`; untested additions from `.claude/scripts/uncovered-diff.py` (see `.claude/rules/agent-briefs.md`).

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

`api-reviewer` and `api-conventions` skill kept HTTP-generic against day `brief` serves HTTP. Triggers do not match CLI-only diff, so `/run-reviewers` skips them until HTTP surface exists.

Reviewers share one severity contract: **BLOCKER** (data loss, reachable panic, silently wrong result, untested change) · **MAJOR** (defect with constructible failure) · **MINOR** (smell with no constructible failure) · **NIT** (preference, never blocks). Finding with no concrete failure downgraded to MINOR and labeled such.

## VERY IMPORTANT: TDD applies to every production change

Every production change (new code, bug fix, refactor) preceded by failing test. Red-green-refactor methodology, naming conventions, black-box/white-box rules, test conventions live in `tdd` + `go-testing` skills (enforced by `test-reviewer`) — invoke matching skill when writing or modifying tests.

## Code quality conventions

`refactor-advisor` enforces patterns from `.claude/refactor-catalog.md` — _Comment as a missing name_, _Compose method_, _Feature envy → Move method_, others.

## Toolchain

Bash tool calls already run inside pinned nix/devenv environment, so `go` and `golangci-lint` resolve to pinned versions. Run directly:

```bash
go test ./...
```

Always confirm `go version` matches pin in `devenv.nix` before trusting result.

**Never write `/nix/store/...` path into checklist, agent prompt, or command.** Go stale on every rebuild. Bash call failing with `operation not permitted` means shell was **sandboxed** — re-run with `dangerouslyDisableSandbox: true`.

Applies to `architect` especially: its checklists tell `developer` what to run, so stale path in plan propagates into every step of that scenario.

Verification: `go build ./...`, `go test ./...`, `go test -race` on touched packages, `golangci-lint run ./...`. Details in `.claude/rules/go-code.md`.