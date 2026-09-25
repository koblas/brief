package scaffold_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// droppedFixtureStep and droppedFixtureSpec build the step file and
// specification every finish_dropped_test.go case needs, against
// config.Default() — the default state headings ("## Traps", "## Open
// debts", ...) are pinned literally in this file's expectations, unlike
// the rest of this package's fixtureConfig-built tests, so this file
// builds its own bespoke config.Default()-based feature rather than
// reusing newFinishFixtureFS.
func droppedFixtureStep(cfg config.Config) string {
	return "---\n" +
		"id: SCENARIO-02\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-02\n\n" +
		cfg.ChecklistHeading + "\n\n" +
		"- [x] done\n"
}

func droppedFixtureSpec(cfg config.Config) string {
	return "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] SCENARIO-02\n"
}

// newDroppedFixtureFS builds a "widgets" feature on an rwfs.Mem, one open
// step SCENARIO-02, against config.Default() so its state headings read
// literally "## Traps" / "## Open debts", with oldState as the state
// file's own on-disk body.
func newDroppedFixtureFS(t *testing.T, oldState string) *rwfs.Mem {
	t.Helper()

	cfg := config.Default()

	return rwfs.NewMem(fstest.MapFS{
		"SCENARIO-02.md":      &fstest.MapFile{Data: []byte(droppedFixtureStep(cfg)), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(droppedFixtureSpec(cfg)), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(oldState), Mode: 0o600},
	})
}

// finishDropped runs FinishFS for "widgets"/SCENARIO-02 against mem, built
// with config.Default().
func finishDropped(t *testing.T, mem *rwfs.Mem, newState []byte) (scaffold.FinishResult, error) {
	t.Helper()

	cfg := config.Default()
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	handoffPattern, err := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, err)

	srv := scaffold.NewServer(cfg, "")

	return srv.FinishFS(mem, testFeaturePath, "widgets", "SCENARIO-02", []byte("h"), newState, pattern, handoffPattern)
}

// droppedOldStateWithEntryAtLine17 is the old STATE.md body SCENARIO-01's
// own Gherkin fixture describes: one entry under "## Traps" kept
// ("- kept entry"), and "- X (SCENARIO-02)" at whole-body line 17 — reached
// by padding with prose filler lines, none of which are list items, so
// they contribute no entry of their own.
func droppedOldStateWithEntryAtLine17() string {
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

// Test_finish_reports_an_entry_the_new_state_body_omits pins SCENARIO-01's
// own Gherkin fixture: X drops out at its old-file line, tagged, under the
// dropped-entry rule.
func Test_finish_reports_an_entry_the_new_state_body_omits(t *testing.T) {
	mem := newDroppedFixtureFS(t, droppedOldStateWithEntryAtLine17())
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 17, Tag: "SCENARIO-02", Text: "X (SCENARIO-02)"},
	}, res.Dropped)
}

// Test_finish_classifies_an_open_debts_drop_as_dropped_debt proves D4's
// rule split: an entry dropped from under the configured Open debts
// heading carries dropped-debt, not dropped-entry, and is reported
// untagged when it carries no trailing "(<token>)".
func Test_finish_classifies_an_open_debts_drop_as_dropped_debt(t *testing.T) {
	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"## Open debts\n\n- D — unowned — dies unless re-opened\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	require.Len(t, res.Dropped, 1)
	assert.Equal(t, scaffold.DropRuleDebt, res.Dropped[0].Rule)
	assert.Empty(t, res.Dropped[0].Tag)
}

// Test_finish_reports_drops_in_old_file_line_order_across_headings proves
// the pooled diff sorts by the old file's own line order, not by
// cfg.StateHeadings.Ordered()'s configured order: "## Open debts" is
// written before "## Traps" here, the reverse of Ordered(), so a drop
// under each must still come back Open-debts-first.
func Test_finish_reports_drops_in_old_file_line_order_across_headings(t *testing.T) {
	oldState := "## Open debts\n\n- OD (TAG1)\n\n" +
		"## Binding decisions\n\n## Left unbuilt\n\n" +
		"## Traps\n\n- TR (TAG2)\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	require.Len(t, res.Dropped, 2)
	assert.Equal(t, 3, res.Dropped[0].Line, "the Open debts entry is written first in the old file")
	assert.Equal(t, "Open debts", res.Dropped[0].Heading)
	assert.Equal(t, 11, res.Dropped[1].Line, "the Traps entry follows it in the old file")
	assert.Equal(t, "Traps", res.Dropped[1].Heading)
}
