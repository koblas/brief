# SCENARIO-08: check groups findings by feature and names each rule

## Scenario

```gherkin
Scenario: SCENARIO-08 check groups findings by feature and names each rule
  When I run "brief check"
  Then findings are grouped under a feature header, paths are relative, a whole-file finding shows no ":0"
  And each finding carries a stable rule id
  And the stderr summary counts ERROR/WARN per rule and names the next action
```

## User-visible contract

Command: `brief check [feature]`. Text mode only (the `--json` document is S09; until then
`--json` still runs this text path).

stdout, one group per feature that has findings, in Check's emission order, one blank line
between groups (none before the first, none after the last):

```
<name>  (in flight)
  ERROR  <rel path>:<line>  <detail>
  ERROR  <rel path>  <detail>            <- Line == 0: no ":0"

<name>  (complete)
  WARN  <rel path>:<line>  <detail>
```

stderr, one line:
`brief check: N ERROR, M WARN in K features (<count> <rule>, <count> <rule>, …)` then the
tail clauses below joined as `; <c1> — <c2>`:
- `ERRORs block finish on in-flight features` — only when N > 0.
- `'brief check <feature>' narrows to one` — only when no feature argument was given AND K > 1.
- neither applies → no `;` tail at all.

Zero findings: unchanged `brief check: no findings` on stderr, empty stdout, exit 0.
Exit codes unchanged: 1 on any ERROR (`errCheckFindings`), 0 on WARN-only or none, 2 usage,
1 refusal (unknown feature via `enrichUnknownFeature`).

## Implementation Plan

Rule ids and feature stamping (internal/assemble)

- [x] Step 1: `internal/assemble/check_test.go` `Test_check_assigns_each_producer_its_stable_rule_id` — table over one fixture per producer (spec missing, spec unreadable, spec fence, spec heading, state missing, state unreadable, state-cap, state fence, state heading, steps unlistable, step unreadable, frontmatter, checklist, depends-on self, depends-on dangling, handoff-cap, feature symlink, feature unreadable) asserting the exact `Rule` of the finding (red)
- [x] Step 2: `internal/assemble/check.go` `Rule` type + `Rule*` constants beside `Severity`, `Finding.Rule` field, `Finding` doc updated (new)
- [x] Step 3: `internal/assemble/assemble.go` — extract an unexported spec-fault classifier returning (rule, `*RefusalError`) that `checkSpecification` wraps unchanged and `checkSpecFindings` uses for its rule (update)
- [x] Step 4: `internal/assemble/check.go` — set `Rule` at every `Finding` construction site in `checkStateFindings` (missing vs unreadable via `fs.ErrNotExist`), `checkStepFindings`, `checkStepChecklistFinding`, `checkStepDependencyFindings`, `checkHandoffCapFinding`, `symlinkFeatureFinding`, `unreadableFeatureFinding` (green)
- [x] Step 5: `internal/assemble/check_test.go` `Test_check_orders_findings_specification_then_state_then_steps_ascending` — also assert the parallel rule list (update)
- [x] Step 6: `internal/assemble/check_test.go` — four focused tests (ordinary in-flight, ordinary complete, symlinked, unreadable), each asserting `Feature`/`FeaturePath`/`InFlight` through both the all-features scan and the named path (green on arrival — Step 4's implementation stamped these fields directly inside `symlinkFeatureFinding`/`unreadableFeatureFinding` and `checkFeatureDir`'s severity loop rather than at the four call sites, per the architect Handoff's own trap warning; mutation-verified by dropping the stamp in `symlinkFeatureFinding` — reddened exactly `Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true`, file restored byte-identical)
- [x] Step 7: `internal/assemble/check.go` `Finding.Feature`, `Finding.FeaturePath`, `Finding.InFlight` — stamped in `checkFeatureDir`'s existing severity loop, and inside `symlinkFeatureFinding`/`unreadableFeatureFinding` themselves (not at their four call sites, which never modify a Feature-level Finding after construction) (green)

Grouping + rendering

- [x] Step 8: `internal/assemble/check_test.go` `Test_GroupByFeature_folds_findings_into_one_group_per_feature_in_order` — two features, order preserved, `Name`/`Path`/`InFlight` from the findings; empty input → empty result (red)
- [x] Step 9: `internal/assemble/check.go` `FeatureFindings{Name, Path, InFlight, Findings}` + `GroupByFeature` — linear fold keyed on `Feature`, first-appearance order (green)
- [x] Step 10: `internal/assemble/render_test.go` — replace the two `RenderFindings` tests: header `(in flight)`/`(complete)` from `InFlight`, two-space-indented rows, `:<line>` only when `Line > 0`, one blank line between groups, empty slice writes nothing, a tab/newline in detail is flattened (red)
- [x] Step 11: `internal/assemble/render.go` `RenderFindings(w, []FeatureFindings)` — new grouped shape; renders `Path` verbatim (the caller relativizes) (green)

Command slice (internal/cli)

- [x] Step 12: `internal/cli/check_test.go` — rewrite `Test_check_prints_findings_on_stdout_and_exits_1_for_an_error_finding` to the byte-exact grouped stdout with a relative path and the exact stderr summary; add `Test_check_groups_two_features_with_a_blank_line_and_counts_rules_by_count_then_id` (K=2, narrowing clause present, rule order pinned incl. a count tie), `Test_check_omits_the_line_suffix_for_a_whole_file_finding`, `Test_check_summary_drops_the_ERRORs_clause_when_every_finding_is_WARN` (rewrites the WARN-severity test's stderr, exit 0), `Test_check_summary_drops_the_narrowing_clause_for_a_named_feature`, `Test_check_labels_a_symlinked_feature_directory_under_its_own_name` (red)
- [x] Step 13: `internal/cli/check.go` `runCheck` — group via `assemble.GroupByFeature`, relativize a **copy** of each finding's `Path` through `displayPath(wd, …)`, render, then write `checkSummary` (green)
- [x] Step 14: `internal/cli/check.go` `checkSummary(groups, featureArg)` — counts, rule tally sorted by count desc then rule id asc (never map iteration order), `feature`/`features` pluralized on K, conditional clauses (green)
- [x] Step 15: `internal/cli/check.go` `checkLong` — replace the `[SEVERITY] <path>:<line> — <problem>` paragraph with the grouped shape, the whole-file rule, and the summary line (update)
- [x] Step 16: `internal/cli/check_drift_test.go` — no change: it compares `Detail` via direct Server calls; confirmed green untouched (verify only)

Verification

- [x] Step 17: mutation-verified individually (cp-to-$TMPDIR per `.claude/rules/agent-briefs.md`, each restored byte-identical after): (a) dropped `Rule: RuleHandoffCap` in `checkHandoffCapFinding` → reddened exactly `Test_check_assigns_each_producer_its_stable_rule_id/handoff_cap`; (b) dropped `Feature`/`FeaturePath`/`InFlight` in `symlinkFeatureFinding` → reddened exactly `Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true`; (c) sorted the rule tally by id only → reddened `Test_check_groups_two_features_with_a_blank_line_and_counts_rules_by_count_then_id` (redesigned with a 2/1/1 count split so id-only sort actually diverges from count-desc-then-id — a pure 1/1 tie does not discriminate this mutation); (d) always emitted the ERRORs clause → reddened `Test_check_summary_drops_the_ERRORs_clause_when_every_finding_is_WARN` (plus the same WARN-only two-feature test as expected collateral); (e) always emitted `:%d` regardless of Line → reddened both `Test_RenderFindings_writes_a_group_header_and_its_indented_findings`/`_separates_groups_with_exactly_one_blank_line` and the cli `Test_check_omits_the_line_suffix_for_a_whole_file_finding`; (f) dropped the blank-line-between-groups branch → reddened `Test_RenderFindings_separates_groups_with_exactly_one_blank_line` and the cli two-feature test
- [x] Step 18: `go build ./...` clean; `go test -count=1 ./...` all 8 packages ok, 0 fail, 0 skip; `go test -count=1 -v ./internal/assemble/... ./internal/cli/...` 649 PASS / 0 FAIL / 0 SKIP (+18 net top-level test funcs in this scenario's three touched test files — 18 table rows collapsed into named case functions plus new stamping/grouping/render/summary tests, minus 4 removed superseded tests, per `go test` output and `git diff --stat`); `go test -race ./internal/assemble/... ./internal/cli/...` both ok; `golangci-lint run ./...` 0 issues (fixed a `maintidx` hit and 3 `thelper` hits by hoisting the rule-id table's 18 closures into named top-level `ruleCase*` functions, each starting `t.Helper()`); `go run ./cmd/brief check` on this repo: 4 groups (brief, cli-cobra, human-output, version-flag), every path relative, zero `:0` occurrences (grepped), exit 1
- [ ] Step 19: all tests green → mark SCENARIO-08 done in specification.md; rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Rule ids (the const values are S09's wire `rule` strings, R8 — never rename):
  `feature-symlink`, `feature-unreadable`, `spec-missing`, `spec-unreadable`,
  `state-missing`, `state-unreadable`, `steps-unlistable`, `step-unreadable`, `frontmatter`,
  `fence`, `heading`, `state-cap`, `handoff-cap`, `checklist`, `depends-on`.
  `fence` and `heading` each cover spec and state; `depends-on` covers self and dangling —
  deliberate (R8 lists each once; the path disambiguates).
- `assemble.Check` keeps `([]Finding, error)`; grouping is `assemble.GroupByFeature`, which
  S09 reuses for `features[]` — ~30 assemble tests and the drift tests depend on the signature.
- `Finding` carries `Rule`, `Feature` (dir name), `FeaturePath` (absolute feature dir),
  `InFlight`. The group header label comes from `InFlight`, **never from severity**: the two
  feature-level producers are hard-coded ERROR regardless of doneness.
- Paths stay absolute in `assemble`; `runCheck` relativizes a copy via `displayPath` (R6).
  `RenderFindings` renders `Path` verbatim.
- Summary: ERROR/WARN never pluralized; `feature(s)` pluralized on K = groups with findings;
  rule tally ordered count desc, then rule id asc.
- Narrowings of product-vision's ruled copy (for the final product-vision pass): the
  `ERRORs block finish…` clause only when N > 0 (false on a WARN-only exit-0 run); the
  `'brief check <feature>' narrows to one` clause only with no feature argument and K > 1.
- Text rows do not show the rule id (ruled row copy has none); the rule is in the summary
  tally and, from S09, in JSON.

**Left unbuilt** — named so nobody assumes it exists:
- `checkDocument` / `check --json` payload (`counts`, `features[]{name,path,in_flight,findings[]}`) — S09.
- "For scripts, use --json; the text layout may change." in check help — S14.

**Traps** — things that look right and are not:
- `symlinkFeatureFinding`/`unreadableFeatureFinding` bypass `checkFeatureDir`'s stamping
  loop; a new feature-level producer must be stamped at its call site or its group header
  prints an empty name. This repo's tree cannot catch it — only the fixture test does.
- `Rule` must be set at every construction site (unlike the stamped fields); an empty rule
  renders as `N ` in the tally.
- Ranging a `map[Rule]int` for the tally gives a flaky byte-pinned summary.
- S09 must build JSON from the un-relativized findings — the cli's relative copy is text-only.
- A zero-step feature with findings is `InFlight == false` → `(complete)` in check, while
  `FeatureStatus.Complete()` says not complete. Pre-existing Check semantics ("vacuously
  every step done"); not reconciled here.
- `flattenTabwriterField` (assemble) is the text flattener for `RenderFindings`; JSON must
  carry raw detail.
