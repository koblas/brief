// White-box: finish's dropped-entry rows against rwfs.Mem, plus a table
// for the unexported dropExcerpt helper.

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
// additionally on the old STATE.md body.
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
// section holds one kept entry and one dropped entry, at line 17.
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

// A drop under "## Open debts" renders its heading and untagged branch
// correctly.
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

// A doubled interior space and a tab standing in for a space are
// whitespace normalizeEntryText already collapses generically, never a drop.
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

// droppedEntries pools entries across all four configured headings as one
// multiset keyed on text alone — an entry's heading is not its identity.
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

// entryItemText strips only the "- "/"* "/"N. " marker, never a leading
// checkbox, so a reworded, re-tagged, or re-ticked entry is a drop.
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
// "## Traps" section holds a multi-line entry, at whole-body line 9.
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

// markdown.Entries folds a multi-line entry's continuation and indented
// sub-item into one Text, so reflowing to one line is never a drop.
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

// Control arm: the new body omits the multi-line entry entirely instead
// of reflowing it; the reported row's line is the entry's first old line.
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

// Surplus old occurrences — the last ones in old-file order — are dropped;
// old carries the identical text twice, new carries it once.
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

// --json's "dropped_entries" is the document's last top-level key; each
// element's keys appear in order severity/rule/path/line/detail/heading/tag/text.
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

// A "dropped_entries" element's "detail" is byte-identical to the same
// drop's text-mode stdout row detail; both render through dropDetail.
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

// memDroppedEntryNewState is the new-pair state body the tests below reuse
// as their diverging --state argument.
func memDroppedEntryNewState() string {
	return "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n"
}

// memStateDivergedDropFixture is newMemStateDivergedDropFixture's return
// shape: mem already carries the fixture's first finish call.
type memStateDivergedDropFixture struct {
	mem                                                           *rwfs.Mem
	handoffPath, newStatePath, recordedHandoffPath, stateFilePath string
}

// newMemStateDivergedDropFixture builds "demo"/SCENARIO-01 already done.
// Every later call reuses fx.mem directly: tree.mem() taking a fresh copy
// would silently discard this first finish's own writes.
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

// Control arm: with the recorded handoff file removed, FinishFS's row-2
// exemption takes refinishWrite instead of refinishStateDiverged.
func Test_finish_accepts_a_drop_bearing_re_finish_when_the_handoff_file_is_missing_mem(t *testing.T) {
	fx := newMemStateDivergedDropFixture(t)
	require.NoError(t, fx.mem.Remove(memKey(fx.recordedHandoffPath)))

	stdout, _, err := runFinishArgsMem(t, fx.mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", fx.handoffPath, "--state", fx.newStatePath})

	require.NoError(t, err)
	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)\n", stdout)
}

// A re-finish whose state argument diverges from what is recorded refuses
// with ErrAlreadyFinished naming the state file.
func Test_finish_refuses_a_state_diverged_re_finish_mem(t *testing.T) {
	fx := newMemStateDivergedDropFixture(t)

	_, _, err := runFinishArgsMem(t, fx.mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", fx.handoffPath, "--state", fx.newStatePath})

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stateFilePath, refusal.Path)
	assert.Contains(t, refusal.Problem, "state differs")
}

// The raw stdout bytes never carry the "dropped_entries" substring at
// all, not merely an empty array.
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

// An identical re-finish of a step whose first finish already dropped an
// entry prints the existing "already done" line the second time.
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

// The second, identical call's document matches an exact literal, so a
// coincidental write that also nets zero drops cannot pass as a no-op.
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

// errRowWriteRefused is the error failNthWriter's configured call returns.
var errRowWriteRefused = errors.New("row write refused")

// failNthWriter is an io.Writer whose callToFail'th Write call fails,
// buffering every other call including ones after the failure.
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

// writeDroppedRows stops at the first row-write error rather than
// continuing past it; runFinish surfaces that error as an exit-1 refusal.
func Test_finish_stops_writing_dropped_rows_on_the_first_write_error_mem(t *testing.T) {
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

	require.ErrorIs(t, err, errRowWriteRefused)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite)
	assert.Equal(t, 1, ExitCode(err))

	stateRel := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Equal(t, "WARN  "+stateRel+":7  dropped from Traps, untagged: first dropped\n", writer.buf.String())
	assert.Equal(t,
		"brief finish: demo SCENARIO-01 done, but the dropped-entries report failed after 1 of 3 rows: "+
			errRowWriteRefused.Error()+"; compare "+stateRel+
			" with its previous version to see what was removed — a retry reports nothing\n",
		stderr.String())
}
