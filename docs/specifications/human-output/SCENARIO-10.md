# SCENARIO-10: new feature / new step say what happened and what's next

## Scenario

```gherkin
Scenario: SCENARIO-10 new feature / new step say what happened and what's next
  When I run "brief new step auth"
  Then stdout is exactly one path, relative to the working directory
  And stderr says what was created and the next command
  And "--json" gives {feature, step, path, created[]}
```

## User-visible contract

- `brief new feature <name>` → stdout: exactly one line, the feature directory relative to
  wd (R6/R7, `displayPath`). stderr: exactly one line
  `brief new feature: created <name> (<spec rel>, <state rel>); add a step with 'brief new step <name>'`.
  Exit 0.
- `brief new step <feature>` → stdout: exactly one line, the step file relative to wd.
  stderr: exactly one line
  `brief new step: created <id> in <feature>; fill in its acceptance criteria and checklist, then 'brief start <feature>'`.
  Exit 0.
- `--json` (either command) → stdout: one compact document
  `{"schema","command","ok","exit_code","feature","step","path","created"}`; stderr: zero
  bytes; no bare path line. `command` is `"new feature"` / `"new step"` (`commandName`
  trims `"brief "` from `CommandPath()` — confirmed). `step` is `null` for `new feature`,
  the id (e.g. `"SCENARIO-01"`) for `new step`. `path` and every `created[]` entry absolute.
- Failure classes are unchanged and already covered (usage exit 2, refusal exit 1, unknown
  feature, invalid config, JSON error documents from S02/S03) — no new rows planned for them.

**Green on arrival (verified, not re-planned):** stdout is already relative to wd —
`new.go` runs `filepath.Rel(wd, path)` before printing, and
`Test_creates_the_feature_and_prints_its_path` / `Test_creates_the_step_and_prints_its_path`
already pin `docs/specifications/payments\n` and `.../SCENARIO-01.md\n`. STATE.md's
"new's stdout path still absolute" entry is stale; the developer drops it.

**Call-site churn (checked):** of ~56 `NewFeature`/`NewStep` calls in scaffold tests, ~46 are
`_, err :=` and need no edit; ~10 bind the path (`path`, `featurePath`, `featureDir`,
`stepPath`) and become `<res>.Path`. None is structural.

## Implementation Plan

- [x] Step 1: `internal/scaffold/scaffold_test.go` `Test_new_feature_reports_the_directory_and_the_files_it_created` — `Path` is the feature dir; `Created` is exactly `[spec abs, state abs]` in write order, each now existing (red)
- [x] Step 2: `internal/scaffold/step_test.go` `Test_new_step_reports_the_step_id_and_only_the_step_file_as_created` — `Step` equals the id the step file's name carries; `Created` is exactly `[step file abs]`; control arm: the specification's bytes changed (progress entry appended) yet it is absent from `Created` (red)
- [x] Step 3: `internal/scaffold/scaffold.go` — new exported result type (feature name, step id or empty, `Path`, `Created []string`) with a contract doc comment; `NewFeature`/`NewStep` return it instead of a bare string; update both doc comments (green)
- [x] Step 4: `internal/scaffold/*_test.go` (`scaffold_test.go`, `step_test.go`, `finish_checklist_test.go`, `finish_headings_test.go`) — mechanical `path` → `res.Path` at the ~10 binding sites (update)
- [x] Step 5: `internal/cli/run_test.go` `Test_creates_the_feature_and_prints_its_path` — add exact stderr line assertion replacing `assert.Empty(stderr)` (red)
- [x] Step 6: `internal/cli/new_step_test.go` `Test_creates_the_step_and_prints_its_path` — add exact stderr line assertion replacing `assert.Empty(stderr)` (red)
- [x] Step 7: `internal/cli/run_test.go` ancestor-config test (wd = `root/a/b`, `feature-directory: specs`) — assert stdout is `../../specs/payments\n` and stderr's rel paths use the same base; replace its `assert.Empty(stderr)` (red on stderr; stdout half green on arrival — say so)
- [x] Step 8: `internal/cli/new.go` `runNewFeature` / `runNewStep` — consume the result type; print stdout via `displayPath(out.wd…)` replacing the inline `filepath.Rel` copies; write the ruled stderr line after stdout (green)
- [x] Step 9: `internal/cli/new_json_test.go` (new) `Test_new_feature_json_is_one_exact_document` — exact-bytes golden (`assert.Equal`, key order pinned), abs paths built from `filepath.Join(wd, …)`, `step:null`, stderr empty, exit 0 (red)
- [x] Step 10: `internal/cli/new_json_test.go` `Test_new_step_json_names_the_step_and_its_file` — decode; `step` equals the id captured from the text-mode run's stdout filename (not a literal); `path` equals the absolute form of that same stdout line; `created` = `[path]`; stderr empty; no bare-path line on stdout (red)
- [x] Step 11: `internal/cli/new.go` — `newDocument` struct (embeds `jsonHeader` first by value; `Feature string`, `Step *string`, `Path string`, `Created []string` non-nil) with doc comment; JSON branch via `out.successHeader()` + `writeJSONDocument`, placed **before** any stdout-path or stderr write (green)
- [x] Step 12: mutation-verify, one at a time, stashed per the standing brief: (a) move each command's JSON branch after the stderr write → that command's JSON stderr-empty assertion goes red (two separate mutations); (b) append the spec path to `NewStep`'s `Created` → Step 2 red; (c) set `Step` on `NewFeature`'s result / drop the `*string` nil → Step 9 golden red; (d) revert `displayPath` to printing the absolute path → Steps 5–7 red. Record each mutation and the test it reddened
- [x] Step 13: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/... ./internal/scaffold/...`, `golangci-lint run ./...` → mark SCENARIO-10 done in specification.md; rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `scaffold.NewFeature`/`NewStep` return a result struct carrying feature, step id, `Path`,
  `Created` — the step id exists only inside scaffold (`pattern.ID(next)`); cli must never
  recompile `step-file-pattern` to recover it.
- `created[]` = files this invocation brought into existence, in write order: new feature →
  `[spec, state]` (directory itself excluded — `path` names it); new step → `[step file]`
  only. The specification's appended progress entry is a modification and is **not** in
  `created`. There is no `modified` field — the product-vision shape is fixed.
- JSON `path` is always the absolute form of the single line text-mode stdout prints (feature
  dir for `new feature`, step file for `new step`).
- `step` is `*string`: `null` for `new feature`, the id for `new step`.
- Exact stderr copy (text mode only, one line, after stdout):
  `brief new feature: created <name> (<spec rel>, <state rel>); add a step with 'brief new step <name>'`
  and
  `brief new step: created <id> in <feature>; fill in its acceptance criteria and checklist, then 'brief start <feature>'`.
  S14's help text must not describe different copy.
- Text-mode stdout/stderr paths go through `displayPath(wd, p)`; `new.go` holds no private
  `filepath.Rel` copy.
- `--json` branch precedes every success-path stdout/stderr write (R1), as in status/check.

**Left unbuilt** — named so nobody assumes it exists:
- `brief new --json` (bare `new`, no type) success document — it only ever errors; nothing
  to build.
- `--json` flag row in `new feature`/`new step` help — S14.
- `finish`'s success stderr and JSON document — S11.

**Traps** — things that look right and are not:
- `filesChangedFor` lists `"new feature"`/`"new step"` and returns `false` — correct for the
  refusal document only. The success shape has **no** `files_changed` key; wiring it in
  would claim nothing changed on a run that wrote files.
- `new step`'s golden cannot pin the id from a production literal; capture it from the
  created file's name.
- On macOS `t.TempDir()` sits under a symlinked `/var`; build expected absolute paths from
  the same `wd` passed to `cli.Run`, never from `filepath.EvalSymlinks`.
- Existing `assert.Empty(t, stderr.String())` on successful `new` runs elsewhere in
  `run_test.go` (e.g. the ancestor-config test) turn red once stderr is written — rewrite
  them to the ruled line, do not delete the assertion.
