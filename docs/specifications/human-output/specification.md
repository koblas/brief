# Specification: Human-oriented diagnostics and `--json` everywhere

## Intent & Goal

**Primary Goal**: Every command's output explains itself to a person — no bare `!`, no `:0`,
relative paths, diagnostics that name the file at fault and the next action — and every
command offers `--json`: one stable machine-readable document carrying status code and
status detail.

**Out of Scope**: color / TTY detection (user decision: not now); `--json` on `completion`
(prints a shell script); collapsing repeated findings in text; R9 dropped-entry reporting
(additive field when R9 lands); R13 truncation (only its JSON clause is written now); the
`mcp` front end.

**User decisions (binding)**: human text is freed — `--json` is the only stable machine
format, byte-pinned text tests are rewritten; `--json` on literally all commands except
`completion`; no color; paths relative to wd in text, absolute in JSON.

## Business Rules & Invariants

- R1: **One JSON document per run on stdout in `--json` mode**, including usage errors and
  refusals; stderr is empty; exit codes unchanged (0 ok / 1 refusal or failure / 2 usage).
- R2: **Common header, flat payload.** Every document starts with `"schema": 1`,
  `"command": "<path>"`, `"ok"`, `"exit_code"`; `ok == (exit_code == 0)`. The command's own
  fields sit beside the header (no `data` wrapper). `schema`, `command`, `ok`, `exit_code`,
  `error` are reserved names. `schema` is one global integer, bumped only on breaking change.
- R3: **Error document.** On failure: `"error": {"kind", "message", "path", "line",
  "problem", "fix", "files_changed"}`. `kind` ∈ `usage` (exit 2), `refusal` (exit 1: invalid
  config, scaffold/assemble refusals, not-found), `failure` (exit 1: anything else). `message`
  is the exact text-mode line. Unused fields are `null`, never omitted. `files_changed` is
  `false` for write commands, `null` for read commands. `fix` always filled.
- R4: **Problems that are data stay data.** `status --json` with malformed features exits 0
  with a per-feature `problem` object; `check --json` findings are payload (exit 1 on any
  ERROR) with no `error` object.
- R5: **`--json` detection.** JSON mode is on iff an exact `--json` token appears in argv
  before `--`, detected before any flag parsing, so flag errors are themselves JSON.
  `--json=<v>` is a text usage error `'--json' takes no value` (exit 2). `--json` after `--`
  is a positional. At the root the sole-argument rule relaxes for exactly `--json`
  (`brief --version --json`, `brief --json --version`); any other trailing token keeps
  "takes no arguments".
- R6: **Paths** absolute in JSON; relative to the working directory in text
  (`filepath.Rel`, falling back to absolute).
- R7: **`new`'s stdout stays exactly one bare path** (a machine contract via
  `f=$(brief new step x)`), now relative to the working directory. All other human text may
  change freely.
- R8: **Findings carry a stable `rule` id** (`frontmatter`, `handoff-cap`, `state-cap`,
  `fence`, `heading`, `checklist`, `depends-on`, …) so scripts never parse English.
- R9: **R14 empty discriminators survive**: `status` prints its header only when at least one
  feature exists (`brief status | wc -l` of 0 still means no features).
- R10: **JSON is never cut at a line boundary**; future truncation is a field
  `"truncated": {"dropped_bytes", "rest"}` (spec R13 amendment).
- R11: **`completion` has no `--json`**: `brief completion bash --json` is a usage error
  document (`completion prints a shell script; --json does not apply`), exit 2.

---

## Triage Brief

**Reproduction (this repo).** `brief status` → three stderr lines naming feature
*directories* by absolute path with fix `run 'brief new step <feature>'` (wrong: the fault is
e.g. `SCENARIO-01.md` lacking frontmatter), then `brief ! ! !` rows, exit 0. `brief check` →
63 flat `[ERROR] <abs>:0 — …` lines, summary `63 finding(s), at least one ERROR`, exit 1.
`brief start --json brief` refuses in plain stderr text (not JSON). `brief status --json` →
`unknown flag: --json`, exit 2. `brief start nosuch` → `no such feature` without naming
known features (R14 violation).

**Root cause of wrong-file diagnostics.** `internal/assemble/errors.go` `newProblem` only
joins the step file onto `base` for `*fs.PathError`; frontmatter parse errors keep `base` =
feature directory and build `Fix` from the feature name. Shared by `status` and `start`.

**Affected surface.** `internal/cli/{cli.go,status.go,check.go,start.go,finish.go,new.go,
refusal.go,classify.go}`; `internal/assemble/{render.go,status.go,check.go,errors.go,
brief.go}`; possibly `internal/scaffold` for finish/new result detail.

**Already exists — do not re-plan.** `leafCommand`'s `addFlags` (start's `--json` is the
precedent); `assemble.Brief` JSON conventions (explicit fields, no `omitempty`,
`step: null`); `RenderJSON` buffers a whole document before writing; `FeatureStatus`
(`Name, Done, Total, Next, Blocked, Problem`) and `Finding` (`Severity, Path, Line, Detail`)
carry most payload fields; `renderRefusal` is the one text seam classifying
`config.InvalidConfigError` / `scaffold.RefusalError` / `assemble.RefusalError` / other;
`ExitCode`; `classifyDashArg` + `runRoot` + help stub for root-level dash args.

**Byte-pinned text tests** exist across `internal/cli/{status,check,start,finish,new_step,
run,help,flag_error}_test.go` and `internal/assemble/{status,check,render}_test.go` —
rewritten per scenario as text changes (user decision).

## Product Verdict

**SHIP WITH CHANGES** (`product-vision`, pre-design). Ranked, all folded above:
1. Common header + flat payload, one document on stdout in `--json` mode, errors as the
   R14a-shaped `error` object, exit codes unchanged.
2. Detect `--json` by exact token before `--`, ahead of flag parsing.
3. Keep problems-as-data out of the error document (status exit 0 with `problem`; check
   findings exit 1, no `error`).
4. Fold in the `newProblem` fix (SCENARIO-04).
5. Stable `rule` id on findings.
6. `new`'s stdout stays a bare path, now relative to the working directory.
7. Write the R13 JSON clause and the version-flag reversal into the specs now (done:
   `docs/specifications/brief/specification.md` R13, R14a, Decisions taken #3, Open
   question 8; `docs/specifications/version-flag/specification.md` verdict item 3).
8. `status` header only when there are features; `check` never renders `:0`.

**Exact human copy ruled by product-vision** (architects use these):
- status table: `FEATURE  DONE  BLOCKED  NEXT` columns; complete → `(complete)`;
  malformed → `-  -  (malformed, see below)`; NEXT shows `<id>  <title>`. stderr after table:
  `brief status: <feature>: <rel path to step file>: <detail>; run 'brief check <feature>' to
  list every fault`, then `brief status: N features: A in progress, B complete, C malformed`.
- check: feature header `<name>  (in flight|complete)`, indented rows
  `  <SEVERITY>  <rel path>[:<line>]  <detail>`; stderr summary
  `brief check: N ERROR, M WARN in K features (<count> <rule>, …); ERRORs block finish on
  in-flight features — 'brief check <feature>' narrows to one`.
- unknown feature: `brief <cmd>: no feature "<name>" in <feature dir>; known: <a>, <b>, …`
  (exit 1).
- finish: `brief finish: <feature> <step> done; wrote <handoff rel>, replaced <state rel>;
  next: <id> — run 'brief start <feature>'` (or no next); re-finish:
  `brief finish: <feature> <step> already done with identical inputs; nothing written`.
- new step stderr: `brief new step: created <id> in <feature>; fill in its acceptance
  criteria and checklist, then 'brief start <feature>'` (new feature analogous).
- help: every `<cmd> --help` lists `--json   print one JSON document on stdout` and a
  paragraph of top-level JSON fields; status/check help add `For scripts, use --json; the text
  layout may change.`

**JSON payload shapes ruled by product-vision**:
- `status`: `{"features":[{"name","path","done","total","blocked","complete",
  "next":{"id","title","path"}|null,"problem":{"path","line","detail","fix"}|null}]}` —
  malformed: `done/total/blocked` null.
- `check`: `{"counts":{"error","warn"},"features":[{"name","path","in_flight",
  "findings":[{"severity","rule","path","line"|null,"detail"}]}]}`.
- `start`: existing `assemble.Brief` fields unchanged + header.
- `finish`: `{"feature","step","changed","handoff_path","state_path","next"|null}`.
- `new feature` / `new step`: `{"feature","step"|null,"path","created":[abs…]}`.
- `--version`: `{"version"}` (`"(devel)"` when unstamped).
- `help`: `{"commands":[{"name","usage","summary","description","flags":[{"name","type",
  "usage"}]}]}`; `help <cmd> --json` ≡ `<cmd> --help --json`, filtered to one entry.

---

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 --json turns usage errors into one JSON document
  When I run "brief status --json --bogus", "brief bogus --json" or "brief --json"
  Then stdout is one JSON document with schema 1, the command, ok false, exit_code 2,
    and error.kind "usage" whose message is the text-mode line
  And stderr is empty and the exit code is 2
  And "brief status --json=x" is a text usage error "'--json' takes no value", and a --json after "--" is a positional

Scenario: SCENARIO-02 --json turns refusals into the error document
  Given a feature whose state file is missing
  When I run "brief start demo --json"
  Then stdout is one document with ok false, exit_code 1, error.kind "refusal", and absolute path, problem and fix filled in
  And stderr is empty and the exit code is 1

Scenario: SCENARIO-03 start --json success carries the common header
  When I run "brief start demo --json" on a feature with an open step
  Then the document has schema, command "start", ok true, exit_code 0, plus every existing brief field unchanged

Scenario: SCENARIO-04 A malformed feature names the step file at fault
  Given a feature whose step file SCENARIO-01.md has no frontmatter
  When I run "brief status" or "brief start <feature>"
  Then the diagnostic names ".../SCENARIO-01.md" (relative path), not the feature directory
  And its fix is "run 'brief check <feature>' to list every fault"

Scenario: SCENARIO-05 An unknown feature lists the known ones
  When I run "brief start nosuch" (likewise finish, new step, check)
  Then stderr is: brief start: no feature "nosuch" in docs/specifications; known: <names>
  And the exit code is 1

Scenario: SCENARIO-06 status reads as a table for people
  Given features in progress, complete and malformed
  When I run "brief status"
  Then stdout has a header FEATURE/DONE/BLOCKED/NEXT, words instead of "!" ("(complete)", "(malformed, see below)"), and the next step's title
  And stderr lists one line per malformed feature naming its file, then a summary line with counts
  And with no features, stdout is empty (no header) and the exit code is 0

Scenario: SCENARIO-07 status --json
  When I run "brief status --json"
  Then the document has features[] with name, absolute path, done, total, blocked, complete, next {id,title,path} or null, problem or null
  And malformed features have null counts and a problem object, and the exit code is 0

Scenario: SCENARIO-08 check groups findings by feature and names each rule
  When I run "brief check"
  Then findings are grouped under a feature header, paths are relative, a whole-file finding shows no ":0"
  And each finding carries a stable rule id
  And the stderr summary counts ERROR/WARN per rule and names the next action

Scenario: SCENARIO-09 check --json
  When I run "brief check --json"
  Then the document has counts {error, warn} and features[] with findings {severity, rule, absolute path, line or null, detail}
  And the exit code is 1 when any ERROR exists, with no error object

Scenario: SCENARIO-10 new feature / new step say what happened and what's next
  When I run "brief new step auth"
  Then stdout is exactly one path, relative to the working directory
  And stderr says what was created and the next command
  And "--json" gives {feature, step, path, created[]}

Scenario: SCENARIO-11 finish reports what it wrote and what's next
  When I finish a step
  Then stderr names the step, the files written, and the next step with its start command
  And a re-finish with identical inputs says "already done with identical inputs; nothing written"
  And "--json" gives {feature, step, changed, handoff_path, state_path, next}

Scenario: SCENARIO-12 --version --json in either order
  When I run "brief --version --json" or "brief --json --version"
  Then the document has the common header plus version (or "(devel)")
  And any other trailing token after --version is still "takes no arguments"

Scenario: SCENARIO-13 help --json is a machine-readable command index
  When I run "brief help --json" or "brief --help --json"
  Then the document lists commands[] with name, usage, summary, description, flags[]
  And "brief help start --json" and "brief start --help --json" return the same document filtered to start
  And "brief completion bash --json" is a usage error document

Scenario: SCENARIO-14 Every command's help advertises --json
  When I run any "<cmd> --help"
  Then it lists "--json  print one JSON document on stdout" and the command's top-level JSON fields
  And status and check help say "For scripts, use --json; the text layout may change."
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: --json turns usage errors into one JSON document
- [x] SCENARIO-02: --json turns refusals into the error document
- [x] SCENARIO-03: start --json success carries the common header
- [x] SCENARIO-04: A malformed feature names the step file at fault
- [x] SCENARIO-05: An unknown feature lists the known ones
- [x] SCENARIO-06: status reads as a table for people
- [x] SCENARIO-07: status --json
- [x] SCENARIO-08: check groups findings by feature and names each rule
- [x] SCENARIO-09: check --json
- [x] SCENARIO-10: new feature / new step say what happened and what's next
- [x] SCENARIO-11: finish reports what it wrote and what's next
- [x] SCENARIO-12: --version --json in either order
- [ ] SCENARIO-13: help --json is a machine-readable command index
- [ ] SCENARIO-14: Every command's help advertises --json
