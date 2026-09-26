# Verification

For `developer` (every scenario and fix pass). `architect` cites it for the Verify step.

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

Coverage gate proves line *ran*, not that test came first. It backs TDD, never replaces it: test written after code and merely executing it is not red-first test (`tdd` skill).

**Counts come from `test-stats.py --base <start> --changed`**: every package whose tests changed, with `now (±delta)` for top-level tests, `t.TempDir()` sites and disk-touching tests, read from git at `<start>` — never from archive or checkout you build yourself.

Rules:

- **Never pipe verification command through `head`/`tail`.** Hides failures below cut, and `$?` become pipe status — `go build ./nonexistent 2>&1 | tail -2` reports **exit 0** for failed build. If must pipe, prefix `set -o pipefail`.
- **Report exact test count and delta, from `.claude/scripts/test-stats.py`** — "green" not result, and hand-rolled counts drift between agents on same commit. Quote its rows as printed. Never write own counting script. Count that moved without explanation = finding, not rounding error.
- Green summary not mean everything ran. `test-stats.py --run <pkgdir>` counts leaf pass/fail/skip in one parallel `go test -json`; check skips before leaning on package.
- Before declaring scenario done, run `go test ./...` from repo root once, unpiped. Exit code of unpiped command = evidence.
