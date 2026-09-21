# SCENARIO-08: "brief help <command>" matches "<command> --help"

## Scenario

Scenario: SCENARIO-08 "brief help <command>" matches "<command> --help"
  When I run "brief help start" and "brief help new feature"
  Then each stdout is byte-identical to "brief start --help" and "brief new feature --help", with exit 0

## User-visible contract

- `brief help <leaf path…>` for every leaf (`new feature`, `new step`, `start`, `status`,
  `check`, `finish`) → stdout byte-identical to `brief <leaf path…> --help`, stderr empty,
  exit 0.
- `brief help` (no topic) → root help (`rootHelp`), unchanged.
- `brief help bogus` → still root help, exit 0 (today's behavior). Unchanged here; S09 owns
  its usage-error copy and exit 2.
- `brief help new` → changes from root help to `new`'s own (currently bare) help render — see
  Traps. Not pinned; S10 owns `new`'s help.
- `brief help start extra` / `brief help start --json` → not pinned; deferred to S09 (see
  Handoff).

## Existing facts this plan relies on

- Probed (temporary white-box test, deleted): `root.Find` never errors on this tree
  (`ArbitraryArgs` everywhere). `Find(["start"])` → `brief start`; `Find(["new","feature"])`
  → `brief new feature`; `Find(["bogus"])` → `brief`, residual `["bogus"]`;
  `Find(["start","extra"])` → `brief start`, residual `["extra"]`; `Find(["start","--json"])`
  → `brief start`, residual `["--json"]`; `Find(["new"])` → `brief new`; `Find(["help"])` →
  the stub.
- Bare `brief help` is served by the hidden stub (cobra's `Find` matches the `help` child
  before `runRoot` runs), not by `runRoot`'s `case "help"` — that branch is unreachable from
  argv today. The existing `rootHelp` table row proves the output, not the path. Do not delete
  the branch in this scenario.
- `help_test.go` already has `const startHelp` (literal `brief start --help` stdout) — reuse it
  as the golden anchor; do not write a new golden.

## Implementation Plan

- [x] Step 1: `internal/cli/help_test.go` `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag` — table over all six leaf paths; each row runs `cli.Run` twice (`help <path…>` and `<path…> --help`), asserts both exit nil, both stderr empty, the `--help` capture is non-empty (guards the both-sides-empty pass), and the two stdouts are equal (red)
- [x] Step 2: `internal/cli/help_test.go` `Test_help_start_prints_the_literal_start_help` — `brief help start` stdout equals the existing `startHelp` constant literally, stderr empty, no error: the anchor a mutation breaking both sides identically cannot pass (red)
- [x] Step 3: `internal/cli/cli.go` hidden `help` stub in `newRootCommand` — resolve the topic with `cmd.Root().Find(args)` and call `target.Help()`; keep `Hidden`, `DisableFlagParsing: true`, `ArbitraryArgs`; no `cobra.CheckErr`, no `os.Exit`; update the stub's comment to state the resolution rule (and that an unresolved topic falls back to root help) without history (green)
- [x] Step 4: `internal/cli/help_test.go` `Test_prints_the_root_help_with_one_line_per_command` — confirm the existing `help` row (bare `brief help` → `rootHelp`) still passes unchanged; no edit expected (green on arrival — say so)
- [x] Step 5: mutation-verify, backing up `internal/cli/cli.go` to `$TMPDIR` and restoring by `cp` (never bare `git stash`/`pop`), one at a time: (a) stub back to `cmd.Root().Help()` → Step 1 (all six rows) and Step 2 red, confirmed byte-identical restore; (b) stub renders `target.Parent()` instead of `target` → **all six** Step 1 rows red plus Step 2 (broader than predicted: every leaf's `Parent()` differs from itself, not just `new feature`/`new step`), confirmed byte-identical restore; (c) `helpTemplate`'s `HasParent` branch rendered empty → Step 1's non-empty guard red on every row and Step 2 red, confirmed byte-identical restore (verify)
- [x] Step 6: `go build ./...` OK; `go test ./...` all 8 packages ok/no-test-files, unpiped exit 0; `internal/cli` 203 passing subtests (`--- PASS` count), 0 skips — this scenario added the 6-row table (`Test_help_topic_prints_the_same_bytes_as_the_command_help_flag`, 1 top-level + 6 subtest PASS lines) plus 1 top-level test (`Test_help_start_prints_the_literal_start_help`), 8 new PASS lines total; `go test -race ./internal/cli/...` ok; `golangci-lint run ./...` 0 issues (verify)
- [x] Step 7: all tests green → mark SCENARIO-08 done in `docs/specifications/cli-cobra/specification.md`; fold the Handoff into `STATE.md` (correct STATE.md's claim that root's own `help` branch serves bare `brief help` — the stub does) (update)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `help <topic…>` resolves via `cmd.Root().Find(args)` then `target.Help()` — the only way to
  keep R8's byte-identity structural (same template, same `Help()` call as `--help`).
- The stub keeps `DisableFlagParsing: true` — without it `help start --json` never reaches the
  stub and becomes an R14 flag error instead.
- The stub never calls `cobra.CheckErr`/`os.Exit`; `Find` cannot fail on this tree, so an
  unresolved topic is signalled only by `target == root` plus residual args.
- R8 byte-identity is pinned for all six leaves by comparing against the live `--help`
  capture, plus one literal anchor (`startHelp`). Changing a leaf's help moves both sides;
  changing start's help also moves `startHelp`.

**Left unbuilt** — named so nobody assumes it exists:
- `help bogus` usage error (`brief help: unknown command "bogus"; expected one of: …`, exit 2)
  — S09. Today it renders root help, exit 0.
- `help <cmd> <extra>` and `help <cmd> --flag` (e.g. `help start extra`, `help start --json`)
  — deliberately unpinned; both currently render start's help. S09's unknown-topic rule must
  key off post-`Find` residual args, and pinning these now would fix that rule's shape before
  S09 chooses it. S09 decides and pins them.
- `help new bogus` → resolves to `new` with residual `["bogus"]` — S09/S10.
- `help help` → renders the hidden stub's own help — unowned, unpinned.
- `new`'s own help (`new --help`, `help new`) — S10.

**Traps** — things that look right and are not:
- `brief help new` now emits `new`'s bare render — exactly
  `"Usage:\n  brief new\n\n\n\nFlags:\n"` (no `Short`/`Long`, empty flag table because `new`
  is `DisableFlagParsing`, so cobra never adds `-h`). S08 causes this; S10 repairs it. It is
  not an S10 regression.
- `runRoot`'s `case "help"` is unreachable from argv — `Find(["help"])` returns the stub. Its
  `-h`/`--help` cases are live (root is `DisableFlagParsing`).
- An equality-only table passes when both sides render empty; the non-empty guard and the
  `startHelp` anchor are what make the table falsifiable.
- `Find(["--json","start"])` returns root (residual `["--json","start"]`) — a leading flag
  before the topic does not resolve the topic. Unpinned.
