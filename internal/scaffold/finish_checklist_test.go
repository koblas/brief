package scaffold_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// step02BodyWithOpenItem returns newFinishFixture's STEP-02 body with its
// checklist's second item left unticked, landing at line 13.
func step02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "open", "second thing")
}

// doneStep02BodyWithOpenItem is step02BodyWithOpenItem with its frontmatter
// already saying done.
func doneStep02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "done", "second thing")
}

// bareOpenItemStep02Body is step02BodyWithOpenItem with its open item
// carrying no text at all, landing at the same line 13.
func bareOpenItemStep02Body(cfg config.Config) string {
	return openItemStep02Body(cfg, "open", "")
}

// noHeadingStep02Body is step02BodyWithOpenItem with its checklist heading
// renamed away, so no line in the body matches cfg.ChecklistHeading.
func noHeadingStep02Body(cfg config.Config) string {
	return strings.Replace(step02BodyWithOpenItem(cfg), cfg.ChecklistHeading, "## Not The Checklist", 1)
}

// zeroItemStep02Body is newFinishFixtureFS's STEP-02 body with an empty
// checklist section, its heading landing at line 10.
func zeroItemStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

// fencedZeroItemStep02Body is zeroItemStep02Body with a fenced block of
// ticked items under the checklist heading, which the counter must not see.
func fencedZeroItemStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"```\n" +
		"- [x] fenced decoy\n" +
		"```\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

// crlfZeroItemStep02Body is zeroItemStep02Body with every line after its
// LF frontmatter converted to CRLF.
func crlfZeroItemStep02Body(cfg config.Config) string {
	frontmatter := "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n"
	rest := "\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"## Fixture Handoff" + "\n"

	return frontmatter + strings.ReplaceAll(rest, "\n", "\r\n")
}

// frontmatterCommentMatchesHeadingStep02Body carries, inside its YAML
// frontmatter, a comment line byte-identical to cfg.ChecklistHeading; the
// real checklist below it holds one ticked item.
func frontmatterCommentMatchesHeadingStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		cfg.ChecklistHeading + "\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

// frontmatterOpenFenceStep02Body carries a "notes: |" YAML block scalar
// whose indented "```" line looks, to a scanner naive of YAML syntax, like
// a fence opened and never closed; the real checklist below it holds an
// unticked second item, landing at line 15.
func frontmatterOpenFenceStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"notes: |\n" +
		"  ```\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"- [ ] second thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

// doneNoHeadingStep02Body is noHeadingStep02Body with its frontmatter
// already saying done.
func doneNoHeadingStep02Body(cfg config.Config) string {
	return strings.Replace(noHeadingStep02Body(cfg), "status: open\n", "status: done\n", 1)
}

// doneZeroItemStep02Body is zeroItemStep02Body with its frontmatter already
// saying done.
func doneZeroItemStep02Body(cfg config.Config) string {
	return strings.Replace(zeroItemStep02Body(cfg), "status: open\n", "status: done\n", 1)
}

// acceptanceAbsentStep02Body is step02BodyWithOpenItem's shape, one item
// ticked, carrying no acceptance heading at all.
func acceptanceAbsentStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

// acceptanceWhitespaceStep02Body is acceptanceAbsentStep02Body with
// cfg.AcceptanceHeading added, its section holding only whitespace.
func acceptanceWhitespaceStep02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.AcceptanceHeading + "\n" +
		"\n" +
		"   \n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

func openItemStep02Body(cfg config.Config, status, item string) string {
	itemLine := "- [ ]"
	if item != "" {
		itemLine = "- [ ] " + item
	}

	return "---\n" +
		"id: STEP-02\n" +
		"status: " + status + "\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		itemLine + "\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

func Test_finish_refuses_a_step_with_an_open_checklist_item(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stepPath("STEP-02.md"), refusal.Path)
	assert.Equal(t, 13, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item "second thing" is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())

	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_finish_refuses_an_open_step_with_no_checklist_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", noHeadingStep02Body(fx.cfg))
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnplannedStep)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 0, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`: no "## Fixture Checklist" heading found; add it with the step's checklist items, tick them, and retry`,
		err.Error())

	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_finish_refuses_an_open_step_whose_checklist_holds_no_items(t *testing.T) {
	cases := []struct {
		name string
		body func(config.Config) string
	}{
		{name: "empty section", body: zeroItemStep02Body},
		{name: "ticked items sit only inside a fenced block", body: fencedZeroItemStep02Body},
		{name: "CRLF line endings after the frontmatter", body: crlfZeroItemStep02Body},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newFinishFixtureFS(t)
			putStepFS(t, fx, "STEP-02.md", c.body(fx.cfg))
			before := fx.mem.Snapshot()

			_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

			require.ErrorIs(t, err, scaffold.ErrUnplannedStep)

			var refusal *scaffold.RefusalError
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, 10, refusal.Line)
			assert.Equal(t,
				fx.stepPath("STEP-02.md")+
					`:10: "## Fixture Checklist" has 0 checklist items, needs at least 1; add the step's items as "- [x]" lines once done, and retry`,
				err.Error())

			assert.Equal(t, before, fx.mem.Snapshot())
		})
	}
}

// Runs NewFeature, NewStep and FinishFS end to end, so the refused line is
// pinned against the scaffold's own output rather than a hand-built fixture.
func Test_finish_refuses_a_freshly_scaffolded_step(t *testing.T) {
	top := newFeatureRootFS(t).(*rwfs.Mem) //nolint:forcetypeassert // newFeatureRootFS always returns *rwfs.Mem
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	newStepFS(t, srv, view, cfg, "widgets")

	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	handoffPattern, err := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, err)

	before := top.Snapshot()

	_, err = srv.FinishFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", "STEP-01",
		[]byte("HANDOFF"), stateBodyEmptySections(cfg), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrUnplannedStep)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 11, refusal.Line)

	assert.Equal(t, before, top.Snapshot())
}

func Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_empty_checklist(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", zeroItemStep02Body(fx.cfg))
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrUnplannedStep)
}

func Test_finish_accepts_an_open_step_whose_acceptance_section_is_empty_or_absent(t *testing.T) {
	cases := []struct {
		name string
		body func(config.Config) string
	}{
		{name: "no acceptance heading", body: acceptanceAbsentStep02Body},
		{name: "whitespace-only acceptance section", body: acceptanceWhitespaceStep02Body},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newFinishFixtureFS(t)
			putStepFS(t, fx, "STEP-02.md", c.body(fx.cfg))

			_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

			require.NoError(t, err)

			got, readErr := fx.mem.ReadFile("STEP-02.md")
			require.NoError(t, readErr)
			assert.Contains(t, string(got), "status: done")
		})
	}
}

func Test_finish_ignores_an_unchecked_item_outside_the_checklist_section(t *testing.T) {
	fx := newFinishFixtureFS(t)
	body := "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		fx.cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"- [ ] not this section's\n"
	putStepFS(t, fx, "STEP-02.md", body)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

func Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	require.NoError(t, fx.mem.Remove(fx.cfg.SpecificationFile))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrMalformedFeature)
}

func Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrOpenChecklistItem)
}

func Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", doneStep02BodyWithOpenItem(fx.cfg))
	divergentHandoff := []byte("DIFFERENT-HANDOFF\n")

	_, err := fx.finish(t, "STEP-02", divergentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

func Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", bareOpenItemStep02Body(fx.cfg))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())
}

func Test_finish_accepts_a_step_whose_frontmatter_comment_matches_the_checklist_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", frontmatterCommentMatchesHeadingStep02Body(fx.cfg))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

func Test_finish_refuses_an_unticked_item_despite_an_unclosed_fence_in_frontmatter(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", frontmatterOpenFenceStep02Body(fx.cfg))
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 15, refusal.Line)

	assert.Equal(t, before, fx.mem.Snapshot())
}
