package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateBodyMissingHeadings returns cfg's four configured headings, each
// carrying content, with every heading in missing left out entirely — the
// heading line and its section both absent, not merely emptied.
func stateBodyMissingHeadings(cfg config.Config, missing ...string) []byte {
	skip := make(map[string]bool, len(missing))
	for _, h := range missing {
		skip[h] = true
	}

	var body strings.Builder

	for _, h := range cfg.StateHeadings.Ordered() {
		if skip[h] {
			continue
		}

		body.WriteString(h + "\n\nNEW-STATE-ENTRY\n\n")
	}

	return []byte(body.String())
}

// stateBodyEmptySections returns cfg's four configured headings with
// nothing under any of them — the exact shape stateSkeleton writes.
func stateBodyEmptySections(cfg config.Config) []byte {
	var body strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		body.WriteString(h + "\n\n")
	}

	return []byte(body.String())
}

// stateBodyShuffled returns cfg's four configured headings, each carrying
// content, in an order that differs from cfg.StateHeadings.Ordered().
func stateBodyShuffled(cfg config.Config) []byte {
	o := cfg.StateHeadings.Ordered()
	shuffled := []string{o[2], o[0], o[3], o[1]}

	var body strings.Builder
	for _, h := range shuffled {
		body.WriteString(h + "\n\nNEW-STATE-ENTRY\n\n")
	}

	return []byte(body.String())
}

func Test_refuses_a_state_body_missing_a_required_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	missingGotchas := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missingGotchas)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t,
		`<state>: state is missing the "## Gotchas" section; add a "## Gotchas" heading to the state body — an empty section is valid — and retry`,
		err.Error())
	assert.Equal(t, 0, refusal.Line, "a missing heading has no line")
}

func Test_accepts_a_state_body_whose_sections_are_all_empty(t *testing.T) {
	fx := newFinishFixtureFS(t)
	empty := stateBodyEmptySections(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, empty)

	require.NoError(t, err)
}

func Test_accepts_a_state_body_whose_headings_are_out_of_configured_order(t *testing.T) {
	fx := newFinishFixtureFS(t)
	shuffled := stateBodyShuffled(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, shuffled)

	require.NoError(t, err)
}

// Hands Finish the exact bytes NewFeature wrote to disk, read back rather
// than re-derived. Stays on disk: exercises NewFeature, NewStep and Finish
// together end to end.
func Test_accepts_the_state_body_new_feature_writes(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	srv := scaffold.NewServer(cfg, root)

	featureRes, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	stepRes, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	stateBody, err := os.ReadFile(filepath.Join(featureRes.Path, cfg.StateFile))
	require.NoError(t, err)

	stepBody, err := os.ReadFile(stepRes.Path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(stepRes.Path, append(stepBody, []byte("\n- [x] done\n")...), 0o600)) //nolint:gosec // stepRes.Path is t.TempDir()-rooted, built by NewStep itself

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)

	_, err = srv.Finish(context.Background(), "widgets", pattern.ID(1), []byte("handoff body\n"), stateBody)
	require.NoError(t, err)
}

func Test_refuses_a_state_body_whose_headings_are_only_inside_a_fenced_block(t *testing.T) {
	fx := newFinishFixtureFS(t)

	var body strings.Builder
	body.WriteString("```\n")
	for _, h := range fx.cfg.StateHeadings.Ordered() {
		body.WriteString(h + "\n\nNEW-STATE-ENTRY\n\n")
	}
	body.WriteString("```\n")
	fenced := []byte(body.String())

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fenced)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.BindingDecisions)
}

func Test_refuses_an_empty_state_body_naming_the_first_configured_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, []byte(""))

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.BindingDecisions)
}

// Paired with Test_refuses_a_state_body_missing_only_the_last_configured_heading:
// alone this test is vacuous, since it also passes were only Ordered()[0]
// ever checked.
func Test_names_the_first_configured_heading_when_several_are_missing(t *testing.T) {
	fx := newFinishFixtureFS(t)
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.BindingDecisions, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.BindingDecisions)
	assert.NotContains(t, refusal.Problem, fx.cfg.StateHeadings.Traps)
}

func Test_refuses_a_state_body_missing_only_the_last_configured_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.OpenDebts)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.OpenDebts)
}

func Test_reports_the_state_cap_before_a_missing_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overCapNoHeadings := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCapNoHeadings)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrMissingStateHeading)
}

func Test_reports_the_state_s_unclosed_fence_before_a_missing_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	unterminated := []byte("```\nunterminated\n")

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, unterminated)

	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)
	assert.NotErrorIs(t, err, scaffold.ErrMissingStateHeading)
}

func Test_a_state_body_missing_a_heading_on_a_done_step_reports_the_heading_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

// Mem.Snapshot includes each entry's ModTime, which advances on every
// write including a byte-identical rewrite, so equality here proves
// nothing was touched at all.
func Test_a_refused_missing_heading_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.Equal(t, before, fx.mem.Snapshot())
}
