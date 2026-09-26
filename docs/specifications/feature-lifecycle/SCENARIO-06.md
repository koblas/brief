---
id: SCENARIO-06
status: done
---

# SCENARIO-06: CLAUDE.md snippet names when to open a feature

## Scenario

Scenario: SCENARIO-06 CLAUDE.md snippet names when to open a feature
Given "brief init"
Then the CLAUDE.md block carries the ruled "Multi-step work gets a feature" sentence
And RecognizeSnippet recognizes it as current for any feature directory

Note (not a checklist item): only `internal/platform/artifact/snippet.go`'s
`snippetTemplateSuffix` const holds the fixed prose; `SnippetBlock`, `RecognizeSnippet`, and
every consumer (`internal/setup`, `internal/doctor`, `internal/cli`) call it live, so they
need no edit. `KindSnippet` is deliberately excluded from `Render`/`Recognize` and from
`shipped_digest_test.go`'s table (`digest.go`'s comment; confirmed no `KindSnippet` row
exists there) — unlike SCENARIO-05's skill, there is no shipped-digest row to move. Whole-repo
grep (including non-Go) for `brief:begin`, `Features under`, `To work on a step, run`,
`SnippetBlock`, `RecognizeSnippet` found every call site listed below; no test or doc outside
`snippet.go` itself hardcodes the fixed prose. `docs/specifications/init-doctor/
specification.md:79` quotes the old sentence as a worked example in an unrelated, already-
shipped feature's spec — historical, out of scope, same pattern as SCENARIO-04/05's notes on
`human-output/SCENARIO-10.md` and the two skill spec docs. Because the ruled sentence adds two
lines (8 lines to 10) and grep for prose strings cannot see a hardcoded line-number assertion
(a `MarkerProblem.Line`, a marker span's `BeginLine`, a "second begin" position), a probe
patched `snippetTemplateSuffix` to the ruled bytes at a `git archive 5a16a59` export and ran
`go test ./...` unpiped from there: exit 0, every package green, including
`internal/platform/artifact` (`ScanSnippetMarkers`/span-line tests), `internal/doctor`, and
`internal/setup`. No test anywhere pins a line number derived from the snippet block's length.

## Implementation Plan

### Red

- [x] Step 1: `internal/platform/artifact/snippet_test.go` `Test_snippet_block_names_when_to_open_a_feature` — assert `artifact.SnippetBlock("docs/specifications")` equals the exact ruled block bytes from the spec's `## Surface & Copy` ("CLAUDE.md snippet"), transcribed in the test with `docs/specifications` substituted for `{dir}`; fails: current bytes lack the "Multi-step work gets a feature: run `brief new feature <name>`…" sentence

### Green

- [x] Step 2: `internal/platform/artifact/snippet.go` `snippetTemplateSuffix` — rewrite the const to render exactly Step 1's transcription: the ruled sentence "Multi-step work gets a feature: run `brief new feature <name>`, write its specification, then add each step with `brief new step <feature>`." inserted between "are tracked by `brief`. " and "To work on a step, run", wrapped across three lines as ruled (the first prose line now ends "…gets a feature: run"), preserving the existing prefix/suffix split around `{dir}` and every other sentence verbatim

### Sweep

- [x] Step 3: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 4: full verification per `.claude/rules/agent-briefs.md` — the one `go test -count=1 -coverpkg=./... ./...` run covers `internal/setup`, `internal/doctor`, and `internal/cli`'s snippet-consuming tests, all of which call `artifact.SnippetBlock`/`RecognizeSnippet` live and need no separate re-run; mutation guard: copy `snippet.go` aside, then replace one of the new sentence's internal `\n` line breaks with a single space (joining two of its wrapped lines into one, leaving every other line and the sentence's own words untouched), confirm `Test_snippet_block_names_when_to_open_a_feature` reddens while `Test_snippet_block_names_the_feature_directory` stays green — proving the new test is the one pinning the ruled wrapping, not the pre-existing prefix/contains tests — then restore and diff to prove byte-identical; separately confirm (no mutation needed — already exercises this) that `Test_recognize_snippet_reports_current_for_its_own_dir` and `Test_recognize_snippet_reports_current_for_a_different_dir` still pass against the new bytes for two different dirs, since neither hardcodes the old prose

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `snippetTemplateSuffix` (`internal/platform/artifact/snippet.go`) carries the ruled "Multi-step work gets a feature" sentence, inserted before the existing "To work on a step, run" sentence; `snippetTemplatePrefix` and the `{dir}` split point are unchanged. This is the feature's last content change — no further edit to the snippet's fixed prose is expected without a new scenario.
- No older-template entry added: `olderSnippetTemplates` stays `[]func(dir string) []byte{}` (empty slice) per Rule 8 and the user ruling recorded in STATE.md — no install has shipped the old snippet bytes, so they are simply no longer recognized as brief's; a repo that had it now reads that block as `OriginEdited` on next `doctor`/`init`, which is the accepted outcome, not a defect to fix.
- `KindSnippet` stays excluded from `Render`/`Recognize`/`shipped_digest_test.go` — `SnippetBlock`/`RecognizeSnippet` remain the snippet's only render/recognize pair, confirmed unchanged by this scenario.

**Left unbuilt** — nothing new; this is the feature's last scenario. Once this plan is implemented and ticked, all six scenarios in `docs/specifications/feature-lifecycle/specification.md`'s `## BDD Acceptance Progress` are done and STATE.md's "Left unbuilt" section (currently naming only this sentence) goes empty.

**Traps** — things that look right and are not:
- Do not add a `KindSnippet` row to `shipped_digest_test.go` — that table only covers `Kind`s `Render`/`Recognize` handle; the snippet's own digest story is `RecognizeSnippet`'s template-replay match, not a compiled sha256 list.
- `docs/specifications/init-doctor/specification.md:79` quotes the pre-change sentence as an unrelated feature's worked example — leave it; it is historical prose, not a pinned test fixture.
- Every consumer test (`internal/setup/snippet_test.go`, `internal/setup/snippet_internal_test.go`, `internal/setup/print_test.go`, `internal/doctor/host_test.go`, `internal/doctor/host_disk_test.go`, `internal/doctor/doctor_test.go`, `internal/cli/init_internal_test.go`, `internal/cli/doctor_internal_test.go`, `internal/cli/uninstall_internal_test.go`, `internal/platform/artifact/snippet_scan_test.go`) builds its expected block by calling `artifact.SnippetBlock(dir)` live — none hardcodes the fixed prose, so none needs a byte-level edit for this scenario. The git-archive probe against `5a16a59` (`go test ./...`, exit 0, all packages green) confirms none of them hardcodes a line number derived from the block's length either.
