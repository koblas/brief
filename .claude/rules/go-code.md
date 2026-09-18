---
paths:
  - "**/*.go"
---

- All Go code should be idomatic with good use of interfaces to support testing
- Comments are written for `go doc`: a `doc.go` per package, function docs that state the
  contract (cite the standard — `RFC 6749 §5.1`, `OIDC Core §3.1.2.1` — where the behaviour
  is one), no history or change narrative anywhere, and no in-function comments restating
  what the code already says. Full rules in `.claude/skills/clean-architecture/SKILL.md`
  (_Documentation & comments_). Check with `go doc ./services/<group>/<name>`.
- the code in `go/gen` is all generated, use `go generate ./gen` or `heph run codegen .` (in the go directory) to update
- testing should always use `testify` it's optional to use the `suite` testing
- The code uses go 1.26 (or bettter) so testing can assume `synctest` is available for testing and any other feature of the language

## Toolchain

Bash tool calls already run inside the pinned nix/devenv environment (`IN_NIX_SHELL` is
set), so `go`, `heph`, `golangci-lint`, `buf`, `node` and `npx` are the pinned versions.
Run them directly:

```bash
go build ./...
```

**Confirm what you got before trusting a result**: `go version` must report the version
pinned in `devenv.nix`. A mismatch means the command ran outside the environment — not
that PATH is broken.

**Never hardcode a `/nix/store/...` path.** They go stale on every rebuild — a pinned
heph path silently pointed at a garbage-collectable store entry for a whole session after
a version bump.

If the toolchain still looks wrong (e.g. heph's `driver not found: scratch`), the shell
was **sandboxed**. Re-run with `dangerouslyDisableSandbox: true` and check
`echo $DEVENV_ROOT` plus `which -a go` before concluding anything about PATH.

## Verification

Iterate with direct commands, from `go/`:

```bash
go build ./...
go test ./...
golangci-lint run ./...
```

**Never `| tail -N` / `| head -N` a verification command.** `$?` becomes the pipe's
status — `go build ./nonexistent 2>&1 | tail -2` reports **exit 0** for a failed build.
Never quote an exit code captured that way as proof anything passed. If you must pipe,
prefix with `set -o pipefail`.

A green `go test ./...` line is the evidence; the exit code of an unpiped command is
trustworthy (1 on failure, 0 on success).

**A green summary does not mean everything ran.** Suites needing docker (dynamodb-local
and the other testcontainers packages) **skip green** when no daemon is reachable. When a
claim depends on one of them, count the skips:

```bash
go test -v ./services/core/identity/... 2>&1 | grep -c -- "--- SKIP"
```

Before declaring a scenario or task complete, run the build graph once:

```bash
go test ./...
```

Direct `go test` bypasses the heph graph, its caching and its codegen targets, so a
change can leave `go test ./...` green while the graph is broken.

`fxpubsub/jetstream`'s `TestJestSuite/TestE2E` used to fail intermittently with
`connect: connect failed: EOF`. **That is fixed — do not dismiss it as a known flake.**
The nats module waits only on `wait.ForListeningPort`, so docker answered the port before
`nats-server` would complete a handshake; `SetupSuite` now probes with a real client
before the suite runs. A recurrence is a real failure, or a new race, and wants
investigating rather than a rerun.
