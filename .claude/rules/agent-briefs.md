# Standing brief for pipeline agents

Everything here used to be retyped into each `architect`/`developer`/reviewer prompt, at
60–100 lines per invocation. It lives here so a prompt carries only what is specific to
that scenario. Read this once; do not ask for it to be repeated.

## Verification

`brief` is one Go module at the repo root. There is no build-graph tool, no codegen step
and no second workspace. Run, from the repo root:

```bash
go build ./...
go test ./...
go test -race ./<touched package>/...
golangci-lint run ./...
```

Rules:

- **Never pipe a verification command through `head`/`tail`.** It hides failures below the
  cut, and `$?` becomes the pipe's status — `go build ./nonexistent 2>&1 | tail -2` reports
  **exit 0** for a failed build. If you must pipe, prefix with `set -o pipefail`.
- **Report the exact test count and the delta** — "green" is not a result. A count that
  moved without explanation is a finding, not a rounding error.
- A green summary does not mean everything ran. Count skips before leaning on a package:
  `go test -v ./<pkg>/... 2>&1 | grep -c -- "--- SKIP"`.
- A Bash call failing with `operation not permitted` means the shell was **sandboxed**.
  Re-run with `dangerouslyDisableSandbox: true`.
- Before declaring a scenario done, run `go test ./...` from the repo root once, unpiped.
  The exit code of an unpiped command is the evidence.

## Scenario plan files are brief step files

`docs/specifications/<feature>/SCENARIO-XX.md` is read by `brief` itself (`brief status`,
`brief check`). The architect writes it starting with frontmatter, then the heading, with the
checklist under `## Implementation Plan`:

```markdown
---
id: SCENARIO-XX
status: open
---

# SCENARIO-XX: <title>
```

The developer sets `status: done` when the scenario is complete, alongside ticking it in
`specification.md`. Every `- [ ]` under `## Implementation Plan` must be ticked by then —
`brief check` reports an unticked item on a done step.

## IDE diagnostics are advisory

The IDE indexes mid-edit, and during mutation windows. It routinely reports compile errors
that `go build` does not, and it indexes files that were deleted. Across one 20-scenario
feature it was wrong every single time.

Do not chase them. Do not re-verify on their account. The authority is `go build`. The one
exception: a diagnostic that **contradicts a claim you just made** is worth a single
targeted check — that is how a live mutation left by a crashed run was caught.

## Mutation verification

A guard, a test, or an "absence" claim is proven by breaking the thing and seeing the
specific test go red — not by the suite being green.

**Copy the file aside so a crash cannot leave the mutation behind:**

```bash
cp <file> "$TMPDIR/<name>.orig"      # take a FRESH copy immediately before each mutation
# apply the mutation, run the targeted test, observe RED
cp "$TMPDIR/<name>.orig" <file>      # restore
diff "$TMPDIR/<name>.orig" <file>    # prove byte-identical
```

An interrupted run once died holding a gutted security guard, and the tree looked merely
"failing" rather than "deliberately broken". The copy makes that recoverable.

**Never use `git stash` for this.** Pipeline work runs in git worktrees, and every worktree
shares one stash stack with the main checkout and any other session: a bare `git stash pop`
can apply someone else's entry. Never reuse an old `$TMPDIR` copy either — a stale copy once
silently reverted a file to a previous commit's contents.

Rules:

- **Verify guards INDIVIDUALLY.** Two guards that only go red when BOTH are disabled means
  either can be deleted silently. Disable one at a time.
- A mutation that breaks compilation is **not** evidence. If every test fails, you proved
  the file parses, nothing more. Make the mutation surgical and still-valid.
- Say which mutation you ran and which test it reddened. "Mutation-verified" alone is not a
  claim anyone can check.

## Reviewing: scope and completeness

A review gate is not free. One 10-scenario feature spent roughly 550k tokens on reviewers and
another 780k on the developer passes answering them, and the largest single cause was
reviewers re-reading whole packages they had already read in an earlier round.

**Read the delta, not the tree.** Your prompt names a commit range or a file list. Start from
`git diff <range>` and read only what the diff touches. Every reviewer has `Bash` for exactly
this; a reviewer that cannot run it says so rather than quietly reading whole packages. Widen to a whole file when the diff
alone cannot settle a question — and say in the finding why you had to. A package you already
reviewed in an earlier round, on a surface this fix did not touch, has nothing new in it.

**Report every finding in the round you find it.** Do not hold a MINOR back "for the next
pass", do not open with a finding you then withdraw, and do not re-raise a finding the
previous round already recorded as deferred. A finding that arrives one round late costs a
whole extra gate: the developer pass, the re-gate, and every reviewer that re-reads the
result.

**Say what you could not check.** A path you had no way to exercise — an environment you
cannot change, a host you cannot detect — is reported as unchecked, not silently passed and
not guessed at. Unchecked is a fact the caller can act on; a guess is one they cannot.

## Assertions that prove nothing

One feature produced **fourteen** assertions that looked like proof and were not. The
recurring shapes:

- Asserting against a constant the fixture set, or a value copied out of the production
  code being tested. A pin derived by reading the code pins nothing.
- Negative assertions satisfied by nothing happening at all — the dominant shape. An
  absence claim needs a **control arm** that shows the thing DOES happen when the guard is
  removed, and the control must differ from the claim in exactly one variable.
- Observables that cannot fire on the path under test.
- Asserting a store is empty without first proving it was non-empty and that the same probe
  would have seen it.
- Comments overclaiming what the test below them covers.

When a refactor removes a call site, **every existing "was never called" assertion on that
fake becomes unfalsifiable.** Repoint them at the new reachable observable, or they pass
with the guard deleted.

## Reporting

- If a step comes out **green on arrival**, say so and say why. Do not manufacture a red.
- If you disagree with an instruction or a finding, say so **with evidence** rather than
  skipping it silently.
- If a control arm does not behave as its plan predicts, **stop and report** — do not
  proceed to green on a claim whose control proved nothing.
- Deferred items stay deferred. Do not opportunistically fix things outside the brief; list
  them instead.
- Never write a `/nix/store/...` path into a plan, prompt, or command. They go stale on
  every rebuild.
