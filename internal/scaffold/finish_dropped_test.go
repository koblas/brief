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

	return finishDroppedWithConfig(t, mem, newState, config.Default())
}

// finishDroppedWithConfig runs FinishFS for "widgets"/SCENARIO-02 against
// mem under cfg, letting a case build a config.Config with a blanked or
// duplicated StateHeadings field while newDroppedFixtureFS's own step and
// specification bodies stay pinned to config.Default().
func finishDroppedWithConfig(t *testing.T, mem *rwfs.Mem, newState []byte, cfg config.Config) (scaffold.FinishResult, error) {
	t.Helper()

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

// Test_finish_ignores_list_lines_that_are_not_true_entries proves D1: a list
// line inside a fenced code block, a paragraph line, and a list item under a
// fifth, non-configured heading are never entries. Each case's old body also
// carries one genuine entry under a configured heading that the new body
// drops, so an implementation that over- or under-scans cannot pass by
// reporting zero rows either way.
func Test_finish_ignores_list_lines_that_are_not_true_entries(t *testing.T) {
	cases := []struct {
		name     string
		oldState string
		newState string
		wantLine int
	}{
		{
			name: "a list line inside a fenced code block is not an entry",
			oldState: strings.Join([]string{
				"## Binding decisions", "",
				"## Left unbuilt", "",
				"## Traps", "",
				"```", "- fenced list line", "```", "",
				"- genuine entry", "",
				"## Open debts", "",
			}, "\n") + "\n",
			newState: "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n",
			wantLine: 11,
		},
		{
			name: "a paragraph line under a heading is not an entry",
			oldState: strings.Join([]string{
				"## Binding decisions", "",
				"## Left unbuilt", "",
				"## Traps", "",
				"just a paragraph line, not a list item", "",
				"- genuine entry", "",
				"## Open debts", "",
			}, "\n") + "\n",
			newState: "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n",
			wantLine: 9,
		},
		{
			name: "a list item under a non-configured heading is not an entry",
			oldState: strings.Join([]string{
				"## Binding decisions", "",
				"## Left unbuilt", "",
				"## Traps", "",
				"- genuine entry", "",
				"## Notes", "",
				"- untouched note", "",
				"## Open debts", "",
			}, "\n") + "\n",
			newState: "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Notes\n\n## Open debts\n",
			wantLine: 7,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := newDroppedFixtureFS(t, c.oldState)

			res, err := finishDropped(t, mem, []byte(c.newState))

			require.NoError(t, err)
			assert.Equal(t, []scaffold.DroppedEntry{
				{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: c.wantLine, Tag: "", Text: "genuine entry"},
			}, res.Dropped)
		})
	}
}

// Test_finish_attributes_no_drop_to_a_heading_absent_from_the_old_body proves
// a configured heading entirely missing from the old body (no line equal to
// it at all) contributes nothing, while a different configured heading's
// genuine drop is still reported.
func Test_finish_attributes_no_drop_to_a_heading_absent_from_the_old_body(t *testing.T) {
	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"## Traps", "",
		"- genuine entry", "",
		"## Open debts", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 5, Tag: "", Text: "genuine entry"},
	}, res.Dropped)
}

// Test_finish_reports_no_drops_from_an_empty_old_state_file proves D7's
// empty-old-file clause: a zero-byte old body yields no drops. The second
// case is the control arm, differing only in the old body's own content — a
// non-empty old body still reports its genuine drop, so the first case's
// empty slice is proof the guard fires on emptiness, not on the assertion
// being unreachable.
func Test_finish_reports_no_drops_from_an_empty_old_state_file(t *testing.T) {
	t.Run("an empty old state body yields no drops", func(t *testing.T) {
		mem := newDroppedFixtureFS(t, "")
		newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

		res, err := finishDropped(t, mem, newState)

		require.NoError(t, err)
		assert.Empty(t, res.Dropped)
	})

	t.Run("a non-empty old state body with a genuine drop is still reported", func(t *testing.T) {
		oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- genuine entry\n\n## Open debts\n"
		mem := newDroppedFixtureFS(t, oldState)
		newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

		res, err := finishDropped(t, mem, newState)

		require.NoError(t, err)
		assert.Equal(t, []scaffold.DroppedEntry{
			{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 7, Tag: "", Text: "genuine entry"},
		}, res.Dropped)
	})
}

// Test_finish_excludes_an_empty_configured_heading_from_scanning proves D7's
// empty-heading clause at the scaffold level: cfg.StateHeadings.OpenDebts
// blanked to "" contributes zero entries from either position it would
// otherwise be scanned from, rather than matching markdown.Section's
// first-blank-line fallback for "". The old body's genuine Traps entry and a
// fifth, non-configured "## Notes" section's own entry both sit after the
// body's first blank line, so an implementation that still scans "" would
// pick up both as false or duplicate rows.
func Test_finish_excludes_an_empty_configured_heading_from_scanning(t *testing.T) {
	cfg := config.Default()
	cfg.StateHeadings.OpenDebts = ""

	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- genuine entry", "",
		"## Notes", "",
		"- notes entry", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 7, Tag: "", Text: "genuine entry"},
	}, res.Dropped)
}

// Test_finish_excludes_a_duplicated_configured_heading_from_scanning proves
// D7's duplicated-heading clause: two StateHeadings fields set to the
// identical text ("## Shared") contribute zero entries from either
// position — not one, per the binding decision that a duplicated heading is
// excluded wholesale rather than "keeping the first". The Left-unbuilt
// entry, under a distinct, undupicated heading, is still reported.
func Test_finish_excludes_a_duplicated_configured_heading_from_scanning(t *testing.T) {
	cfg := config.Default()
	cfg.StateHeadings.BindingDecisions = "## Shared"
	cfg.StateHeadings.Traps = "## Shared"

	oldState := strings.Join([]string{
		"## Shared", "",
		"- shared entry", "",
		"## Left unbuilt", "",
		"- left entry", "",
		"## Open debts", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Shared\n\n## Left unbuilt\n\n## Open debts\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Left unbuilt", Line: 7, Tag: "", Text: "left entry"},
	}, res.Dropped)
}

// Test_finish_keeps_correct_line_numbers_and_blank_line_handling_under_crlf
// proves an old body using CRLF line endings throughout still reports the
// entry's correct 1-based physical line, and that the fold's blank-line
// check treats a bare "\r" line as blank: a kept entry immediately followed
// by such a line and then a paragraph line must not fold the paragraph in,
// which would change its normalized text against the new body's plain-LF,
// symmetric-content counterpart and report it as a second, false drop.
func Test_finish_keeps_correct_line_numbers_and_blank_line_handling_under_crlf(t *testing.T) {
	oldLines := []string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- kept entry",
		"",
		"paragraph line, not an entry", "",
		"## Open debts", "",
		"- dropped entry", "",
	}
	oldState := strings.ReplaceAll(strings.Join(oldLines, "\n")+"\n", "\n", "\r\n")
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte(
		"## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
			"- kept entry\n\nparagraph line, not an entry\n\n## Open debts\n",
	)

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleDebt, Heading: "Open debts", Line: 13, Tag: "", Text: "dropped entry"},
	}, res.Dropped)
}

// Test_finish_normalizes_an_interior_carriage_return_on_a_continuation_line
// proves normalizeEntryText's ReplaceAll(raw, "\r", "") strips a carriage
// return embedded mid-continuation-line — one trimEOL/TrimSpace, which only
// trim a line's own edges, never reach. The "\r" sits directly between two
// non-whitespace characters, with no adjacent space of its own: deleting it
// concatenates them into one word, which is what discriminates this from
// strings.Fields alone, since Fields already treats a lone "\r" as a
// separator identical to a space — a "\r" with a space on either side would
// tokenize the same whether or not the strip ran. The interior-CR entry's
// old and new forms normalize to the same text and so are not reported; a
// genuinely dropped, unrelated entry under a different heading is.
func Test_finish_normalizes_an_interior_carriage_return_on_a_continuation_line(t *testing.T) {
	oldState := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"- kept multiline entry\n" +
		"with an embedded\rcarriage return\n\n" +
		"## Open debts\n\n" +
		"- unrelated dropped entry\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n" +
		"- kept multiline entry\n" +
		"with an embeddedcarriage return\n\n" +
		"## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleDebt, Heading: "Open debts", Line: 12, Tag: "", Text: "unrelated dropped entry"},
	}, res.Dropped)
}
