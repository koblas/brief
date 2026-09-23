---
id: SCENARIO-04
status: done
---

# SCENARIO-04: doctor reports the brief-workflow skill's health (host-skill)

## Scenario

```gherkin
Scenario: SCENARIO-04 doctor reports the brief-workflow skill's health (host-skill)
  Rows for not installed (SKIP), missing beside plugin (WARN), unreadable (WARN),
  not a regular file (ERROR, exit 1), older (WARN), edited (OK), current (OK)
```

User-visible contract (`brief doctor`, text and `--json`), verbatim from the specification's
`## Surface & Copy` table. The row id is `host-skill` and the path is
`.claude/skills/brief-workflow/SKILL.md`. It sits after `host-hook` and before `host-snippet`:
`… host-plugin, host-hook, host-skill, host-snippet, host-agents, roles`.

- no Plugin(true) ∪ Agents() file present, and the skill absent → SKIP `not installed`, fix `run 'brief init --host claude-code'`
- skill absent while a plugin or agent file is present → WARN `not installed; bound agents cannot preload it`, fix `run 'brief init'`
- unreadable → WARN `not readable (<reason>)` (`notReadableReason`), fix `notReadableFix(...)`
- not a regular file → ERROR `not a regular file`, fix `run 'brief init'`, doctor exits **1**
- older render → WARN `installed by an older brief release`, fix `run 'brief init'`
- edited → OK `edited locally`, no fix
- current → OK `installed`, no fix

Rows go to stdout and the summary goes to stderr, as today. The exit code is 1 only when some row is ERROR. `--json` `checks[]` gains the row, and `counts` keeps its shape.

Sources: STATE.md, the specification (Rules 1/7, Product Verdict item 7, Surface & Copy `brief doctor`), and the `internal/doctor` surface. No prior SCENARIO file was opened.

## Implementation Plan

### Red
- [x] Step 1: `internal/doctor/doctor_test.go` `newHealthyDoctorFixture` + `Test_diagnose_reports_every_check_ok_in_a_healthy_repository`. The fixture also writes every `h.Skills()` file from `artifact.Render`. Insert `"host-skill"` between `"host-hook"` and `"host-snippet"` in the pinned id list. Fails: the row is not in the order.
- [x] Step 2: `internal/doctor/host_test.go` `Test_diagnose_classifies_host_skill` (table through `runHostCheckCases`, `wantPathSuffix` = the skill relpath). Cases: nothing installed → SKIP; plugin files present and skill absent → WARN missing; skill mode 0o000 beside the plugin → WARN not readable with the `notReadableFix` text; skill path is a directory → ERROR; edited bytes → OK `edited locally`; current render → OK `installed`; skill alone, with no plugin or agent file → OK `installed` (the skill classifies itself). Fails: no `host-skill` row, so `findCheck` fails.
- [x] Step 3: `internal/doctor/host_internal_test.go` `Test_hostSkillRow_reports_an_older_render_as_warn`. White-box call of the state→Check func with a synthetic `integrationFileState{present, regular, origin: OriginOlder}`. Assert ID `host-skill`, WARN, the older detail, and fix `run 'brief init'`. This is the only way to reach the arm: `olderSkillWorkflowDigests` ships empty. Fails: the func does not exist.
- [x] Step 4: `internal/doctor/doctor_test.go` `Test_diagnose_classifies_env_path_by_whether_the_integration_is_installed`. Add the case "not on PATH, only the brief-workflow skill is installed" → WARN. This is **green on arrival**: it pins the install-signal decision (Handoff). The existing "plugin is installed → ERROR" case is its control, and the two differ only in which file is written. Proven in Verify, not by a red here.
- [x] Step 5: `internal/cli/doctor_internal_test.go` `newFullyInstalledDoctorFixture` + a new `Test_doctor_reports_a_non_regular_host_skill_as_an_error_and_exits_1`. The fixture also writes `h.Skills()`. The new test replaces SKILL.md with a directory, then asserts `ExitCode(err) == 1`, the exact stdout line `ERROR  host-skill  .claude/skills/brief-workflow/SKILL.md  not a regular file; fix: run 'brief init'`, and stderr `brief doctor: 1 ERROR, 0 WARN; …`. Fails: no such row, and exit 0.

### Green
- [x] Step 6: `internal/doctor/host.go` `skillFileOf(h)` — returns `h.Skills()`'s single entry. Derive the path from `Skills()` and never hardcode it.
- [x] Step 7: `internal/doctor/host.go` `hostSkillCheck(wd, root, h, installed)` + the unexported state→Check func (`hostSkillRow`). `hostSkillCheck` probes with `probeIntegrationFile`. The func applies, in order: absent and !installed → SKIP; absent → WARN missing; unreadable → WARN; !regular → ERROR; everything else goes through `originRow(state.origin, runInit, "")`. Add a named constant for the missing detail. Leave `hostHookCheck` untouched.
- [x] Step 8: `internal/doctor/doctor.go` `Diagnose`. Append `hostSkillCheck(absWd, root, h, filesInstalled)` between the host-hook and host-snippet rows. Keep `anyIntegrationFilePresent` unchanged: the skill is not an install signal.

### Sweep
- [x] Step 9: `internal/cli/doctor_internal_test.go` — count/order bumps (sweep). In `Test_doctor_prints_one_row_per_check_and_exits_0_in_a_healthy_repo`, insert `SKIP  host-skill  .claude/skills/brief-workflow/SKILL.md  not installed; fix: run 'brief init --host claude-code'` after the host-hook line. In `Test_doctor_json_reports_absolute_paths_null_fix_and_counts`, change `Len` from 12 to 13 and `Counts.Skip` from 5 to 6.
- [x] Step 10: `internal/doctor/doctor.go`, `host.go`, `doc.go` — doc comments (sweep). Update:
  - the `Check` ID list
  - the SeverityWarn/SeverityError/SeveritySkip enumerations (ERROR gains host-skill's non-regular file)
  - `Diagnose`'s fixed-order list
  - `doc.go`'s "five host-integration rows" count and its integration list
  - `originRow`'s row list ("the four")
  - `runInit`'s row list
  - `anyIntegrationFilePresent`: state that `Skills()` is excluded on purpose, and why
  - a contract doc on `hostSkillCheck`/`hostSkillRow` in `hostHookCheck`'s style
- [x] Step 11: fix what `go build ./... && golangci-lint run ./...` reports (sweep).

### Verify
- [x] Step 12: full verification per `.claude/rules/agent-briefs.md`, reporting the test count and its delta. Then run two mutations, **each on its own**, from a fresh `$TMPDIR` copy:
  - (a) In `hostSkillRow`, change the non-regular arm's `SeverityError` to `SeverityWarn`. Both `Test_doctor_reports_a_non_regular_host_skill_as_an_error_and_exits_1` and the ERROR case of `Test_diagnose_classifies_host_skill` must go red.
  - (b) Add `anyPresent(probeIntegrationFiles(root, h.Skills()))` to `anyIntegrationFilePresent`. The skill-alone case of `Test_diagnose_classifies_env_path_by_whether_the_integration_is_installed` must go red, while its plugin-installed control stays green.

  Restore each file and `diff` it against its copy.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- **The skill is not an install signal.** `anyIntegrationFilePresent` stays Plugin(true) ∪ Agents() only, and does not include `Skills()`. The reason is Rule 1: uninstall keeps an edited SKILL.md. If the skill counted, a clean `brief uninstall` that kept an edited skill would flip host-plugin to ERROR "incomplete", host-hook to WARN and env-path to ERROR, so doctor would exit 1 after a clean uninstall. Pinned by the skill-alone case in `Test_diagnose_classifies_env_path_by_whether_the_integration_is_installed`.
- **host-skill's missing WARN keys on `filesInstalled`, not `integrationInstalled`.** A repo with only the CLAUDE.md snippet reports host-skill SKIP, the same as host-hook.
- **A present skill always classifies itself**, even with no plugin (OK/WARN/ERROR by its own state). SKIP only happens when the skill is absent and nothing else is installed.
- **host-skill's older arm goes through `originRow`** (single-subject, suffix `""`, fix `runInit`) and nowhere else. All host rows keep one origin precedence.

**Left unbuilt** — named so nobody assumes it exists:
- The `roles-skill` doctor row, plus doctorLong's roles-clause replacement. S05 owns both. doctorLong gets **no** host-skill sentence in S04, because the spec rules only the roles clause.
- A fixture-reachable OriginOlder for the skill. `olderSkillWorkflowDigests` ships empty, so only `hostSkillRow`'s white-box test reaches that arm.
- `--edit-agents`, `agents_missing_skill`, and the uninstall bound-agent edits: S06–S08.

**Traps** — things that look right and are not:
- Doctor's row order is `host-hook, host-skill, host-snippet, host-agents`. STATE.md's **init report** order (`hook, skill, agent…, snippet`) is a different list, so don't copy it into doctor.
- Every fixture that writes a "full install" (`newHealthyDoctorFixture`, `newFullyInstalledDoctorFixture`) must also write `h.Skills()`. If it doesn't, the all-OK tests go red on a WARN rather than on the behaviour under test. A new full-install fixture needs the same.
- host-skill is a single-subject row built from `h.Skills()`'s one entry. If `Skills()` ever returns two files, the row silently checks only one.
- Don't route the skill through `hostHookCheck` or through a shared helper with it. The missing-arm copy and fix differ (`installed without the check hook` / `run 'brief init' to add it` against the skill's own strings).
