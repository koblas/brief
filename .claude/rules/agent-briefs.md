# Standing brief for pipeline agents

Standing brief every `architect`/`developer`/reviewer prompt would otherwise retype. Live here so prompt carry only what specific to that scenario. Read once; no ask for repeat.

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

**Coverage gate before handing off.** Every production line you added must be executed by test. `uncovered-diff.py` lists each added non-test line no test executes, grouped into runs with enclosing function, exits 1 if any left. Reach zero, or mark genuinely unreachable defensive branch in code with `// unreachable: <reason>` on the line (or the line above it) — then it move to "declared unreachable" section reviewer judge, and stop failing gate every later pass. Untested branch added by fix pass becomes next round's test-reviewer MAJOR.

**Counts come from `test-stats.py --base <start> --changed`**: every package whose tests changed, with `now (±delta)` for top-level tests, `t.TempDir()` sites and disk-touching tests, read from git at `<start>` — never from archive or checkout you build yourself.

Rules:

- **Never pipe verification command through `head`/`tail`.** Hides failures below cut, and `$?` become pipe status — `go build ./nonexistent 2>&1 | tail -2` reports **exit 0** for failed build. If must pipe, prefix `set -o pipefail`.
- **Report exact test count and delta, from `.claude/scripts/test-stats.py`** — "green" not result, and hand-rolled counts drift between agents on same commit. Quote its rows as printed. Never write own counting script. Count that moved without explanation = finding, not rounding error.
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

IDE indexes mid-edit, and during mutation windows. Routinely reports compile errors `go build` does not, and indexes deleted files.

No chase them. No re-verify on their account. Authority is `go build`. One exception: diagnostic that **contradicts claim you just made** worth single targeted check — it can be live mutation left behind by crashed run.

## Mutation verification

Guard, test, or "absence" claim proven by breaking thing and seeing specific test go red — not by suite being green.

**Copy the file aside so a crash cannot leave the mutation behind:**

```bash
B="$TMPDIR/mutation-$(basename <file>).$$"   # unique per run; take FRESH copy before each mutation
cp <file> "$B" && test -f "$B" || exit 1     # a shared name may be a DIRECTORY
# apply the mutation, run the targeted test, observe RED
cp "$B" <file>                               # restore
diff "$B" <file>                             # prove byte-identical
```

Interrupted run can die holding gutted guard, and tree then looks merely "failing" not "deliberately broken". Copy make that recoverable.

**Unique backup name, checked to be file.** Fixed path like `$TMPDIR/mutation-backup` shared by every agent in session: if earlier one left *directory* there, `cp <file> "$TMPDIR/mutation-backup"` silently copies INTO it, restore then fails with mutation still live. Only mandated `diff` reveals it.

**Never use `git stash` for this.** Pipeline work runs in git worktrees, and every worktree shares one stash stack with main checkout and any other session: bare `git stash pop` can apply someone else entry. Never reuse old `$TMPDIR` copy either — stale copy silently reverts file to older contents.

Rules:

- **Mutate only guards plan names.** Architect picks which guards matter; developer add no mutation checks of own. Mutation per step = how scenario double its tool calls without proving anything named ones do not.
- **Verify guards INDIVIDUALLY.** Two guards that only go red when BOTH disabled mean either can be deleted silently. Disable one at a time.
- Mutation that breaks compilation **not** evidence. If every test fails, you proved file parses, nothing more. Make mutation surgical and still-valid.
- Say which mutation you ran and which test it reddened. "Mutation-verified" alone not claim anyone can check.
- Mutation results go in the report and STATE.md, never in a test comment (`go-testing` → *Test comments*).
- **Reviewers never mutate worktree.** Reviewers run parallel; mutation in shared tree poisons every concurrent run. Mutate `git archive <sha>` export under `$TMPDIR`. Only developer (runs alone) mutates in place.
- **Run affected package with `-run`, not whole suite.** Mutation targets one file; full-suite run per check = most repeated waste in long scenario.
- **Two reddened tests not two behaviours.** Pair sharing Given, When and Then is one case named twice; mutation report counting both overstates coverage. Check each cited test discriminates something others do not.

## Reviewing: scope and completeness

Review gate not free, and largest avoidable cost is reviewers re-reading whole packages they already read in earlier round.

**Read the delta, not the tree.** Your prompt names commit range or file list. Start from `git diff <range>`, read only what diff touches. Every reviewer has `Bash` for exactly this; reviewer that cannot run it say so rather than quietly reading whole packages. Widen to whole file when diff alone cannot settle question — and say in finding why you had to. Package you already reviewed in earlier round, on surface this fix did not touch, has nothing new in it.

**Report every finding in the round you find it.** No hold MINOR back "for next pass", no open with finding you then withdraw, no re-raise finding previous round already recorded as deferred. Finding that arrives one round late costs whole extra gate: developer pass, re-gate, and every reviewer that re-reads result.

**Say what you could not check.** Path you had no way to exercise — environment you cannot change, host you cannot detect — reported as unchecked, not silently passed, not guessed at. Unchecked = fact caller can act on; guess = one they cannot.

## Planning: coverage the gate will demand

Commonest blocking findings share one shape: fallible call in new code with no fault test, or numeric bound with no outside-the-bound test. Each costs fix pass plus re-gate for test architect could have listed up front. Every architect checklist for new or changed command, feature-package method or adapter carries, as named steps:

- **One fault test per fallible call** — each `Store` call, file read/write/rename, `exec`, and parse the code makes. Include call that re-reads on resume or retry branch, not just first one.
- **Every numeric bound tested just outside it**, in-bound case as control (line caps, count limits, depth limits).
- **Every fallback branch of error → exit-code mapper** — the `default:` arm, not just named sentinels.
- **One decode-fault test per decoded record kind** for adapter or parser reading files — frontmatter as well as body items. Corrupt-child-item test on a read does not cover corrupt root on same read.

## Fix passes: no new behaviour

Fix pass applies findings. Cheap MINOR/NIT folds = docs, renames, test additions, extractions keeping behaviour identical. **Fold adding runtime behaviour not cheap:** new branch folded in from MINOR and left untested becomes MAJOR forcing another fix pass and re-gate. New behaviour goes to STATE.md `## Open debts`, or folded with its tests planned in same brief, named per branch.

**Applies to MAJOR's fix too, not only folds.** When fix for finding adds runtime behaviour (timeout, retry, fallback, new branch), fix-pass brief names, before dispatch:

- **positive assertion** for each new branch — what DID happen, not only what did not — with control arm differing in one variable;
- for every new numeric bound, **in-bound and just-outside-bound tests** ("Planning" above applies to fix passes unchanged);
- **mutation expected to redden each test**.

Test reading `ctx.Err()` from value defaulting to nil, or bound test that cannot see bound's value, passes whether or not new branch works — each such gap found at gate costs whole extra fix pass and re-gate.

## Assertions that prove nothing

Assertions that look like proof and are not recur in few shapes:

- Asserting against constant fixture set, or value copied out of production code being tested. Pin derived by reading code pins nothing.
- Negative assertions satisfied by nothing happening at all — dominant shape. Absence claim needs **control arm** showing thing DOES happen when guard removed, and control must differ from claim in exactly one variable.
- Observables that cannot fire on path under test.
- Asserting store empty without first proving it non-empty and same probe would have seen it.
- Comments overclaiming what test below them covers.

When refactor removes call site, **every existing "was never called" assertion on that fake become unfalsifiable.** Repoint them at new reachable observable, or they pass with guard deleted.

## Verification greps

Grep is evidence only if it can see what it looks for.

- **Comment sweeps must be multiline-aware.** `//` blocks wrap, so phrase splits across lines and line-based `grep` silently reports zero. Flatten continuations first (strip leading `//`) before matching. "0 hits" from line-based grep over prose = untested claim, not clean sweep.
- **Run positive control before believing zero.** Grep for symbol you KNOW is present with same flags and scope. Control not found → sweep cannot see target, its zero means nothing.
- State what grep would MISS, not just what it found.

## Reporting

- Step comes out **green on arrival** → say so and say why. No manufacture red.
- Disagree with instruction or finding → say so **with evidence**, no silent skip.
- Control arm not behave as its plan predicts → **stop and report** — no proceed to green on claim whose control proved nothing.
- Deferred items stay deferred. No opportunistic fix outside brief; list them instead.
- Never write `/nix/store/...` path into plan, prompt, or command. They go stale every rebuild.