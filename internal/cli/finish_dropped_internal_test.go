// finish's dropped-entry rows (SCENARIO-01 of dropped-entries): the text
// branch's stdout WARN rows and its stderr "(dropped N ...)" suffix,
// against rwfs.Mem the same way finish_internal_test.go's own _mem tests
// do, plus a white-box table for the unexported excerpt helper.

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

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
