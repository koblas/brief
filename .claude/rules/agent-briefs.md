# Standing brief for pipeline agents

All here used to get retyped into each `architect`/`developer`/reviewer prompt, 60–100 lines per invocation. Live here so prompt carry only what specific to that scenario. Read once; no ask for repeat.

## Verification

`brief` is one Go module at repo root. No build-graph tool, no codegen step, no second workspace. Run from repo root:

```bash
go build ./...
go test -count=1 -coverpkg=./... -coverprofile="$TMPDIR/cover.out" ./...   # the full suite, once
go test -race ./<touched package>/...
golangci-lint run ./...
.claude/scripts/uncovered-diff.py --profile "$TMPDIR/cover.out" <start>    # coverage gate, no re-run
.claude/scripts/test-stats.py --base <start> --changed                     # counts and deltas
```

`<start>` = commit your scenario or fix pass started from.

**Narrow loop while working, full run once.** During scenario `### Red` and `### Green` phases run only packages and tests in play — `go test ./internal/setup/ -run 'Skill|Init'`. Run block above once, in `### Verify` phase (and at end of every fix pass). Full suite after every edit = most expensive habit, proves nothing final run does not. That one `go test` line is both full suite and coverage data — do not run suite second time for gate.

**Coverage gate before handing off.** Every production line you added must be executed by test. `uncovered-diff.py` lists each added non-test line no test executes, grouped into runs with enclosing function, exits 1 if any left. Reach zero, or mark genuinely unreachable defensive branch in code with `// unreachable: <reason>` on the line (or the line above it) — then it move to "declared unreachable" section reviewer judge, and stop failing gate every later pass. On one feature, untested branches added by previous pass were bulk of test-reviewer MAJORs, cost six fix passes.

**Counts come from `test-stats.py --base <start> --changed`**: every package whose tests changed, with `now (±delta)` for top-level tests, `t.TempDir()` sites and disk-touching tests, read from git at `<start>` — never from archive or checkout you build yourself.

Rules:

- **Never pipe verification command through `head`/`tail`.** Hides failures below cut, and `$?` become pipe status — `go build ./nonexistent 2>&1 | tail -2` reports **exit 0** for failed build. If must pipe, prefix `set -o pipefail`.
- **Report exact test count and delta, from `.claude/scripts/test-stats.py`** — "green" not result, and hand-rolled counts drifted up to nine tests between agents on same commit. Quote its rows as printed. Never write own counting script. Count that moved without explanation = finding, not rounding error.
- Green summary not mean everything ran. `test-stats.py --run <pkgdir>` counts leaf pass/fail/skip in one parallel `go test -json`; check skips before leaning on package.
- Write scratch files only under `$TMPDIR` or session scratchpad — never `/tmp`, never path outside worktree you got.
- Bash call failing with `operation not permitted` mean shell was **sandboxed**. Re-run with `dangerouslyDisableSandbox: true`.
- Before declaring scenario done, run `go test ./...` from repo root once, unpiped. Exit code of unpiped command = evidence.

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

## IDE diagnostics are advisory

IDE indexes mid-edit, and during mutation windows. Routinely reports compile errors `go build` does not, and indexes deleted files. Across one 20-scenario feature it wrong every single time.

No chase them. No re-verify on their account. Authority is `go build`. One exception: diagnostic that **contradicts claim you just made** worth single targeted check — that how live mutation left by crashed run got caught.

## Mutation verification

Guard, test, or "absence" claim proven by breaking thing and seeing specific test go red — not by suite being green.

**Copy the file aside so a crash cannot leave the mutation behind:**

```bash
cp <file> "$TMPDIR/<name>.orig"      # take a FRESH copy immediately before each mutation
# apply the mutation, run the targeted test, observe RED
cp "$TMPDIR/<name>.orig" <file>      # restore
diff "$TMPDIR/<name>.orig" <file>    # prove byte-identical
```

Interrupted run once died holding gutted security guard, tree looked merely "failing" not "deliberately broken". Copy make that recoverable.

**Never use `git stash` for this.** Pipeline work runs in git worktrees, and every worktree shares one stash stack with main checkout and any other session: bare `git stash pop` can apply someone else entry. Never reuse old `$TMPDIR` copy either — stale copy once silently reverted file to previous commit contents.

Rules:

- **Mutate only guards plan names.** Architect picks which guards matter; developer add no mutation checks of own. Mutation per step = how scenario double its tool calls without proving anything named ones do not.
- **Verify guards INDIVIDUALLY.** Two guards that only go red when BOTH disabled mean either can be deleted silently. Disable one at a time.
- Mutation that breaks compilation **not** evidence. If every test fails, you proved file parses, nothing more. Make mutation surgical and still-valid.
- Say which mutation you ran and which test it reddened. "Mutation-verified" alone not claim anyone can check.
- Mutation results go in the report and STATE.md, never in a test comment (`go-testing` → *Test comments*).
- **Reviewers never mutate worktree.** Reviewers run parallel; mutation in shared tree poisons every concurrent run. Mutate `git archive <sha>` export under `$TMPDIR`. Only developer (runs alone) mutates in place.

## Reviewing: scope and completeness

Review gate not free. One 10-scenario feature spent roughly 550k tokens on reviewers, another 780k on developer passes answering them, and largest single cause was reviewers re-reading whole packages they already read in earlier round.

**Read the delta, not the tree.** Your prompt names commit range or file list. Start from `git diff <range>`, read only what diff touches. Every reviewer has `Bash` for exactly this; reviewer that cannot run it say so rather than quietly reading whole packages. Widen to whole file when diff alone cannot settle question — and say in finding why you had to. Package you already reviewed in earlier round, on surface this fix did not touch, has nothing new in it.

**Report every finding in the round you find it.** No hold MINOR back "for next pass", no open with finding you then withdraw, no re-raise finding previous round already recorded as deferred. Finding that arrives one round late costs whole extra gate: developer pass, re-gate, and every reviewer that re-reads result.

**Say what you could not check.** Path you had no way to exercise — environment you cannot change, host you cannot detect — reported as unchecked, not silently passed, not guessed at. Unchecked = fact caller can act on; guess = one they cannot.

## Assertions that prove nothing

One feature produced **fourteen** assertions that looked like proof and were not. Recurring shapes:

- Asserting against constant fixture set, or value copied out of production code being tested. Pin derived by reading code pins nothing.
- Negative assertions satisfied by nothing happening at all — dominant shape. Absence claim needs **control arm** showing thing DOES happen when guard removed, and control must differ from claim in exactly one variable.
- Observables that cannot fire on path under test.
- Asserting store empty without first proving it non-empty and same probe would have seen it.
- Comments overclaiming what test below them covers.

When refactor removes call site, **every existing "was never called" assertion on that fake become unfalsifiable.** Repoint them at new reachable observable, or they pass with guard deleted.

## Reporting

- Step comes out **green on arrival** → say so and say why. No manufacture red.
- Disagree with instruction or finding → say so **with evidence**, no silent skip.
- Control arm not behave as its plan predicts → **stop and report** — no proceed to green on claim whose control proved nothing.
- Deferred items stay deferred. No opportunistic fix outside brief; list them instead.
- Never write `/nix/store/...` path into plan, prompt, or command. They go stale every rebuild.