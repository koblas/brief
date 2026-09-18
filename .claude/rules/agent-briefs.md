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

**Stash the mutation so a crash cannot leave it behind:**

```bash
git stash push -m "mutation: <what>" -- <file>   # or cp to $TMPDIR
# run the targeted test, observe RED
git stash pop                                     # or restore the copy
diff <original> <file>                            # prove byte-identical
```

An interrupted run once died holding a gutted security guard, and the tree looked merely
"failing" rather than "deliberately broken". Stashing makes that recoverable.

Rules:

- **Verify guards INDIVIDUALLY.** Two guards that only go red when BOTH are disabled means
  either can be deleted silently. Disable one at a time.
- A mutation that breaks compilation is **not** evidence. If every test fails, you proved
  the file parses, nothing more. Make the mutation surgical and still-valid.
- Say which mutation you ran and which test it reddened. "Mutation-verified" alone is not a
  claim anyone can check.

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
