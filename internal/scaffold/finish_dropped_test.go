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
// specification every case in this file needs, against config.Default():
// its own state headings are pinned literally in this file's expectations,
// so it builds its own bespoke feature rather than reusing
// newFinishFixtureFS.
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
// step SCENARIO-02, against config.Default(), with oldState as the state
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
// mem under cfg, letting a case reconfigure StateHeadings while
// newDroppedFixtureFS's own bodies stay pinned to config.Default().
func finishDroppedWithConfig(t *testing.T, mem *rwfs.Mem, newState []byte, cfg config.Config) (scaffold.FinishResult, error) {
	t.Helper()

	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	handoffPattern, err := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, err)

	srv := scaffold.NewServer(cfg, "")

	return srv.FinishFS(mem, testFeaturePath, "widgets", "SCENARIO-02", []byte("h"), newState, pattern, handoffPattern)
}

// droppedOldStateWithEntryAtLine17 is an old STATE.md body: one entry
// under "## Traps" kept ("- kept entry"), and "- X (SCENARIO-02)" at
// whole-body line 17, reached by padding with prose filler lines that
// contribute no entry of their own.
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

func Test_finish_reports_an_entry_the_new_state_body_omits(t *testing.T) {
	mem := newDroppedFixtureFS(t, droppedOldStateWithEntryAtLine17())
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n")

	res, err := finishDropped(t, mem, newState)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 17, Tag: "SCENARIO-02", Text: "X (SCENARIO-02)"},
	}, res.Dropped)
}

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

// "## Open debts" is written before "## Traps" here, the reverse of
// Ordered(), so a drop under each must still come back Open-debts-first.
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

// A list line inside a fenced code block, a paragraph line, and a list
// item under a fifth, non-configured heading are never entries. Each
// case's old body also carries one genuine entry that the new body drops,
// so an implementation that over- or under-scans cannot pass vacuously.
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
				"- notes entry", "",
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

// The second subtest is the control arm, differing only in the old body's
// own content, proving the first's empty slice is not vacuous.
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

// A blanked heading ("") must not match markdown.Section's
// first-blank-line fallback and pick up unrelated content after it.
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

// Two StateHeadings fields set to the identical text contribute zero
// entries, excluded wholesale rather than "keeping the first".
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

// A bare "\r" line must fold as blank, or the following paragraph line
// would join the kept entry's text and report a false second drop.
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

// The "\r" sits directly between two non-whitespace characters with no
// adjacent space, so only an explicit strip (not strings.Fields alone, for
// which a lone "\r" already separates like a space) makes old and new
// normalize to the same text.
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

// newDroppedReFinishFixtureFS builds newDroppedFixtureFS's "widgets"
// feature and runs one FinishFS call against oldState and handoff, so the
// fixture opens on a step already done. Every later call in this file
// reuses the returned mem directly; a second build would discard this
// first finish's own writes.
func newDroppedReFinishFixtureFS(t *testing.T, oldState string, handoff []byte) *rwfs.Mem {
	t.Helper()

	mem := newDroppedFixtureFS(t, oldState)

	cfg := config.Default()
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	handoffPattern, err := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, err)

	srv := scaffold.NewServer(cfg, "")
	_, err = srv.FinishFS(mem, testFeaturePath, "widgets", "SCENARIO-02", handoff, []byte(oldState), pattern, handoffPattern)
	require.NoError(t, err)

	return mem
}

// droppedReFinishHandoffName is the recorded handoff file
// newDroppedReFinishFixtureFS's own first finish call writes.
func droppedReFinishHandoffName() string {
	return "SCENARIO-02" + config.Default().HandoffFileSuffix
}

// The second subtest, removing the recorded handoff file, is the control
// arm proving the same old/new pair is drop-bearing on the write path.
func Test_finish_state_diverged_refusal_returns_no_dropped_entries(t *testing.T) {
	oldState := droppedOldStateWithEntryAtLine17()
	newState := []byte("## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n- kept entry\n\n## Open debts\n")

	t.Run("handoff file present refuses with no dropped entries", func(t *testing.T) {
		mem := newDroppedReFinishFixtureFS(t, oldState, []byte("h"))

		res, err := finishDropped(t, mem, newState)

		require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)
		assert.Empty(t, res.Dropped)
	})

	t.Run("handoff file missing writes and reports the dropped entry", func(t *testing.T) {
		mem := newDroppedReFinishFixtureFS(t, oldState, []byte("h"))
		require.NoError(t, mem.Remove(droppedReFinishHandoffName()))

		res, err := finishDropped(t, mem, newState)

		require.NoError(t, err)
		assert.Equal(t, []scaffold.DroppedEntry{
			{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 17, Tag: "SCENARIO-02", Text: "X (SCENARIO-02)"},
		}, res.Dropped)
	})
}

// droppedNestedOpenDebtsConfig returns config.Default() with OpenDebts
// reconfigured to "### Open debts", one level deeper than "## Traps", so a
// body writing headings in Ordered() order nests Open debts' section
// inside Traps' rather than after it.
func droppedNestedOpenDebtsConfig() config.Config {
	cfg := config.Default()
	cfg.StateHeadings.OpenDebts = "### Open debts"

	return cfg
}

// droppedNestedOldState is every nested-heading test's shared old body:
// "### Open debts" sits inside "## Traps"' own section, holding
// "- debt entry" at line 11; "## Traps" holds "- trap entry" at line 7,
// kept unchanged in every case below.
func droppedNestedOldState() string {
	return strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- trap entry", "",
		"### Open debts", "",
		"- debt entry", "",
	}, "\n") + "\n"
}

// "- debt entry" sits physically inside both "## Traps" and "### Open
// debts"; each old-body line must attribute to only its one
// nearest-enclosing heading, or moving the entry reads as a false drop.
func Test_finish_treats_an_entry_moved_out_of_a_nested_open_debts_heading_as_no_drop(t *testing.T) {
	cfg := droppedNestedOpenDebtsConfig()
	mem := newDroppedFixtureFS(t, droppedNestedOldState())
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"- debt entry", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- trap entry", "",
		"### Open debts", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Empty(t, res.Dropped)
}

// Same nested fixture, but "- debt entry" is omitted entirely: reported
// once, under its own nearest-enclosing heading ("### Open debts",
// dropped-debt), not twice under both headings.
func Test_finish_reports_a_drop_under_a_nested_open_debts_heading_exactly_once(t *testing.T) {
	cfg := droppedNestedOpenDebtsConfig()
	mem := newDroppedFixtureFS(t, droppedNestedOldState())
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- trap entry", "",
		"### Open debts", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleDebt, Heading: "Open debts", Line: 11, Tag: "", Text: "debt entry"},
	}, res.Dropped)
}

// "### Notes" (unconfigured) is a sibling of "### Open debts" under "##
// Traps", so only "## Traps"' own overrunning scan reaches its entry — not
// "the nearest configured heading by line", which would misattribute it.
func Test_finish_attributes_a_drop_under_an_unconfigured_subsection_to_its_actual_ancestor(t *testing.T) {
	cfg := droppedNestedOpenDebtsConfig()
	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- trap entry", "",
		"### Open debts", "",
		"- debt entry", "",
		"### Notes", "",
		"- notes entry", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"- trap entry", "",
		"### Open debts", "",
		"- debt entry", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 15, Tag: "", Text: "notes entry"},
	}, res.Dropped)
}

// droppedReversedNestingConfig reconfigures Traps to "### Traps", one
// level deeper than "## Open debts": Traps is scanned before OpenDebts in
// Ordered()'s fixed order, but is the physically deeper heading here.
func droppedReversedNestingConfig() config.Config {
	cfg := config.Default()
	cfg.StateHeadings.Traps = "### Traps"

	return cfg
}

// An implementation that let the later scan win regardless of heading
// depth would misattribute "- trap entry" to "Open debts"; the correct,
// depth-based attribution is "Traps".
func Test_finish_attributes_a_drop_to_the_physically_deeper_heading_even_when_scanned_first(t *testing.T) {
	cfg := droppedReversedNestingConfig()
	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Open debts", "",
		"- debt entry", "",
		"### Traps", "",
		"- trap entry", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Open debts", "",
		"- debt entry", "",
		"### Traps", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Traps", Line: 11, Tag: "", Text: "trap entry"},
	}, res.Dropped)
}

// poolOccurrences' by-line dedup must apply to the new body too, not only
// the old: here "### Open debts" nests inside "## Traps" in the new body
// only, so an un-deduped new-side count would read "dup text" as present
// twice and report no drop at all.
func Test_finish_dedupes_a_new_bodys_nested_heading_overlap_before_diffing(t *testing.T) {
	cfg := droppedNestedOpenDebtsConfig()
	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"- dup text", "",
		"## Left unbuilt", "",
		"- dup text", "",
		"## Traps", "",
		"### Open debts", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"## Traps", "",
		"### Open debts", "",
		"- dup text", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleEntry, Heading: "Left unbuilt", Line: 7, Tag: "", Text: "dup text"},
	}, res.Dropped)
}

// cfg.StateHeadings.Traps set to the plain string "Traps" (no "#") must
// not overrun to end of file and pool "- debt entry" a second time under
// both headings; it reports once, under its true nearest ancestor.
func Test_finish_reports_a_drop_once_when_a_configured_heading_carries_no_hash(t *testing.T) {
	cfg := config.Default()
	cfg.StateHeadings.Traps = "Traps"

	oldState := strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"Traps", "",
		"- trap entry", "",
		"## Open debts", "",
		"- debt entry", "",
	}, "\n") + "\n"
	mem := newDroppedFixtureFS(t, oldState)
	newState := []byte(strings.Join([]string{
		"## Binding decisions", "",
		"## Left unbuilt", "",
		"Traps", "",
		"- trap entry", "",
		"## Open debts", "",
	}, "\n") + "\n")

	res, err := finishDroppedWithConfig(t, mem, newState, cfg)

	require.NoError(t, err)
	assert.Equal(t, []scaffold.DroppedEntry{
		{Rule: scaffold.DropRuleDebt, Heading: "Open debts", Line: 11, Tag: "", Text: "debt entry"},
	}, res.Dropped)
}
