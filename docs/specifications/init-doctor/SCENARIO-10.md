---
id: SCENARIO-10
status: done
---

# SCENARIO-10: doctor reports Claude Code integration health

## Scenario

```gherkin
Scenario: SCENARIO-10 doctor reports Claude Code integration health
  Given init ran with --host claude-code
  When I run "brief doctor"
  Then rows host-plugin, host-hook, host-snippet, host-agents and roles report OK, or WARN when installed by an older version, or "edited locally", or SKIP when not installed, each with a fix
  And env-path is ERROR when the integration is installed but brief is not on PATH
```

Inherited from STATE.md (not re-read from prior scenarios): `config.Locate` root rule; doctor
imports only `internal/platform/*`; `host.Host` Plugin/Agents/InstructionFiles; plugin agents
addressed `brief:<role>` and shadowed by a same-name project `.claude/agents/<role>.md`; doctor
has no host detection. S06's handoff (stray-plugin question) was read via grep only.

## User-visible contract

`brief doctor [--json]` — unchanged surface. Rows appended after `env-path`, in this order:
`host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` (one row each — R13's id list
is closed; no new ids, no new Severity). Stdout rows / stderr summary / exit code follow S02
unchanged: exit 1 on any ERROR, else 0; `setup ok` only when no ERROR and no WARN (so every
"not installed" answer is SKIP, never WARN — a default `brief init --host none` stays `setup ok`).

Install root = `config.Locate`'s dir when a config is found (including the unparseable arm),
else `wd`. Host is always claude-code (`host.Lookup(host.ClaudeCode)`); no detection.

- `host-plugin` (subject: `Plugin(false)` = manifest + two skills; path = `<root>/.claude/skills/brief`)
  - no file of `Plugin(true)` ∪ `Agents()` present → SKIP `not installed`, fix `run 'brief init --host claude-code'`
  - any subject file missing or not a regular file while another integration file exists → ERROR `incomplete: missing <rel, …>`, fix `run 'brief init'`
  - any subject file `OriginOlder` → WARN `installed by an older brief release: <rel, …>`, fix `run 'brief init'`
  - any subject file `OriginEdited` → OK `edited locally: <rel, …>`, fix nil
  - all current → OK `installed`
- `host-hook` (path = `<root>/.claude/skills/brief/hooks/hooks.json`)
  - host-plugin is SKIP and no hook file → SKIP `not installed`, fix `run 'brief init --host claude-code'`
  - plugin present, hook absent → WARN `installed without the check hook`, fix `run 'brief init' to add it` (doctor cannot tell `--no-hook` from a lost file; WARN keeps exit 0)
  - not a regular file → ERROR; older → WARN fix `run 'brief init'`; edited → OK `edited locally`; current → OK `installed`
- `host-snippet` (path = the candidate from `InstructionFiles()` under root)
  - lone/duplicate/misordered marker in any candidate → ERROR, detail = scanner problem + `line N`, fix = scanner fix
  - both candidates hold a block → ERROR on `.claude/CLAUDE.md`, fix `delete that block`
  - block `OriginCurrent`, `Dir` equals configured feature directory (trailing `/` trimmed) → OK `installed`
  - block current for a different dir → WARN `names <dir>/, .brief.yaml says <dir>/`, fix `run 'brief init'`
  - config unparseable → no dir comparison; current block → OK `installed (feature directory not compared: .brief.yaml did not parse)`
  - older → WARN fix `run 'brief init'`; edited → OK `edited locally`
  - no block in any candidate (includes a CRLF file — its lines never equal an LF marker) → SKIP `not installed`, fix `run 'brief init --host claude-code'`
- `host-agents` (path = `<root>/.claude/skills/brief/agents`)
  - none of the three present → SKIP `not installed`, fix `run 'brief init --with-agents'`
  - some missing / not regular → WARN `missing <rel, …>`, fix `run 'brief init --with-agents'`
  - older → WARN; edited → OK `edited locally`; all current → OK `installed`
- `roles` (path = nearest `.brief.yaml`, `""` when none) — WARN never ERROR (R1 amendment: roles are reported, not enforced)
  - no config found, or all three bindings empty → SKIP `no roles bound`, fix `run 'brief init --with-agents'`
  - config unparseable → SKIP `skipped: .brief.yaml did not parse`, fix nil
  - per bound role: `brief:<name>` resolves when `<root>/.claude/skills/brief/agents/<name>.md` **or** `<root>/.claude/agents/<name>.md` is a regular file (the project agent overrides the plugin one; either way something runs); any other `<plugin>:<name>` counts as bound, unverified; a bare `<name>` resolves via `<root>/.claude/agents/<name>.md` or `<home>/.claude/agents/<name>.md`
  - any unbound role or unresolved binding → WARN, detail lists each (`planner unbound; reviewer: brief:reviewer not found`), fix `bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'`
  - all bound and resolved → OK `planner, implementer, reviewer bound` (append `not verified: <role>` for other-plugin bindings)
- `env-path`: brief not on PATH **and** integration installed (any file of `Plugin(true)` ∪ `Agents()` present, or a snippet block found) → ERROR `brief not found on PATH; the Claude Code integration runs it`, fix `install brief on PATH`. Not installed → unchanged WARN. Different-version WARN unchanged.

SKIP rows may carry a Fix (the scenario's "each with a fix"): a not-installed SKIP names the install
command; SKIPs meaning "could not run" (root-dir, roles on an unparseable config) keep fix nil. OK rows
always have a nil fix, so the `--json` null-fix golden still has something to assert. Step 24 rewrites
`Check.Fix`'s doc accordingly.

Stray-plugin check (S06 handoff): **not built.** doctor cannot know Claude Code's launch dir,
and R13's closed id set has no row for it.

## Implementation Plan

Artifact: `OriginOlder` and the marker scanner move down

- [x] Step 1: `internal/platform/artifact/digest_internal_test.go` `Test_classify_reports_older_for_a_digest_only_in_the_older_list` — white-box against an unexported `classify(current, older [][32]byte, body)` with a synthetic older list: current / older / edited / a digest in both lists is current (red)
- [x] Step 2: `internal/platform/artifact/digest.go` — `OriginOlder` constant; split every Kind into its current list (unchanged — `configDigests` keeps **both** current renders) and an empty `older…Digests` list; `Recognize` delegates to `classify`; rewrite `Origin`'s and `Recognize`'s doc (green)
- [x] Step 3: `internal/platform/artifact/snippet_internal_test.go` `Test_recognizeSnippetWith_reports_older_for_an_older_template` — white-box over an unexported helper taking current and older template lists (red)
- [x] Step 4: `internal/platform/artifact/snippet.go` — `olderSnippetTemplates` (empty) + helper; `RecognizeSnippet` returns `OriginOlder` with `Dir` for an older template (green)
- [x] Step 5: `internal/platform/artifact/snippet_scan_test.go` — move the three `Test_scanSnippetMarkers_*` tests from `internal/setup/snippet_internal_test.go`, renamed to the exported symbol (red: symbol missing)
- [x] Step 6: `internal/platform/artifact/snippet_scan.go` — exported `ScanSnippetMarkers(body) (*SnippetSpan, *MarkerProblem)` with `SnippetSpan{Start, End, BeginLine}` and `MarkerProblem{Line, Problem, Fix}`, moved verbatim from setup; update the `SnippetBegin`/`SnippetEnd` doc and package doc (artifact now owns locating the span; setup owns merge/remove) (green)
- [x] Step 7: `internal/setup/snippet.go` `scanSnippetCandidates` — delete `markerProblem`/`scanSnippetMarkers`, call `artifact.ScanSnippetMarkers` at that single call site and convert its `*SnippetSpan` into setup's own private `snippetSpan` (kept by design: `mergeSnippet`, `removeSnippet`, `chooseSnippetLocation`, `snippetArtifact` and the `Test_mergeSnippet_*`/`Test_removeSnippet_*` literals stay byte-identical); green on arrival (refactor)

Doctor: seams, root, fixtures

- [x] Step 8: `internal/doctor/doctor.go` `WithHomeDir(func() (string, error)) Option` — default `os.UserHomeDir`; a home error or `""` means no home agents (new)
- [x] Step 9: `internal/doctor/doctor_test.go` — extend `newHealthyDoctorFixture` into a fully installed repo (plugin + hook + agents written from `artifact` renders at `host.Lookup(host.ClaudeCode)`'s paths, root `CLAUDE.md` = `artifact.SnippetBlock` for the configured dir, roles bound to `artifact.AgentBindings()`), inject an empty-dir `WithHomeDir`; extend `Test_diagnose_reports_every_check_ok_in_a_healthy_repository`'s pinned id list to the full 12-row order (red)
- [x] Step 10: `internal/doctor/doctor.go` `Diagnose` — compute install root in all three arms; scan host state once before `env-path`; append the five rows after it (green scaffold: rows present, severities filled by the steps below)

Doctor: the five rows, each red then green (new file `internal/doctor/host.go`, tests in `internal/doctor/host_test.go`, external package, fixtures built from `artifact` renders — never through `internal/setup`)

- [x] Step 11: `host_test.go` `Test_diagnose_classifies_host_plugin` — table: not installed SKIP / manifest missing ERROR / skill a directory ERROR / hook-only install ERROR / edited OK `edited locally` / healthy OK; each row asserts severity, detail and fix (red)
- [x] Step 12: `host.go` — per-file probe (Lstat, regular, read, `artifact.Recognize`) and the host-plugin aggregate (ERROR > WARN older > OK edited > OK) (green)
- [x] Step 13: `host_test.go` `Test_diagnose_classifies_host_hook` — plugin absent SKIP / plugin present hook absent WARN + fix / hook a directory ERROR / edited OK / healthy OK (red)
- [x] Step 14: `host.go` — host-hook row (green)
- [x] Step 15: `host_test.go` `Test_diagnose_classifies_host_snippet` — lone begin ERROR with line / two blocks ERROR on `.claude/CLAUDE.md` / block only in `.claude/CLAUDE.md` OK / block for another dir WARN / unparseable config + block OK-not-compared / edited OK / no CLAUDE.md SKIP / CRLF block SKIP (red)
- [x] Step 16: `host.go` — host-snippet row over `InstructionFiles()` using `artifact.ScanSnippetMarkers` + `artifact.RecognizeSnippet` (green)
- [x] Step 17: `host_test.go` `Test_diagnose_classifies_host_agents` — none SKIP / one missing WARN / edited OK / healthy OK (red)
- [x] Step 18: `host.go` — host-agents row (green)
- [x] Step 19: `host_test.go` `Test_diagnose_classifies_roles` — no config SKIP / all empty SKIP / unparseable SKIP / one unbound WARN / `brief:reviewer` with plugin file deleted WARN / same deleted but `.claude/agents/reviewer.md` present OK / bare name found in project OK / bare name found only in injected home OK / bare name nowhere WARN / `other:agent` OK not verified (red)
- [x] Step 20: `host.go` — roles row (green)
- [x] Step 21: `doctor_test.go` `Test_diagnose_classifies_env_path` — add rows: not on PATH + plugin installed ERROR / not on PATH + snippet only ERROR / not on PATH + nothing installed WARN (control arm: differs only in the integration) (red)
- [x] Step 22: `internal/doctor/checks.go` `(*Server).checkEnvPath` — take the integration-installed predicate; ERROR arm (green)
- [x] Step 23: `internal/doctor/host_internal_test.go` `Test_originRow_reports_older_as_warn_with_the_init_fix` — `host.go` factors an unexported pure origin→row mapping (e.g. `originRow`) used by plugin, hook, snippet and agents; the test calls it directly with `artifact.OriginOlder` for each row id (WARN + `run 'brief init'`) — the only way to reach the arm while every older list is empty (red→green)
- [x] Step 24: `internal/doctor/doc.go`, `Check` doc (incl. `Fix`: may be set on a not-installed SKIP), `Diagnose` doc, `SeveritySkip`/`SeverityWarn` docs — name the five rows, the root rule and the home seam (update)

CLI

- [x] Step 25: `internal/cli/doctor_internal_test.go` — `doctorFakeSeams` gains an empty-dir `doctor.WithHomeDir`; healthy golden (text) and `--json` golden gain the five rows (SKIP in the bare fixture: stderr stays `setup ok`); add one fully installed case through `run` (all OK) and one not-on-PATH-with-plugin case (exit 1, `1 ERROR, 0 WARN` summary) (red)
- [x] Step 26: `internal/cli/doctor.go` `doctorLong` — name host integration and roles among what doctor checks, and the home read for roles (green)

Verification

- [x] Step 27: mutation verification — copy each file to `$TMPDIR` first, never `git stash`; one mutation at a time, restore with `cp`, then `diff` to prove byte-identical:
  1. `host.go`: host-hook's "plugin present, hook absent" arm returns SKIP → Step 13's WARN row reddens
  2. `host.go`: roles' project-override lookup removed → Step 19's `.claude/agents/reviewer.md` row reddens
  3. `host.go`: snippet dir comparison removed → Step 15's other-dir WARN row reddens
  4. `checks.go`: installed predicate forced false → Step 21's ERROR rows redden, the WARN control stays green
  5. `host.go`: `originRow`'s `OriginOlder` arm returns OK → only Step 23 reddens
  6. `digest.go`: `classify` checks the older list before the current one → Step 1's in-both-lists case reddens (proves the test, not production behavior)
- [x] Step 28: `go build ./...`, `go test ./...` (unpiped), `go test -race ./internal/doctor/... ./internal/platform/artifact/... ./internal/setup/... ./internal/cli/...`, `golangci-lint run ./...`; report per-package test counts and deltas
- [x] Step 29: all tests green → mark SCENARIO-10 done in specification.md; rewrite STATE.md

## Handoff

**Binding decisions:**
- Host rows follow `env-path` in the fixed order host-plugin, host-hook, host-snippet, host-agents, roles; one row per id — R13's id list is closed, `config-values` is the only multi-row id
- "Not installed" is always SKIP, never WARN — a default `brief init` (no agents, no roles) has to stay `setup ok`. A not-installed SKIP carries the install command as its Fix; could-not-run SKIPs and every OK carry nil
- Install root = `Locate`'s dir (the unparseable arm too), else wd; host fixed to claude-code, no detection in doctor
- Integration installed = any `Plugin(true)` ∪ `Agents()` file present under root, or a snippet block found. That makes env-path's not-on-PATH ERROR; nothing else is conditional
- `artifact.OriginOlder` means the digest is in a Kind's *older* list, never "index > 0": `configDigests` holds two *current* renders (`ConfigFile`, `ConfigFileWithRoles`)
- `artifact.ScanSnippetMarkers` is the one marker scanner (setup converts its result into a private `snippetSpan` at one call site). setup and doctor must both call it, because the lone-marker path/line/fix is a user-visible contract that the two commands must never report differently
- roles: WARN never ERROR; `brief:<name>` resolves via the plugin agent **or** a project `.claude/agents/<name>.md`; home lookups only through `doctor.WithHomeDir`
- doctor imports `platform/{config,host,artifact}` + stdlib only — never `internal/setup`, not even from tests
- Two distinct "installed" predicates, not one: `filesInstalled` (any `Plugin(true)` ∪ `Agents()` file present) gates host-plugin's and host-hook's own SKIP; the broader `integrationInstalled` (`filesInstalled` **or** a snippet block found) gates only env-path's ERROR arm. Passing the broader flag into `hostPluginCheck`/`hostHookCheck` would make a snippet-only install report host-plugin as non-SKIP with nothing to show
- `host.go`'s `originRow(origin, olderFix, suffix)` is the one place the WARN-older/OK-edited/OK-current triple is decided; host-plugin/host-agents call it per aggregate list (`": <rel, …>"` suffix, their own `olderFix`), host-hook/host-snippet call it per single subject (`""` suffix, always `runInit`)
- `artifact.extractSnippetDir` derives each template's own prefix/suffix by rendering it once with a sentinel string (`snippetDirSentinel`) rather than the old hardcoded `snippetTemplatePrefix`/`snippetTemplateSuffix` constants — required so a genuine second (older) template, with different fixed prose, is still recognized; `SnippetBlock`'s own render is unaffected, and every pre-existing `RecognizeSnippet` test (own dir, different dir, edited, CRLF) still passes unmodified as the control

**Left unbuilt:**
- setup's handling of `OriginOlder` (init rewrite, uninstall remove). setup still compares `== OriginCurrent`, so an older file is treated as `kept "edited locally"`. Safe only while every `older…Digests`/`olderSnippetTemplates` list is empty. The first release that appends an older digest must teach setup to rewrite and remove older files first; otherwise doctor's `run 'brief init'` fix never clears the WARN. **Unowned — no scenario currently closes it; the final review must rule on it.**
- A stray-plugin row (`wd/.claude/skills/brief` when wd ≠ root): doctor cannot know Claude's launch dir, and no R13 id exists for it
- Agent resolution by frontmatter `name:`; roles checks filenames only
- `~/.claude/skills/brief` (user-scope plugin): never consulted

**Traps:**
- `Report.Counts()` has no default arm. A row carrying anything but the four Severity constants is silently left out of counts, the summary and the exit code
- `newHealthyDoctorFixture` (internal/doctor) and the cli doctor goldens (`doctor_internal_test.go`) both pin the full 12-row order and content. Adding or reordering a row without updating both turns an unrelated test red
- A CRLF CLAUDE.md with a block reads as "no block" (SKIP), while init refuses that same file. That difference is deliberate; don't add a CRLF row
- A doctor test that leaves `WithHomeDir` unset reads the developer's real `~/.claude/agents`. Every test that binds a bare-name role, or builds a `Server` directly rather than through a helper that already injects one, must inject a home
- The `OriginOlder` arms are reachable only white-box, via `host.originRow` (`host_internal_test.go`) and `artifact.classify`/`recognizeSnippetWith` (`digest_internal_test.go`, `snippet_internal_test.go`). Any end-to-end older-file assertion is vacuous until a real older digest ships
- host-agents' own fix is always `--with-agents`, even for an "older render" WARN — a plain `brief init` never calls `planAgentFiles` at all (setup.go), so it can never repair an agent file on its own; don't copy host-plugin's plain `run 'brief init'` fix onto host-agents
