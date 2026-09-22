---
id: SCENARIO-05
status: done
---

# SCENARIO-05: check --hook scopes a Claude Code hook call to the edited feature

## Scenario

```gherkin
Scenario: SCENARIO-05 check --hook scopes a Claude Code hook call to the edited feature
  Given a PostToolUse payload on stdin whose tool_input.file_path is inside feature "auth"
  When I run "brief check --hook claude-code"
  Then only "auth" is checked; exit codes follow check's rules, and the first stderr line stands alone as a summary with the next command
  And a path outside the feature root, or no .brief.yaml found, is silent with exit 0
```

The Gherkin's "check's exit codes / first stderr line" is superseded by R12 as amended
(commit 1b12f18): on Claude Code, a PostToolUse hook's exit 1 never reaches the model; exit 0
with `hookSpecificOutput.additionalContext` on stdout does.

## User-visible contract (pinned by this plan, per amended R12)

Command line: `brief check --hook claude-code`, payload JSON on stdin.

- Path inside feature `<f>` that has at least one ERROR finding: exit 0, stderr empty, stdout is
  exactly one JSON document
  `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"brief check: <feature dir rel>: N ERROR finding(s); run 'brief check <f>'"}}`
  — `<feature dir rel>` is the feature directory through `displayPath(wd, …)`; the noun
  pluralizes on N ("1 ERROR finding", "2 ERROR findings"). No findings table on stdout: stdout
  belongs to the host's JSON protocol.
- Feature with no findings or WARN findings only: nothing on stdout, nothing on stderr, exit 0.
- No `.brief.yaml` anywhere above `wd` (`config.Locate` nearest == ""): silent, exit 0.
- Path outside `<root>/<feature-directory>`, the feature root itself, a file directly in the
  feature root, or a first path element that is not a feature directory: silent, exit 0.
- Invalid `.brief.yaml` (R1): check's normal refusal, exit 1 — the hook does not silence R1
  (Claude Code shows its first stderr line to the user as a hook-error notice).
- Usage errors, exit 2 (`ErrUsage`): unknown host
  (`brief check: unknown host "x"; expected one of: claude-code; run 'brief check --hook claude-code'`);
  `--hook` with a positional feature; `--hook` with `--json`; stdin empty, not JSON, or JSON
  without a non-empty `tool_input.file_path`.
- Config is located from `cli.Run`'s injected `wd` (the hook runs in the session's current
  directory), never the payload's `cwd`. A relative `file_path` resolves against `wd`.

Payload shape relied on: only `tool_input.file_path` (verified by the coordinator against
code.claude.com/docs/en/hooks: absolute path; the payload also carries `cwd`, unused here).

## Implementation Plan

- [x] Step 1: `internal/platform/host/claudecode_test.go` `Test_claude_code_reads_the_edited_path_from_a_post_tool_use_payload` + table of malformed payloads (empty, not JSON, no `tool_input`, empty `file_path`) → `ErrMalformedPayload` (red)
- [x] Step 2: `internal/platform/host/claudecode_test.go` `Test_claude_code_writes_the_summary_as_post_tool_use_additional_context` — decode the written bytes and assert `hookSpecificOutput.hookEventName` and `additionalContext`; a summary carrying quotes/newline stays one valid JSON document (red)
- [x] Step 3: `internal/platform/host/doc.go` — package doc: agent-host adapters (hook payload parsing and hook output rendering now, artifact rendering later), no feature knowledge (new)
- [x] Step 4: `internal/platform/host/host.go` — `Host` interface (`Name`, `HookPath(io.Reader) (string, error)`, `WriteHookContext(io.Writer, string) error`), `ErrMalformedPayload`, `HookHosts() []string`, `Lookup(name) (Host, bool)` (new)
- [x] Step 5: `internal/platform/host/claudecode.go` — `ClaudeCode` name const, payload decode, `hookSpecificOutput` encode (green)
- [x] Step 6: `internal/assemble/feature_containing_test.go` `Test_feature_containing_*` — abs path inside a feature, relative path, path outside the root, `..` escape, the feature root itself, a file directly in the root, a first element that is a regular file, a path reached through a symlinked ancestor (`t.TempDir` `/var` vs `/private/var`) (red)
- [x] Step 7: `internal/assemble/features.go` `(*Server).FeatureContaining(path string) (string, bool)` — lexical `filepath.Rel` against `root/FeatureDirectory` first, retry with `filepath.EvalSymlinks` on both sides only when lexical says outside; first element must be a real directory (green)
- [x] Step 8: `internal/cli/check_hook_test.go` `Test_check_hook_reports_only_the_feature_containing_the_edited_path_as_additional_context` — two features with ERROR findings, payload inside one; exit 0, stderr empty, stdout decodes to the pinned document naming only that feature and its own N (red)
- [x] Step 9: `internal/cli/check_hook_test.go` silent-0 tests (stdout AND stderr empty, exit 0), each with a control arm differing in one variable — the same fixture with an in-feature path whose stdout is the non-empty `additionalContext` document: path outside the feature root; no `.brief.yaml` above `wd` with a `docs/specifications/auth` tree carrying ERROR findings present (so `Resolve`'s defaults WOULD find it); WARN-only / conforming feature (red)
- [x] Step 10: `internal/cli/check_hook_test.go` usage-error matrix via `cli.Run`: unknown host, `--hook` + feature argument, `--hook` + `--json`, empty stdin, non-JSON stdin, missing `file_path` → exit 2 and pinned stderr, empty stdout; invalid `.brief.yaml` → refusal exit 1 (red). "`--hook` + `--json`" pulled out of the table into its own test: R5 routes that usage error to stdout as a JSON document, not stderr — a different assertion shape.
- [x] Step 11: `internal/cli/cli.go` `newRootCommand` — `check` gains a `--hook <host>` flag (usage string `check [feature] [--hook <host>]` per R14), passes `stdin` and the flag value through (update)
- [x] Step 12: `internal/cli/check.go` `runCheckHook` — flag/host validation, `host.Lookup`, `HookPath`, `config.Locate` for the opt-in gate, then `resolveRoot` (R1 refusal), `FeatureContaining`, `Check(feature)`, `countFindings`; ERROR > 0 → the hook summary line through `WriteHookContext` and a nil error (exit 0); `runCheck` dispatches to it when `--hook` is set (green). Added after initial review: `Test_check_hook_resolves_a_relative_edited_path_against_wd`, pinning "a relative `file_path` resolves against `wd`" (the plan's own contract line) — the initial Step 8-10 tests only ever exercised an already-absolute payload path, leaving the `filepath.IsAbs`/`filepath.Join(wd, …)` branch untested; mutation-verified (dropping the join reddens only this one test, confirmed via the same cp-to-`$TMPDIR` protocol).
- [x] Step 13: `internal/cli/check.go` `checkLong` + `hookFlagUsage` — document `--hook`, its silence rules and that findings reach the host as hook context with exit 0; update help goldens (`help_test.go`, `help_json_test.go`) the new flag/usage line moves (update). `help_json_test.go` needed no edit — it derives flags/usage dynamically from text help. `hookFlagUsage` wraps onto a second line (pflag's own embedded-newline cue) to fit `Test_every_leaf_help_line_fits_in_80_columns`.
- [x] Step 14: mutation-verified individually, each copied to `$TMPDIR` first and restored after, `diff` confirming byte-identical restore every time:
  - (a) first pass dropped `featureContainingLexical`'s whole `rel == "."`/`".."`/prefix `if` as one bundle; at `internal/cli` level exactly one test reddened as the plan describes, but decomposing the three clauses individually (mandatory per `go-testing`'s "verify guards individually") showed `rel == "."` and `rel == ".."` are each independently dead — dropping either alone reddens nothing, at either package level, because `strings.SplitN(rel, sep, 2)` already yields one element for both and the `len(parts) < 2` check below catches them. Only `strings.HasPrefix(rel, "../")` is load-bearing (a deeper escape like `"../evil/x"` splits into two parts, and without this guard `os.Lstat(featureRoot + "..")` would pass as a plausible feature directory). Removed the two dead clauses from production code (with a comment recording why); re-verified the sole remaining guard in isolation → the same one test, `Test_check_hook_is_silent_when_the_edited_path_is_outside_the_feature_root`, plus the three `internal/assemble` tests, reddened exactly as before. Full suite re-run green after the simplification.
  - (b) replaced the `config.Locate` opt-in gate with an unreachable condition (`resolveRoot` alone governs) → only `Test_check_hook_is_silent_when_no_brief_yaml_is_found` reddened. Confirmed.
  - (c) dropped the `out.json` exclusivity check in `runCheckHook` → only `Test_check_hook_with_json_reports_the_usage_error_as_json` reddened. Confirmed.
  - (d) dropped `featureContainingLexical`'s `os.Lstat`/symlink/`IsDir` guard → only `Test_FeatureContaining_ReturnsFalseWhenTheFirstPathElementIsARegularFile` reddened (assemble level); no `internal/cli` test reddened. Confirmed.
  - (e) returned `errCheckFindings` instead of `nil` after `WriteHookContext` → reddened six tests, not one: every `check_hook_test.go` test that reaches the ERROR-finding branch and asserts `require.NoError(t, err)` reddened (the Step 8 test, `Test_check_hook_resolves_a_relative_edited_path_against_wd` added after initial review, and Step 9's three control-arm tests that land on the ERROR side: inside-the-root, same-tree-has-.brief.yaml, and same-feature-has-open-step). This is broader than the plan's "only the Step 8 exit-0 assertion" claim because every one of these tests also asserts `require.NoError` before checking stdout — a stronger, not weaker, guard than the plan anticipated. Re-verified after the relative-path test was added (count moved from five to six, same rule). No test outside this branch reddened.
  - (f) changed the hook-path ERROR-count gate to `errorCount+warnCount` → only `Test_check_hook_is_silent_for_a_warn_only_feature` reddened. Confirmed.
- [x] Step 15: `go build ./...` clean; `go test -count=1 ./...` — 12 packages, all `ok` (`cmd/brief` has no test files); `go test -race ./internal/cli/... ./internal/assemble/... ./internal/platform/host/...` — all `ok`; `go test -v` over the same three packages shows 0 `--- SKIP`; `golangci-lint run ./...` — 0 issues. Tick SCENARIO-05 in specification.md, `status: done` here, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Host adapters live in `internal/platform/host` — `internal/cli` (hook parsing + hook output,
  now) and `internal/setup` (artifact rendering, S06) both consume them, and a feature package
  cannot import another feature package. Host-protocol JSON is rendered there, never in cli.
- `host.HookHosts()` is separate from `setup.Hosts()`; `claude-code` is NOT in `setup.Hosts()`
  yet — adding it before S06 makes `init --host claude-code` claim an install that writes
  nothing. S06 reconciles the two lists (setup should derive from the `host` registry).
- Path → feature mapping is `assemble.(*Server).FeatureContaining`; cli never computes it.
- The hook's opt-in gate is `config.Locate` nearest == "" → silent 0, then `resolveRoot` so an
  invalid config still refuses (R1, exit 1). `Resolve` alone returns defaults when no file
  exists and would check a non-opted-in repo.
- Config is located from the injected `wd`, not the payload's `cwd`.
- Hook path (R12 amended, 1b12f18): ERROR findings → exit 0, stdout = the one
  `hookSpecificOutput.additionalContext` document, stderr empty; no-ERROR is fully silent.
  stdout on the hook path carries only host-protocol JSON — never check's table.
- `--hook` excludes both a positional feature and `--json`; malformed payload (empty /
  non-JSON / no `tool_input.file_path`) — all usage errors, exit 2.

**Left unbuilt** — named so nobody assumes it exists:
- Payload `cwd`, `tool_name`, `hook_event_name` handling — unowned; add to the `ClaudeCode`
  adapter only if a later scenario needs them.
- `hooks/hooks.json` rendering, `--no-hook`, host `claude-code` in `setup.Hosts()` — S06.
- `host-hook` doctor row — S10.

**Traps** — things that look right and are not:
- `t.TempDir()` on macOS is `/var/folders/…`, a symlink to `/private/var/…`; a payload path in
  one form and a root in the other makes `filepath.Rel` yield `..` and a silent-0 test pass for
  the wrong reason. Every silent-0 test needs its in-feature control arm.
- Exit 0 is now both the "silent" and the "findings" outcome — a silent-0 test asserting only
  the exit code proves nothing; it must assert stdout empty, against a control whose stdout is
  the non-empty document.
- A symlinked feature directory is itself a check finding; resolving symlinks FIRST would map
  its files outside the root. Lexical match first, `EvalSymlinks` only as the fallback.
- The no-config test must still contain a feature tree with ERROR findings at the default
  `feature-directory`, otherwise dropping the `Locate` gate stays silent and proves nothing.
