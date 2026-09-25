// finish's dropped-entry rows (SCENARIO-01 of dropped-entries): the text
// branch's stdout WARN rows and its stderr "(dropped N ...)" suffix,
// against rwfs.Mem the same way finish_internal_test.go's own _mem tests
// do, plus a white-box table for the unexported excerpt helper.

package cli

import (
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

// Test_finish_reports_an_open_debts_drop_as_dropped_debt_mem proves the
// dropped-debt rule and the untagged rendering end-to-end through the CLI
// against "## Open debts" specifically (SCENARIO-02 of dropped-entries):
// dropRuleFor and dropDetail's untagged branch were already unit-tested at
// Server level by SCENARIO-01, but no CLI-level test had exercised the
// Open debts heading or the "unowned — dies unless re-opened" text before
// this.
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
