// finish's dropped-entry rows (SCENARIO-01 of dropped-entries): the text
// branch's stdout WARN rows and its stderr "(dropped N ...)" suffix,
// against rwfs.Mem the same way finish_internal_test.go's own _mem tests
// do, plus a white-box table for the unexported excerpt helper.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMemFinishFixtureWithState mirrors newMemFinishFixture, parameterized
// additionally on the old STATE.md body: every dropped-entry _mem test
// needs its own state shape, unlike newMemFinishFixture's fixed prose.
func newMemFinishFixtureWithState(checklistItem, state string) *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		checklistItem + "\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	return tree
}

// memOldStateWithDroppedEntry returns an old STATE.md body whose "## Traps"
// section holds one entry the new body keeps ("- kept entry") and one it
// drops ("- X (SCENARIO-02)") at whole-body line 17 — the same fixture
// shape SCENARIO-01's own Gherkin scenario names, reached by padding with
// prose filler lines that contribute no entry of their own.
func memOldStateWithDroppedEntry() string {
	lines := []string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept entry", "",
		"filler one", "",
		"filler two", "",
		"filler three", "",
		"filler four", "",
		"- X (SCENARIO-02)", "",
		"## Open debts", "",
	}

	return strings.Join(lines, "\n") + "\n"
}

func Test_finish_prints_a_warn_row_for_a_dropped_entry_mem(t *testing.T) {
	tree := newMemFinishFixtureWithState("- [x] do the thing", memOldStateWithDroppedEntry())
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)\n", stdout)
	assert.Contains(t, stderr, "(dropped 1 entry, listed on stdout)")
}

func Test_finish_pluralises_the_drop_count_and_renders_untagged_rows_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step01 := "---\nid: SCENARIO-01\nstatus: open\ndepends-on: []\n---\n\n" +
		"# SCENARIO-01 Demo step\n\n## Implementation Plan\n\n- [x] do the thing\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step01)
	step02 := "---\nid: SCENARIO-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# SCENARIO-02 Demo step\n\n## Implementation Plan\n\n- [x] do the thing\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-02.md"), step02)

	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"- TAGGED (TAGX)\n- UNTAGGED ENTRY\n\n## Open debts\n\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), oldState)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n- [ ] SCENARIO-02\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	wantStdout := fmt.Sprintf(
		"WARN  %s:7  dropped from Traps, tagged TAGX: TAGGED (TAGX)\nWARN  %s:8  dropped from Traps, untagged: UNTAGGED ENTRY\n",
		stateRel, stateRel)
	assert.Equal(t, wantStdout, stdout)
	wantStderr := fmt.Sprintf(
		"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s (dropped 2 entries, listed on stdout), ticked %s; next: SCENARIO-02 — run 'brief start demo'\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"),
		stateRel,
		filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Equal(t, wantStderr, stderr)
}

func Test_finish_complete_line_carries_the_drop_suffix_mem(t *testing.T) {
	oldState := "## Binding decisions\n\n- KEEP\n\n## Left unbuilt\n\n## Traps\n\n- DROP ME\n\n## Open debts\n\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n- KEEP\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	statePath := memWriteInput(tree, "state.md", newState)

	_, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	wantStderr := fmt.Sprintf(
		"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s (dropped 1 entry, listed on stdout), ticked %s; demo is complete\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"),
		stateRel,
		filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Equal(t, wantStderr, stderr)
}

// Test_finish_reports_an_open_debts_drop_as_dropped_debt_mem proves, through
// the CLI, that a drop under "## Open debts" renders its heading and its
// untagged branch correctly, against the em-dash "unowned — dies unless
// re-opened" text. Text mode never renders Rule, so dropped-debt itself
// stays proven only at Server level
// (internal/scaffold/finish_dropped_test.go
// Test_finish_classifies_an_open_debts_drop_as_dropped_debt) until
// SCENARIO-04 wires the JSON rule field.
func Test_finish_reports_an_open_debts_drop_as_dropped_debt_mem(t *testing.T) {
	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n\n" +
		"- kept entry\n\n- D — unowned — dies unless re-opened\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n\n- kept entry\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":11  dropped from Open debts, untagged: D — unowned — dies unless re-opened\n", stdout)
	assert.Contains(t, stderr, "(dropped 1 entry, listed on stdout)")
}

// Test_finish_reports_no_drops_when_the_new_body_only_reflows_whitespace_mem
// proves SCENARIO-03: a doubled interior space and a tab standing in for a
// space are whitespace normalizeEntryText already collapses generically
// (built by SCENARIO-01), never a drop. The trailing-space entry is
// included for delta coverage, not as proof — entryItemText's trimEOL
// already strips it before normalizeEntryText ever runs. The old body
// carries one entry each under Binding decisions, Traps and Open debts,
// wording and token order unchanged between old and new — only the
// whitespace reflows.
func Test_finish_reports_no_drops_when_the_new_body_only_reflows_whitespace_mem(t *testing.T) {
	oldLines := []string{
		"## Binding decisions", "",
		"- foo  bar (TAG1)", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- foo\tbar (TAG2)", "",
		"## Open debts", "",
		"- foo bar (TAG3) ", "",
	}
	oldState := strings.Join(oldLines, "\n") + "\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newLines := []string{
		"## Binding decisions", "",
		"- foo bar (TAG1)", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- foo bar (TAG2)", "",
		"## Open debts", "",
		"- foo bar (TAG3)", "",
	}
	newState := strings.Join(newLines, "\n") + "\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, memWantFinishCompleteLine("demo", "SCENARIO-01"), stderr)
}

// Test_finish_treats_moving_an_entry_between_state_headings_as_no_drop_but_moving_it_out_as_a_drop_mem
// proves SCENARIO-05: droppedEntries pools old and new entries across all
// four configured headings as one multiset keyed on normalized text alone
// (SCENARIO-01's binding decision) — an entry's heading is not part of its
// identity. The old body is identical in both cases: "## Traps" carries
// "- kept trap" then "- moved entry" at old-file line 9, the other three
// headings present and empty. Both new bodies carry all four configured
// headings (an omitted one would refuse with ErrMissingStateHeading,
// masking a true no-drop as an empty-stdout false positive) plus a
// trailing "## Notes" heading, and keep "- kept trap" under "## Traps" so
// the diff is never vacuous. When the new body carries "- moved entry"
// under "## Binding decisions" (a state heading), moving it is not a drop.
// When the new body carries it only under "## Notes" (not a configured
// heading), moving it out is a drop, reported against its *old* heading
// and *old* line — not "Notes".
func Test_finish_treats_moving_an_entry_between_state_headings_as_no_drop_but_moving_it_out_as_a_drop_mem(t *testing.T) {
	oldLines := []string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept trap", "",
		"- moved entry", "",
		"## Open debts", "",
	}
	oldState := strings.Join(oldLines, "\n") + "\n"

	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	handoffRel := filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")
	specRel := filepath.Join("docs", "specifications", "demo", "specification.md")

	newStateMovedBetweenStateHeadings := strings.Join([]string{
		"## Binding decisions", "",
		"- moved entry", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept trap", "",
		"## Open debts", "",
		"## Notes", "",
	}, "\n") + "\n"

	newStateMovedToNonStateHeading := strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept trap", "",
		"## Open debts", "",
		"## Notes", "",
		"- moved entry", "",
	}, "\n") + "\n"

	cases := []struct {
		name       string
		newState   string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "moved between two state headings is not a drop",
			newState:   newStateMovedBetweenStateHeadings,
			wantStdout: "",
			wantStderr: memWantFinishCompleteLine("demo", "SCENARIO-01"),
		},
		{
			name:       "moved to a non-state heading is a drop",
			newState:   newStateMovedToNonStateHeading,
			wantStdout: "WARN  " + stateRel + ":9  dropped from Traps, untagged: moved entry\n",
			wantStderr: fmt.Sprintf(
				"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s (dropped 1 entry, listed on stdout), ticked %s; demo is complete\n",
				handoffRel, stateRel, specRel),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
			handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
			statePath := memWriteInput(tree, "state.md", c.newState)

			stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

			require.NoError(t, err)
			assert.Equal(t, c.wantStdout, stdout)
			assert.Equal(t, c.wantStderr, stderr)
		})
	}
}

// Test_finish_reports_a_reworded_re_tagged_or_re_ticked_entry_as_dropped_mem
// proves SCENARIO-06: droppedEntries keys its pooled multiset diff on
// normalizeEntryText(e.Text) alone (SCENARIO-01's binding decision), and
// markdown.Entries' entryItemText strips only the "- "/"* "/"N. " marker,
// never a leading "[ ]"/"[x]" checkbox — so a reworded, re-tagged, or
// re-ticked entry already normalizes to a different string from its old
// occurrence and is reported as a drop carrying the *old* text/tag/line.
// The shared old body's "## Traps" section holds "- kept entry" at line 7
// then "- [x] Fix the thing (TAG1)" at line 9 — the same layout
// Test_finish_treats_moving_an_entry_between_state_headings_... uses — and
// only the new body's line 9 varies per case, so each drop case differs
// from the old body, and from the whitespace-reflow control, by exactly
// one variable. "kept entry" is unchanged in every new body so the diff is
// never vacuous.
func Test_finish_reports_a_reworded_re_tagged_or_re_ticked_entry_as_dropped_mem(t *testing.T) {
	oldLines := []string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept entry", "",
		"- [x] Fix the thing (TAG1)", "",
		"## Open debts", "",
	}
	oldState := strings.Join(oldLines, "\n") + "\n"

	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	handoffRel := filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")
	specRel := filepath.Join("docs", "specifications", "demo", "specification.md")

	wantDroppedStdout := "WARN  " + stateRel + ":9  dropped from Traps, tagged TAG1: [x] Fix the thing (TAG1)\n"
	wantDroppedStderr := fmt.Sprintf(
		"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s (dropped 1 entry, listed on stdout), ticked %s; demo is complete\n",
		handoffRel, stateRel, specRel)

	cases := []struct {
		name       string
		newLine9   string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "reworded",
			newLine9:   "- [x] Fix the other thing (TAG1)",
			wantStdout: wantDroppedStdout,
			wantStderr: wantDroppedStderr,
		},
		{
			name:       "re-tagged",
			newLine9:   "- [x] Fix the thing (TAG2)",
			wantStdout: wantDroppedStdout,
			wantStderr: wantDroppedStderr,
		},
		{
			name:       "re-ticked",
			newLine9:   "- [ ] Fix the thing (TAG1)",
			wantStdout: wantDroppedStdout,
			wantStderr: wantDroppedStderr,
		},
		{
			name:       "whitespace-only reflow is not a drop",
			newLine9:   "- [x] Fix  the   thing (TAG1)",
			wantStdout: "",
			wantStderr: memWantFinishCompleteLine("demo", "SCENARIO-01"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newLines := []string{
				"## Binding decisions", "",
				"## Left unbuilt", "",
				"## Traps", "",
				"- kept entry", "",
				c.newLine9, "",
				"## Open debts", "",
			}
			newState := strings.Join(newLines, "\n") + "\n"

			tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
			handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
			statePath := memWriteInput(tree, "state.md", newState)

			stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

			require.NoError(t, err)
			assert.Equal(t, c.wantStdout, stdout)
			assert.Equal(t, c.wantStderr, stderr)
		})
	}
}

// memOldStateWithMultilineTrapEntry returns an old STATE.md body whose
// "## Traps" section holds a multi-line entry — an unindented wrapped
// continuation line plus an indented sub-item, the sub-item's own marker
// kept (SCENARIO-07's folding rule) — at whole-body line 9, and whose "##
// Binding decisions" section holds a second, single-line entry kept
// unchanged, so a diff that ignored the new body entirely cannot pass by
// accident. The folded, normalized text is "wrap item text continued
// words - sub item text" (well under dropExcerpt's 80-rune cut).
func memOldStateWithMultilineTrapEntry() string {
	lines := []string{
		"## Binding decisions", "",
		"- kept entry", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- wrap item text",
		"continued words",
		"  - sub item text", "",
		"## Open debts", "",
	}

	return strings.Join(lines, "\n") + "\n"
}

// Test_finish_reports_no_drop_when_a_wrapped_entry_is_reflowed_to_one_line_mem
// proves SCENARIO-07: markdown.Entries folds a multi-line entry's
// continuation and indented sub-item into one Text, so a new body that
// reflows the same words onto a single physical line normalizes to the
// same identity as the old multi-line entry and is never reported as a
// drop.
func Test_finish_reports_no_drop_when_a_wrapped_entry_is_reflowed_to_one_line_mem(t *testing.T) {
	oldState := memOldStateWithMultilineTrapEntry()
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newLines := []string{
		"## Binding decisions", "",
		"- kept entry", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- wrap item text continued words - sub item text", "",
		"## Open debts", "",
	}
	newState := strings.Join(newLines, "\n") + "\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, memWantFinishCompleteLine("demo", "SCENARIO-01"), stderr)
}

// Test_finish_reports_a_dropped_multiline_entry_at_its_first_line_mem is
// Test_finish_reports_no_drop_when_a_wrapped_entry_is_reflowed_to_one_line_mem's
// control arm, differing in exactly one variable: the new body omits the
// multi-line entry entirely instead of reflowing it. The reported row's
// line is the entry's own first old-file line (D6, line 9) and its excerpt
// carries the fully folded text, continuation and sub-item words included.
func Test_finish_reports_a_dropped_multiline_entry_at_its_first_line_mem(t *testing.T) {
	oldState := memOldStateWithMultilineTrapEntry()
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n- kept entry\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":9  dropped from Traps, untagged: wrap item text continued words - sub item text\n", stdout)
	assert.Contains(t, stderr, "(dropped 1 entry, listed on stdout)")
}

// Test_finish_reports_a_duplicated_entry_dropped_once_at_its_later_occurrence_mem
// proves D2's "surplus old occurrences — the last ones in old-file order —
// are dropped": the old body carries the identical normalized text as two
// separate column-0 items under "## Traps", at old-file lines 7 and 9, and
// the new body carries it once, so exactly one of the two is a surplus.
// dropped.go's idxs[len(idxs)-surplus:] already selects the tail of the
// line-ordered group — this test is expected green on arrival.
func Test_finish_reports_a_duplicated_entry_dropped_once_at_its_later_occurrence_mem(t *testing.T) {
	oldLines := []string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- dup text", "",
		"- dup text", "",
		"## Open debts", "",
	}
	oldState := strings.Join(oldLines, "\n") + "\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- dup text\n\n## Open debts\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":9  dropped from Traps, untagged: dup text\n", stdout)
	assert.Contains(t, stderr, "(dropped 1 entry, listed on stdout)")
}

func Test_dropExcerpt(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "exactly eighty runes stays uncut",
			text: strings.Repeat("a", 80),
			want: strings.Repeat("a", 80),
		},
		{
			name: "eighty-one runes is cut to eighty runes plus an ellipsis",
			text: strings.Repeat("a", 81),
			want: strings.Repeat("a", 80) + "…",
		},
		{
			name: "multibyte text is cut on a rune boundary, not a byte one",
			text: strings.Repeat("é", 81),
			want: strings.Repeat("é", 80) + "…",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, dropExcerpt(c.text))
		})
	}
}

// Test_finish_json_dropped_entries_key_order_and_values_mem proves
// SCENARIO-04: --json's "dropped_entries" field is the document's last top-
// level key, each element's own keys appear in the order
// severity/rule/path/line/detail/heading/tag/text, the Open-debts element
// carries "rule":"dropped-debt" and "tag":null (untagged), the Traps
// element carries "rule":"dropped-entry" and its tag as a JSON string, and
// an over-80-rune entry's "text" is the full literal value while its
// "detail" is the same value cut to 80 runes plus "…". The old body keeps
// one entry under each heading a drop is reported from — Left unbuilt,
// Traps and Open debts — so a diff that ignored the new body entirely
// would still fail this test.
func Test_finish_json_dropped_entries_key_order_and_values_mem(t *testing.T) {
	longBase := strings.Repeat("A", 90)
	oldLines := []string{
		"## Binding decisions", "", // 1-2
		"- kept binding decision", "", // 3-4
		"## Left unbuilt", "", // 5-6
		"- kept left unbuilt", "", // 7-8
		"- " + longBase + " (LONGTAG)", "", // 9-10
		"## Traps", "", // 11-12
		"- kept trap", "", // 13-14
		"- a trap that gets dropped (TAGX)", "", // 15-16
		"## Open debts", "", // 17-18
		"- kept debt", "", // 19-20
		"- an untagged debt that gets dropped", // 21
	}
	oldState := strings.Join(oldLines, "\n") + "\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n- kept binding decision\n\n" +
		"## Left unbuilt\n\n- kept left unbuilt\n\n" +
		"## Traps\n\n- kept trap\n\n" +
		"## Open debts\n\n- kept debt\n"
	statePath := memWriteInput(tree, "state.md", newState)

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	stateFilePath := filepath.Join(featureDir, "STATE.md")
	stepFilePath := filepath.Join(featureDir, "SCENARIO-01.md")
	specFilePath := filepath.Join(featureDir, "specification.md")

	longExcerpt := strings.Repeat("A", 80) + "…"
	longText := longBase + " (LONGTAG)"

	wantDropped := `[` +
		`{"severity":"WARN","rule":"dropped-entry","path":` + memJSONString(t, stateFilePath) +
		`,"line":9,"detail":` + memJSONString(t, "dropped from Left unbuilt, tagged LONGTAG: "+longExcerpt) +
		`,"heading":"Left unbuilt","tag":"LONGTAG","text":` + memJSONString(t, longText) + `},` +
		`{"severity":"WARN","rule":"dropped-entry","path":` + memJSONString(t, stateFilePath) +
		`,"line":15,"detail":"dropped from Traps, tagged TAGX: a trap that gets dropped (TAGX)"` +
		`,"heading":"Traps","tag":"TAGX","text":"a trap that gets dropped (TAGX)"},` +
		`{"severity":"WARN","rule":"dropped-debt","path":` + memJSONString(t, stateFilePath) +
		`,"line":21,"detail":"dropped from Open debts, untagged: an untagged debt that gets dropped"` +
		`,"heading":"Open debts","tag":null,"text":"an untagged debt that gets dropped"}` +
		`]`

	want := `{"schema":1,"command":"finish","ok":true,"exit_code":0,"feature":"demo","step":"SCENARIO-01","changed":true,"handoff_path":` +
		memJSONString(t, filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")) + `,"state_path":` +
		memJSONString(t, stateFilePath) + `,"next":null,"modified":[` +
		memJSONString(t, stateFilePath) + `,` + memJSONString(t, stepFilePath) + `,` + memJSONString(t, specFilePath) +
		`],"dropped_entries":` + wantDropped + `}` + "\n"

	assert.Equal(t, want, stdout)
}

// Test_finish_json_dropped_entries_detail_matches_the_text_row_mem proves
// SCENARIO-04: a "dropped_entries" element's "detail" is byte-identical to
// the same drop's text-mode stdout row detail (the substring after the
// "WARN  <path>:<line>  " prefix) — both render through dropDetail, never
// through two separate string-building paths. runFinish's --json branch
// returns before the text-mode WARN-row loop ever runs, so the two renders
// come from two independently built fixtures rather than one shared run.
func Test_finish_json_dropped_entries_detail_matches_the_text_row_mem(t *testing.T) {
	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"- kept trap\n\n- a dropped entry (FOO)\n\n## Open debts\n"
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept trap\n\n## Open debts\n"

	textTree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	textHandoffPath := memWriteInput(textTree, "handoff.md", "NEW-HANDOFF\n")
	textStatePath := memWriteInput(textTree, "state.md", newState)

	textStdout, _, textErr := runFinishArgsMem(t, textTree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", textHandoffPath, "--state", textStatePath})
	require.NoError(t, textErr)

	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	textLine := memOneLine(t, textStdout)
	prefix := "WARN  " + stateRel + ":9  "
	require.True(t, strings.HasPrefix(textLine, prefix), "line %q must start with %q", textLine, prefix)
	wantDetail := strings.TrimPrefix(textLine, prefix)

	jsonTree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	jsonHandoffPath := memWriteInput(jsonTree, "handoff.md", "NEW-HANDOFF\n")
	jsonStatePath := memWriteInput(jsonTree, "state.md", newState)

	jsonStdout, _, jsonErr := runFinishArgsMem(t, jsonTree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", jsonHandoffPath, "--state", jsonStatePath, "--json"})
	require.NoError(t, jsonErr)

	var doc struct {
		Dropped []struct {
			Detail string `json:"detail"`
		} `json:"dropped_entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStdout), &doc))
	require.Len(t, doc.Dropped, 1)

	assert.Equal(t, wantDetail, doc.Dropped[0].Detail)
}

// memDroppedEntryNewState is the new-pair state body every SCENARIO-09
// _mem test below reuses as its own diverging --state argument: the same
// "- kept entry" survivor Test_finish_prints_a_warn_row_for_a_dropped_entry_mem's
// own newState carries, omitting memOldStateWithDroppedEntry()'s own tagged
// "- X (SCENARIO-02)" entry at old-file line 17.
func memDroppedEntryNewState() string {
	return "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n"
}

// memStateDivergedDropFixture is newMemStateDivergedDropFixture's own
// return shape: mem already carries the fixture's first finish call, and
// handoffPath/newStatePath are the --handoff/--state arguments every later
// call reuses unchanged, alongside recordedHandoffPath (the step's own
// recorded handoff file, for a test that removes it) and stateFilePath
// (the feature's own recorded STATE.md, for a refusal's own Path
// assertion).
type memStateDivergedDropFixture struct {
	mem                                                           *rwfs.Mem
	handoffPath, newStatePath, recordedHandoffPath, stateFilePath string
}

// newMemStateDivergedDropFixture builds "demo"/SCENARIO-01 already done
// against memOldStateWithDroppedEntry()'s own body, recording handoffPath's
// bytes as the step's own handoff file — the shared done-step drop-bearing
// pair SCENARIO-09's Context describes. Every later call reuses fx.mem
// directly: tree.mem() is never called a second time, since a second call
// would take a fresh copy of tree's own entries and silently discard this
// first finish's own writes, turning an intended re-finish into a first
// finish. handoffPath's bytes are supplied unchanged to every later call
// too — only newStatePath's own body and recordedHandoffPath's own
// presence vary — so the refusal a later call hits is provably
// refinishStateDiverged, never refinishHandoffDiverged.
func newMemStateDivergedDropFixture(t *testing.T) memStateDivergedDropFixture {
	t.Helper()

	tree := newMemFinishFixtureWithState("- [x] do the thing", memOldStateWithDroppedEntry())
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	oldStatePath := memWriteInput(tree, "old-state.md", memOldStateWithDroppedEntry())
	newStatePath := memWriteInput(tree, "new-state.md", memDroppedEntryNewState())
	mem := tree.mem()

	_, _, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", oldStatePath})
	require.NoError(t, err)

	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	return memStateDivergedDropFixture{
		mem:                 mem,
		handoffPath:         handoffPath,
		newStatePath:        newStatePath,
		recordedHandoffPath: filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"),
		stateFilePath:       filepath.Join(featureDir, "STATE.md"),
	}
}

// Test_finish_accepts_a_drop_bearing_re_finish_when_the_handoff_file_is_missing_mem
// is SCENARIO-09's control arm: with the recorded handoff file removed,
// FinishFS's row-2 exemption takes refinishWrite instead of
// refinishStateDiverged, so the state-diverged re-finish against
// memDroppedEntryNewState() succeeds and prints the dropped entry's WARN
// row — proving the fixture pair really is drop-bearing on the write path,
// the arm Test_finish_refuses_a_state_diverged_re_finish_mem's
// refusal is checked against.
func Test_finish_accepts_a_drop_bearing_re_finish_when_the_handoff_file_is_missing_mem(t *testing.T) {
	fx := newMemStateDivergedDropFixture(t)
	require.NoError(t, fx.mem.Remove(memKey(fx.recordedHandoffPath)))

	stdout, _, err := runFinishArgsMem(t, fx.mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", fx.handoffPath, "--state", fx.newStatePath})

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)\n", stdout)
}

// Test_finish_refuses_a_state_diverged_re_finish_mem
// proves D5's first enforcement point at the CLI: a re-finish whose state
// argument diverges from what is recorded (refinishStateDiverged) refuses
// with ErrAlreadyFinished naming the state file — the same pair is
// drop-bearing when it reaches the write path (this test's own control
// arm, above). It does not itself assert stdout is empty: runFinish's
// `srv.Finish` error branch returns before the WARN-row loop is reachable
// at all, so no mutation of this feature's own code could ever make this
// refusal print a row; that is proven instead by D5's dedicated
// scaffold-level test (internal/scaffold/finish_dropped_test.go
// Test_finish_state_diverged_refusal_returns_no_dropped_entries).
func Test_finish_refuses_a_state_diverged_re_finish_mem(t *testing.T) {
	fx := newMemStateDivergedDropFixture(t)

	_, _, err := runFinishArgsMem(t, fx.mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", fx.handoffPath, "--state", fx.newStatePath})

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stateFilePath, refusal.Path)
	assert.Contains(t, refusal.Problem, "state differs")
}

// Test_finish_json_state_diverged_refusal_has_no_dropped_entries_key_mem is
// Test_finish_refuses_a_state_diverged_re_finish_mem's
// --json counterpart: the raw stdout bytes never carry the
// "dropped_entries" substring at all — not merely an empty array — and the
// decoded error document's own path names the state file.
func Test_finish_json_state_diverged_refusal_has_no_dropped_entries_key_mem(t *testing.T) {
	fx := newMemStateDivergedDropFixture(t)

	stdout, stderr, err := runFinishArgsMem(t, fx.mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", fx.handoffPath, "--state", fx.newStatePath, "--json"})

	require.Error(t, err)
	assert.Empty(t, stderr)
	assert.NotContains(t, stdout, "dropped_entries")

	decoded := memDecodeErrorDocument(t, []byte(stdout), "finish")
	require.NotNil(t, decoded.Path)
	assert.Equal(t, fx.stateFilePath, *decoded.Path)
}

// Test_finish_identical_re_finish_of_a_drop_bearing_step_reports_the_noop_line_mem
// proves SCENARIO-09's second Gherkin clause: an identical re-finish of a
// step whose first finish already dropped an entry prints the existing
// "already done with identical inputs" line the second time — even though
// the first call's own WARN row proves the pair really is drop-bearing.
// refinishNoop (internal/scaffold/finish.go) never calls droppedEntries at
// all and returns a literal empty Dropped slice, which is why there is no
// row to print; this test does not itself assert the second call's stdout
// is empty, since runFinish's success path only ever writes a row per
// res.Dropped element (writeDroppedRows), so an empty Dropped slice
// already guarantees empty stdout regardless of any mutation this
// feature's own row-writing code could introduce.
func Test_finish_identical_re_finish_of_a_drop_bearing_step_reports_the_noop_line_mem(t *testing.T) {
	tree := newMemFinishFixtureWithState("- [x] do the thing", memOldStateWithDroppedEntry())
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", memDroppedEntryNewState())
	mem := tree.mem()
	args := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}

	firstStdout, _, firstErr := runFinishArgsMem(t, mem, args)
	require.NoError(t, firstErr)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	require.Equal(t, "WARN  "+stateRel+":17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)\n", firstStdout)

	_, secondStderr, secondErr := runFinishArgsMem(t, mem, args)

	require.NoError(t, secondErr)
	assert.Equal(t, "brief finish: demo SCENARIO-01 already done with identical inputs; nothing written\n", secondStderr)
}

// Test_finish_json_identical_re_finish_reports_no_dropped_entries_mem is
// Test_finish_identical_re_finish_of_a_drop_bearing_step_reports_the_noop_line_mem's
// --json counterpart: the second, identical call's document matches an
// exact literal — "changed":false, "modified":[], "dropped_entries":[] —
// so a coincidental write that also nets zero drops cannot pass as a
// no-op.
func Test_finish_json_identical_re_finish_reports_no_dropped_entries_mem(t *testing.T) {
	tree := newMemFinishFixtureWithState("- [x] do the thing", memOldStateWithDroppedEntry())
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", memDroppedEntryNewState())
	mem := tree.mem()
	args := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}

	_, _, firstErr := runFinishArgsMem(t, mem, args)
	require.NoError(t, firstErr)

	secondStdout, secondStderr, secondErr := runFinishArgsMem(t, mem, args)

	require.NoError(t, secondErr)
	assert.Empty(t, secondStderr)

	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	stateFilePath := filepath.Join(featureDir, "STATE.md")
	handoffFilePath := filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")

	want := `{"schema":1,"command":"finish","ok":true,"exit_code":0,"feature":"demo","step":"SCENARIO-01","changed":false,"handoff_path":` +
		memJSONString(t, handoffFilePath) + `,"state_path":` + memJSONString(t, stateFilePath) +
		`,"next":null,"modified":[],"dropped_entries":[]}` + "\n"

	assert.Equal(t, want, secondStdout)
}

// errRowWriteRefused is failNthWriter's own sentinel: the error its
// configured call number returns.
var errRowWriteRefused = errors.New("row write refused")

// failNthWriter is an io.Writer whose callToFail'th Write call fails,
// buffering every other call including ones *after* the failure: a fake
// that fails every call from some point onward cannot distinguish "stop at
// the first failure" from "capture the first error but keep looping" —
// both write exactly the calls before the failure and nothing after,
// since every later call would fail anyway. Only a fake that would
// *succeed* again on a later row, if the implementation wrongly kept
// going, can tell the two apart.
type failNthWriter struct {
	callToFail int
	calls      int
	buf        bytes.Buffer
}

func (w *failNthWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.callToFail {
		return 0, errRowWriteRefused
	}

	return w.buf.Write(p)
}

// Test_finish_stops_at_the_first_dropped_row_write_error_and_reports_an_internal_error_mem
// proves the MAJOR fix at internal/cli/finish.go: runFinish no longer
// ignores a WARN-row write error. The old body carries three dropped
// entries; the fake stdout writer fails only its *second* call and would
// succeed on a third if writeDroppedRows kept going past the failure — so
// the buffer holding exactly the first row's bytes, and nothing from the
// third, is proof the loop actually stops rather than merely capturing
// the first error while continuing to write every later row that itself
// happens to succeed. runFinish also never goes on to print the
// "replaced ..." success line, and reports the failure through the
// package's own visible error path (reporter.refusal), stating that the
// state was already replaced — not a bare wrapped error main.go would
// exit on without ever printing (SilenceErrors is set on the root
// command, so nothing but ExitCode(err) is ever read from a plain
// returned error).
func Test_finish_stops_at_the_first_dropped_row_write_error_and_reports_an_internal_error_mem(t *testing.T) {
	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"- first dropped\n- second dropped\n- third dropped\n\n## Open debts\n\n"
	tree := newMemFinishFixtureWithState("- [x] do the thing", oldState)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n\n"
	statePath := memWriteInput(tree, "state.md", newState)

	writer := &failNthWriter{callToFail: 2}
	var stderr strings.Builder

	err := run(t.Context(), memRoot, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath},
		nil, writer, &stderr, noBuildInfo, withRootFS(tree.mem()))

	require.Error(t, err)
	require.ErrorIs(t, err, errRowWriteRefused)
	assert.Equal(t, 1, ExitCode(err))

	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":7  dropped from Traps, untagged: first dropped\n", writer.buf.String())
	assert.Equal(t,
		"brief finish: state replaced but dropped entries could not be written: write dropped-entry row: "+
			errRowWriteRefused.Error()+"\n",
		stderr.String())
}
