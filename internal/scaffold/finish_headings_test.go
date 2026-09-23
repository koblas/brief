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
// carrying NEW-STATE-ENTRY content, with every heading in missing left out
// entirely — the heading line and its section are both absent, not merely
// emptied, so a test built against it proves absence rather than an empty
// section.
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
// nothing under any of them — the exact shape stateSkeleton (NewFeature)
// writes — so a test built against it proves the heading check triggers on
// presence alone, never on section content.
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

// Test_refuses_a_state_body_missing_a_required_heading pins SCENARIO-19's
// refusal copy directly, against the fixture's own retitled heading ("##
// Gotchas"), proving the text is read from configuration rather than
// hardcoded.
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

// Test_accepts_a_state_body_whose_sections_are_all_empty pins decision 1:
// the trigger is markdown.Section's found return, never section content, so
// a freshly scaffolded state file with every section empty is accepted.
func Test_accepts_a_state_body_whose_sections_are_all_empty(t *testing.T) {
	fx := newFinishFixtureFS(t)
	empty := stateBodyEmptySections(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, empty)

	require.NoError(t, err)
}

// Test_accepts_a_state_body_whose_headings_are_out_of_configured_order
// records decision 2 as a green test, not only as prose: order is not
// enforced, since assemble.stateSections reads every heading by name.
func Test_accepts_a_state_body_whose_headings_are_out_of_configured_order(t *testing.T) {
	fx := newFinishFixtureFS(t)
	shuffled := stateBodyShuffled(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, shuffled)

	require.NoError(t, err)
}

// Test_accepts_the_state_body_new_feature_writes hands Finish the exact
// bytes NewFeature wrote to disk, read back rather than re-derived in the
// test, so the pin is against production's own output. It stays on disk:
// unlike the rest of this file, it exercises NewFeature, NewStep and
// Finish together end to end.
func Test_accepts_the_state_body_new_feature_writes(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	srv := scaffold.NewServer(cfg, root)

	featureRes, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	stateBody, err := os.ReadFile(filepath.Join(featureRes.Path, cfg.StateFile))
	require.NoError(t, err)

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)

	_, err = srv.Finish(context.Background(), "widgets", pattern.ID(1), []byte("handoff body\n"), stateBody)
	require.NoError(t, err)
}

// Test_refuses_a_state_body_whose_headings_are_only_inside_a_fenced_block is
// the measured case that succeeds today: every configured heading's text
// appears, but only inside a fenced code block, so it must still refuse.
// This proves the check reuses markdown.Section, fence-aware, rather than a
// substring scan.
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

// Test_refuses_an_empty_state_body_naming_the_first_configured_heading
// covers the degenerate empty body: no heading is present, so the refusal
// names the first entry in cfg.StateHeadings.Ordered().
func Test_refuses_an_empty_state_body_naming_the_first_configured_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, []byte(""))

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.BindingDecisions)
}

// Test_names_the_first_configured_heading_when_several_are_missing pins
// that the first missing heading, in cfg.StateHeadings.Ordered() order, is
// named when more than one is absent. This test alone is vacuous — it also
// passes when only Ordered()[0] is ever checked — and is paired with
// Test_refuses_a_state_body_missing_only_the_last_configured_heading and a
// reverse-iteration mutation for the real proof.
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

// Test_refuses_a_state_body_missing_only_the_last_configured_heading proves
// the loop covers every position, not only index 0: only the last
// configured heading is missing, so it must be the one named.
func Test_refuses_a_state_body_missing_only_the_last_configured_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.OpenDebts)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, fx.cfg.StateHeadings.OpenDebts)
}

// Test_reports_the_state_cap_before_a_missing_heading pins the heading
// check's position after the cap band: a body that is both over cap and
// carries no headings at all reports ErrOverCap, not the heading sentinel.
func Test_reports_the_state_cap_before_a_missing_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overCapNoHeadings := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCapNoHeadings)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrMissingStateHeading)
}

// Test_reports_the_state_s_unclosed_fence_before_a_missing_heading pins the
// heading check's position after checkArgumentFence: a body whose fence
// never closes reports ErrUnterminatedFence, not the heading sentinel — an
// open fence leaves the headings unreadable in the first place.
func Test_reports_the_state_s_unclosed_fence_before_a_missing_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	unterminated := []byte("```\nunterminated\n")

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, unterminated)

	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)
	assert.NotErrorIs(t, err, scaffold.ErrMissingStateHeading)
}

// Test_a_state_body_missing_a_heading_on_a_done_step_reports_the_heading_not_the_re_finish_refusal
// pins the heading check's position ahead of (refinish).verdict: a
// headingless state body aimed at an already-done step reports the missing
// heading, not ErrAlreadyFinished, even though the body also differs from
// what is recorded.
func Test_a_state_body_missing_a_heading_on_a_done_step_reports_the_heading_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

// Test_a_refused_missing_heading_leaves_every_file_byte_identical pairs the
// Mem snapshot probe against a control arm proving it can see a write —
// Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish, in
// finish_idempotent_test.go — not duplicated here. Mem.Snapshot includes
// each entry's ModTime, which advances on every write including a
// byte-identical rewrite, so equality alone already proves nothing was
// touched, not merely that whatever was written reproduced the same bytes.
func Test_a_refused_missing_heading_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.Equal(t, before, fx.mem.Snapshot())
}
