# Standing brief for pipeline agents

Standing brief every `architect`/`developer`/reviewer prompt would otherwise retype. This file auto-loads; detail lives in `.claude/briefs/` and each agent reads only its row. Prompts cite these files, never retype them.

| Who                         | Reads from `.claude/briefs/`                                    |
| --------------------------- | --------------------------------------------------------------- |
| `architect`                 | `planning.md`, `evidence.md`                                    |
| `developer` (scenario)      | `planning.md`, `verification.md`, `mutation.md`, `evidence.md`  |
| `developer` (fix mode)      | above plus `fix-pass.md`                                        |
| every reviewer              | `reviewing.md`, `evidence.md`                                   |
| `test-reviewer`             | above plus `mutation.md`                                        |
| orchestrator, fix-pass brief | `fix-pass.md`, `planning.md` → *Coverage*                      |

## TDD is not optional anywhere in pipeline

Phases, FOLDs, coverage gate and mutation checks change *how often* tests run and *how* claims get proved — never whether failing test precedes production change. Scenario: Red phase fails at assertion before Green edit. Fix pass: behaviour-changing finding gets red test before fix (`fix-pass.md`). Coverage and mutation are second proofs on top, not substitutes.

## Everyone

- Write scratch files only under `$TMPDIR` or session scratchpad — never `/tmp`, never path outside worktree you got.
- Bash call failing with `operation not permitted` mean shell was **sandboxed**. Re-run with `dangerouslyDisableSandbox: true`.
- Never write `/nix/store/...` path into plan, prompt, or command. They go stale every rebuild.

## IDE diagnostics are advisory

IDE indexes mid-edit, and during mutation windows. Routinely reports compile errors `go build` does not, and indexes deleted files.

No chase them. No re-verify on their account. Authority is `go build`. One exception: diagnostic that **contradicts claim you just made** worth single targeted check — it can be live mutation left behind by crashed run.

## Reporting

- Step comes out **green on arrival** → say so and say why. No manufacture red.
- Disagree with instruction or finding → say so **with evidence**, no silent skip.
- Control arm not behave as its plan predicts → **stop and report** — no proceed to green on claim whose control proved nothing.
- Deferred items stay deferred. No opportunistic fix outside brief; list them instead.
