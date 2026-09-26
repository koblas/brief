---
id: SCENARIO-04
status: done
---

# SCENARIO-04: CLI copy teaches the loop

## Scenario

```gherkin
Scenario: SCENARIO-04 CLI copy teaches the loop
  Given the ruled copy
  Then "brief new feature"'s success line names writing the specification before adding steps
  And root "--help" carries the feature-lifecycle paragraph
  And "brief start --help" and "brief finish --help" carry the amended sentences
  And docs/specifications/brief/specification.md records the R11 and new-step amendments
```

This scenario is a pure copy change — no new decision surface, every string is ruled
verbatim in the spec's `## Surface & Copy`. CLI-only; no HTTP surface exists, so
`api-conventions` does not apply.

User-visible contract (verbatim from the spec):

- `brief new feature`'s stderr success line tail changes from `; add a step with 'brief
  new step <name>'` to `; write its specification, then add each step with 'brief new
  step <name>'`. `brief new step`'s own success line is unchanged.
- Root `--help`'s first line (`brief manages feature specifications as files in your
  repository.`) gains a second paragraph, verbatim:
  ```
  A feature is a specification, ordered step files and one state file. Open one
  with 'brief new feature', write its specification, add steps with 'brief new
  step', then take each step from 'brief start' to 'brief finish'.
  ```
  Checked against `internal/cli/help_json.go`: `helpIndex` walks only `root.Commands()`
  and never emits an entry for root itself (`Test_help_json_spellings_produce_identical_documents`'s
  "full index" case pins the exact name list, which excludes root). So this paragraph is
  text-only by construction — no `--json` behavior to touch, no new JSON test to write.
- `startLong`: replace the sentence "A missing optional convention — the step's
  acceptance heading, or a state file heading — is named on stderr instead" with "A
  shortfall — the step's acceptance heading missing or empty, a checklist with no items
  yet, or a state file heading missing — is named on stderr instead"; the rest of that
  sentence (", one line each, and the brief still prints on stdout, still exiting 0.")
  is unchanged.
- `finishLong`: append, as a new sentence continuing its first paragraph (no blank line
  before it — the same in-paragraph pattern `startLong`'s own parallel sentence already
  uses), immediately after "...for at most one of --handoff and --state.": `brief finish
  refuses, changing no files, while the step's checklist heading is missing, has no
  items, or has an item not ticked.` This sentence still sits before the existing
  drop-reporting paragraph and the JSON-fields paragraph.
- Both amended sentences must be hand-wrapped to the file's existing prose width so
  `Test_every_leaf_help_line_fits_in_80_columns` (already in `internal/cli/help_test.go`,
  covers `start` and `finish`, root is exempt) stays green.

Repo-wide grep for the old strings (`A missing optional convention`, `manages feature
specifications as files`, `add a step with`) turns up only `internal/cli` production and
test files, plus two files intentionally out of scope (see Handoff).

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/run_test.go` `Test_creates_the_feature_and_prints_its_path` — update the expected stderr string's tail to the ruled copy; fails against the unchanged production tail.
- [x] Step 2: `internal/cli/run_test.go` `Test_creates_the_feature_where_an_ancestor_config_directs` — same tail update; fails the same way.
- [x] Step 3: `internal/cli/help_test.go` `rootHelp` — insert the ruled lifecycle paragraph after the first line; `Test_prints_the_root_help_with_one_line_per_command` fails: root's rendered `Long` lacks the new paragraph.
- [x] Step 4: `internal/cli/help_test.go` `startHelp` — replace the shortfall sentence; `Test_prints_start_help_as_usage_line_prose_and_flag_table` fails: `startLong` still carries the old wording.
- [x] Step 5: `internal/cli/help_test.go` `finishHelp` — insert the appended refusal sentence; `Test_prints_finish_flag_prose_in_its_flag_table` fails: `finishLong` has no such sentence yet.
- [x] Step 6: `internal/cli/help_json_test.go` `Test_help_finish_json_is_the_exact_document`'s `description` literal — insert the same appended sentence in the same position; fails: the JSON `description` field (built from `cmd.Long` via `helpEntry`) still lacks it.

### Green

- [x] Step 7: `internal/cli/new.go` `runNewFeature` — change the stderr `Fprintf` tail to the ruled copy.
- [x] Step 8: `internal/cli/cli.go` — add a `rootLifecycleParagraph` const (one-line doc) and join it into root's `Long` in `newRootCommand` (`rootShort + "\n\n" + rootLifecycleParagraph`), leaving `rootShort` itself, and its "one-sentence" doc comment, untouched.
- [x] Step 9: `internal/cli/start.go` `startLong` — replace the shortfall sentence.
- [x] Step 10: `internal/cli/finish.go` `finishLong` — append the refusal sentence to the first paragraph.

### Sweep

- [x] Step 11: `docs/specifications/brief/specification.md` R11 — add "Finishing an open step whose checklist heading is absent or holds no items is refused." right after "Finishing a step with open checklist items is refused, naming the first." (sweep)
- [x] Step 12: `docs/specifications/brief/specification.md` line 219, the `new step` command-table row — say the scaffold carries an empty acceptance section as well as an empty checklist (sweep)
- [x] Step 13: `docs/specifications/brief/specification.md` line 393, the approved scenario ("Then the step file has...") — say the same (sweep)
- [x] Step 14: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

Not re-planned: `internal/scaffold/finish.go`'s doc-sentence reversal (the other half of Rule
7) already landed in SCENARIO-03 (`finish.go:314`, "step with no checklist items, or no
checklist heading at all, is refused").

### Verify

- [x] Step 15: full verification per `.claude/rules/agent-briefs.md` from start commit
  `db70338`. Expected `test-stats` delta: 0 top-level tests (every Red step edits an
  existing golden literal; none add a test). No behavior mutation to verify — every
  changed string is pinned byte-exact by a golden assertion, so the Red/Green pairing
  itself is the proof. Instead, mutation-verify the one structural claim this scenario
  relies on rather than changes: temporarily add root's own entry to `helpIndex`'s walk
  in `internal/cli/help_json.go` (before or alongside `walk(root.Commands())`) →
  `Test_help_json_spellings_produce_identical_documents`'s "full index" case goes red
  (an unexpected name appears ahead of `"new"`). Confirms the root paragraph really is
  invisible to `--json` today, not merely assumed so. Restore after observing red.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The lifecycle paragraph lives only in a new `rootLifecycleParagraph` const joined into
  root's `Long`; `rootShort` keeps its original one-sentence value and doc comment.
  `helpIndex` (`internal/cli/help_json.go`) never emits an entry for root itself, so this
  paragraph never reaches `--json` — SCENARIO-05/06 must not duplicate it into any JSON
  copy path, and no future change to `helpIndex` may start walking root itself without
  re-checking this.
- `finishLong`'s new refusal sentence continues its first paragraph (no blank line before
  it), the same in-paragraph placement `startLong` already uses for its own parallel
  sentence — not a new stand-alone paragraph, and not placed after the drop-reporting
  paragraph.
- `brief new feature`'s stderr tail is "; write its specification, then add each step
  with 'brief new step <name>'"; `brief new step`'s own success line is unchanged (spec
  is explicit on this point).

**Left unbuilt** — named so nobody assumes it exists:
- `.claude/skills/brief-workflow/SKILL.md`'s Lifecycle section — SCENARIO-05.
- The CLAUDE.md snippet's "Multi-step work gets a feature" sentence
  (`internal/platform/artifact/snippet.go`) — SCENARIO-06.

**Traps** — things that look right and are not:
- `cmd/brief/main.go`'s package doc comment ("Command brief manages feature
  specifications as files in your ... repository.") echoes `rootShort`'s wording but is
  a separate Go doc comment, not part of the ruled CLI copy — left untouched.
- `docs/specifications/human-output/SCENARIO-10.md` quotes the *old* `new feature`
  success-line format twice, as the historical record of an already-shipped scenario in a
  different feature. It is out of Rule 7's scope (which names only
  `docs/specifications/brief/specification.md`) and must not be edited here.
- `internal/cli/help_json_test.go`'s generic cross-check
  (`Test_help_json_entries_agree_with_each_commands_text_help`) only asserts
  `Description` is non-empty, so it will not catch a mismatched `finishLong` — only
  `Test_help_finish_json_is_the_exact_document`'s literal `description` string does.
