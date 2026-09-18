---
paths:
  - "**"
---

# Standing brief for pipeline agents

Everything here used to be retyped into each `architect`/`developer`/reviewer prompt, at
60–100 lines per invocation. It lives here so a prompt carries only what is specific to
that scenario. Read this once; do not ask for it to be repeated.

## Verification

**Go — from `**/\*`:** `go build ./...`, `go test ./...`, `go test -race`on touched
packages,`golangci-lint run` on touched packages.

**Gates:** `heph run //frontend:test`, `//frontend:lint`, `//go:test-all`.

Rules:

- **Never pipe a verification command through `head`/`tail`.** It hides failures below the
  cut. Compress vitest with
  `grep -E "RUN +v|Tests +[0-9]|Test Files +[0-9]|failed|FAIL"` instead.
- **Report the exact test count and the delta**, per workspace — "green" is not a result.
  A count that moved without explanation is a finding, not a rounding error.
- `fxpubsub/jetstream`'s container-readiness flake is **fixed**. Do not wave a failure
  there through as "the known flake" — it is either real or a new race.
- A Bash call failing with `operation not permitted`, or heph reporting
  `driver not found: scratch`, means the shell was **sandboxed**. Re-run with
  `dangerouslyDisableSandbox: true`.
- Proto changes regenerate with **`heph run codegen //...` from the repo root** — never
  `codegen //go`, which leaves `papi/openapi_*.yaml` and the TS client stale while the
  build stays green. Then **grep both artifacts** to confirm the change landed.

## IDE diagnostics are advisory

The IDE indexes mid-edit, and during mutation windows. It routinely reports compile errors
that `tsc`/`go build` do not, and it indexes files that were deleted. Across one 20-scenario
feature it was wrong every single time.

Do not chase them. Do not re-verify on their account. The authority is `go build` / `npx
tsc`. The one exception: a diagnostic that **contradicts a claim you just made** is worth a
single targeted check — that is how a live mutation left by a crashed run was caught.

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

When a refactor removes a call site, **every existing `.not.toHaveBeenCalled()` on that
symbol becomes unfalsifiable.** Repoint them at the new reachable observable, or they pass
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
