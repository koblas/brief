---
id: SCENARIO-07
status: done
---

# SCENARIO-07: status --json

## Scenario

```gherkin
Scenario: SCENARIO-07 status --json
  When I run "brief status --json"
  Then the document has features[] with name, absolute path, done, total, blocked, complete, next {id,title,path} or null, problem or null
  And malformed features have null counts and a problem object, and the exit code is 0
```

## User-visible contract

- Command: `brief status --json` (`--json` anywhere before `--`, already stripped by `scanJSONFlag`).
- stdout: exactly one compact document + `\n`, via `writeJSONDocument`:
  `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[...]}`. Each row is, in this key
  order: `name`, `path` (absolute feature dir), `done`, `total`, `blocked` (integers, or `null` all three
  on a malformed row), `complete` (`(FeatureStatus).Complete()`), `next` (`{"id","title","path"}` or
  `null`), `problem` (`{"path","line","detail","fix"}` or `null`). Rows are in the same order as the text table
  (`Status` relies on `fs.ReadDir`'s sorted-by-filename order).
- No features (root absent or empty): `"features":[]` (never `null`), exit 0.
- stderr: empty on every success path, including malformed rows and no features (R1). The text-mode
  malformed lines, summary line and "no features found" notice are not written in JSON mode.
- Exit 0 whenever `Status` succeeds, malformed rows included (R4). Refusals (invalid config) and usage
  errors already render as error documents (S01/S02) — unchanged, no new work.
- Paths absolute (R6): `path`, `next.path`, `problem.path` are the raw `assemble` values, never
  `displayPath`. `next.title` and `problem.detail`/`fix` are raw — no `flattenTabwriterField` /
  `flattenOneLine`.
- `problem.line` is always `null` in this scenario: `assemble.Problem` has no `Line` field and none of
  `newProblem`'s three sources (`*fs.PathError`, `*stepFrontmatterError`, generic) carries a line number.
- Text mode (`brief status` without `--json`) is byte-unchanged.

## Implementation Plan

Tests live in a new `internal/cli/status_json_test.go` (`package cli_test`), driven through `cli.Run`,
reusing `writeStatusStep`, `stepTitle`, `writeMalformedStatusFeature` (status_test.go) and `jsonString`
(json_refusal_test.go).

- [x] Step 1: `status_json_test.go` `Test_status_json_document_golden` — exact-bytes golden (`assert.Equal`, not `JSONEq`) for one fixture holding four features named so lexical order is the golden order — `alpha` in-progress (next non-null, one blocked step), `beta` complete (`next` null, `complete` true), `delta` malformed (counts null, `complete` false, `problem` object, `line` null), `epsilon` no-steps (a bare feature directory: `0/0/0`, `complete` false, `next` null); absolute paths built from `wd` + `jsonString`, the malformed `detail`/`fix` taken from a value captured from an independent text-mode `brief status` run on the same fixture (its stderr line), never a literal copied from `newProblem`; stderr empty, exit 0 (red)
- [x] Step 2: `internal/cli/status.go` — unexported JSON types beside `runStatus`: `statusDocument` (`jsonHeader` embedded first, then `Features []statusFeatureJSON` tagged `features`), `statusFeatureJSON` with `*int` `done`/`total`/`blocked`, `complete bool`, `*statusNextJSON`, `*statusProblemJSON` (whose `line` is `*int`); every field explicitly tagged, no `omitempty` (new)
- [x] Step 3: `internal/cli/status.go` `statusFeatures(rows)` — maps `[]assemble.FeatureStatus` to the row slice: non-nil empty slice for zero rows, counts nil iff `Problem != nil`, `complete` from `row.Complete()`, `next`/`problem` nil iff source nil, `problem.line` nil (green)
- [x] Step 4: `internal/cli/status.go` `runStatus` — after `srv.Status` succeeds, branch on `out.json` **before** the zero-rows notice: write `statusDocument{jsonHeader: out.successHeader(), ...}` via `writeJSONDocument`, wrap a write error as `brief status: %w`, return nil; text path below untouched (green)
- [x] Step 5: `status_json_test.go` `Test_status_json_with_no_features` — table over "feature root absent" / "feature root empty": exact bytes `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[]}` + `\n`, stderr empty, exit 0; control arm in the same test: text-mode run on the same wd writes the notice to stderr (proves the JSON-mode empty stderr is the branch, not an absent notice) (green on arrival via Step 3/4 — say so)
- [x] Step 6: `status_json_test.go` `Test_status_json_keeps_the_next_title_raw` — a step whose `# ` heading contains an interior tab (`markdown.Title` only `TrimSpace`s the ends, so the tab survives): decoded `next.title` equals the raw heading, while the text-mode table row for the same fixture shows it flattened (control arm differs only in `--json`) (green on arrival once Step 4 lands — say so)
- [x] Step 7: `status_json_test.go` `Test_status_json_counts_are_null_only_on_a_malformed_row` — decode into `map[string]json.RawMessage` per row: malformed row's `done`/`total`/`blocked` are literally `null` and its `path` is still the absolute feature dir; a well-formed zero-step row's are literally `0` (control: same key, one variable — malformed vs not) (green on arrival — say so)
- [x] Step 8: `json_refusal_test.go` — delete `Test_json_mode_status_problem_stays_text_until_its_payload_lands` (the S02 interim pin); replace with `Test_json_mode_status_with_a_malformed_feature_is_a_success_document`: `status --json` on `writeMalformedStatusFeature` → decodes as a document with `ok` true, `exit_code` 0, no `error` key, stderr empty; control arm: same wd without `--json` writes the malformed line + summary to stderr. Verified at planning time: no other test pins status-under-`--json` to the text path (the only other `status`+`--json` tests are the usage-error rows in `json_usage_test.go` and the invalid-config refusal row at `json_refusal_test.go:207`, both unaffected) (update)
- [x] Step 9: mutation verification, one at a time, stashed per `.claude/rules/agent-briefs.md`, naming the test each reddens:
  (a) `complete` computed as `row.Next == nil` → Step 1 golden (no-steps row) red — confirmed;
  (b) counts emitted as non-nil pointers on a malformed row → Steps 1 and 7 red — confirmed;
  (c) `statusFeatures` returns a nil slice for zero rows → Step 5 red — confirmed;
  (d) JSON branch placed after the zero-rows notice / summary write → Steps 5 and 8 stderr assertions red — confirmed;
  (e) `statusFeatures` replaces the tab in `next.title` (`strings.ReplaceAll(title, "\t", " ")` — `flattenTabwriterField` is unexported in `assemble` and unreachable from `cli`) → Step 6 red — confirmed;
  (f) `path` passed through `displayPath` → Step 1 red — confirmed
- [x] Step 10: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-07 done in specification.md; rewrite STATE.md (drop the "status --json document" / `Problem.Line` / "known list via status --json" Left-unbuilt entries, record the decisions below)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The status JSON types (`statusDocument`, `statusFeatureJSON`, `statusNextJSON`, `statusProblemJSON`)
  live in `internal/cli/status.go`, not `assemble` — same placement as `startDocument`; S09's
  `checkDocument` should follow it so `assemble` stays free of the header.
- Malformed rows' `done`/`total`/`blocked` are `*int` nil in the JSON row type; `assemble.FeatureStatus`
  keeps plain ints (zero on a Problem row). The nulling happens only in the cli mapping — do not add
  pointers to `FeatureStatus`, the text table and `statusSummary` depend on its current shape.
- `complete` = `(FeatureStatus).Complete()`; a zero-step feature is `complete:false` with `next:null`.
  Scripts distinguish "no steps" from "complete" by `total == 0`.
- `features` is `[]` (never `null`) on zero rows; that is the JSON form of R9's empty discriminator.
- `problem.line` is always `null`; the key is present so adding `assemble.Problem.Line` later is additive
  (no schema bump).
- JSON mode on status writes zero stderr bytes on success, including the no-features case.
- JSON strings (`next.title`, `problem.detail`, `problem.fix`) are raw — flattening is a text-only concern.

**Left unbuilt** — named so nobody assumes it exists:
- `assemble.Problem.Line` — unowned; no Problem source yields a line today.
- status help sentence "For scripts, use --json; the text layout may change." and `statusLong`'s
  text-only wording ("one line on stderr saying so") — S14.
- structured `known` array in the JSON error object — S14/unowned; `status --json`'s `features[].name`
  is now the machine-readable known list.
- `assemble.RenderJSON` removal — still unowned.

**Traps** — things that look right and are not:
- Branching on `out.json` after the `len(rows) == 0` check leaks the text notice onto stderr in JSON mode.
- `var rows []statusFeatureJSON` encodes as `null`; use a sized `make`.
- The golden's malformed `detail` is a YAML/parser message — capture it from a text run, never paste it,
  or the golden pins a copy of production.
- `known:` (unknown-feature errors) and `status --json`'s `features` can disagree for a symlink /
  unopenable directory (pre-existing, see STATE.md) — do not reconcile them here.
- `runStatus` reads `out.json`, not a `jsonOut` parameter (only `start` takes one, for its help row).
