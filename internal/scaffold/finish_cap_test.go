package scaffold_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodyOfLines returns a handoff or state body of exactly n lines, each
// distinct, with a trailing newline. It carries no configured heading; see
// stateBodyOfLines for a state body that must also pass the heading check.
func bodyOfLines(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

// stateBodyOfLines returns a state body of exactly n lines that carries
// cfg's four configured headings, each with distinct content, padded with
// distinct filler lines. n must be at least 12 (three lines per heading).
func stateBodyOfLines(cfg config.Config, n int) []byte {
	var lines []string

	for i, h := range cfg.StateHeadings.Ordered() {
		lines = append(lines, h, "", fmt.Sprintf("content %d", i))
	}

	for i := 0; len(lines) < n; i++ {
		lines = append(lines, fmt.Sprintf("filler line %d", i))
	}

	return []byte(strings.Join(lines[:n], "\n") + "\n")
}

func Test_refuses_a_handoff_one_line_over_the_configured_cap(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overCap := bodyOfLines(fx.cfg.HandoffCapLines + 1)

	_, err := fx.finish(t, "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.HandoffSource, refusal.Path)
	assert.Equal(t, 0, refusal.Line)
	assert.Equal(t,
		"<handoff>: handoff is 11 lines, over the cap of 10; cut the handoff to 10 lines or fewer, "+
			"or raise handoff-cap-lines in .brief.yaml, and retry",
		err.Error())
}

func Test_accepts_a_handoff_of_exactly_the_configured_cap(t *testing.T) {
	fx := newFinishFixtureFS(t)
	atCap := bodyOfLines(fx.cfg.HandoffCapLines)

	_, err := fx.finish(t, "STEP-02", atCap, fx.newState)

	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

func Test_accepts_a_handoff_of_exactly_the_configured_cap_with_no_trailing_newline(t *testing.T) {
	fx := newFinishFixtureFS(t)
	atCap := bodyOfLines(fx.cfg.HandoffCapLines)
	atCap = []byte(strings.TrimSuffix(string(atCap), "\n"))

	_, err := fx.finish(t, "STEP-02", atCap, fx.newState)

	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

// Control arm proving the Mem snapshot probe can see a write:
// Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish (finish_idempotent_test.go).
func Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	overCap := bodyOfLines(fx.cfg.HandoffCapLines + 1)

	_, err := fx.finish(t, "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	overCap := bodyOfLines(fx.cfg.HandoffCapLines + 1)

	_, err := fx.finish(t, "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

func Test_refuses_a_state_body_one_line_over_the_configured_cap(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overCap := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCap)

	require.ErrorIs(t, err, scaffold.ErrOverCap)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.StateSource, refusal.Path)
	assert.Equal(t, 0, refusal.Line)
	assert.Equal(t,
		"<state>: state is 21 lines, over the cap of 20; cut the state to 20 lines or fewer, "+
			"or raise state-cap-lines in .brief.yaml, and retry",
		err.Error())
}

func Test_accepts_a_state_body_of_exactly_the_configured_cap(t *testing.T) {
	fx := newFinishFixtureFS(t)
	atCap := stateBodyOfLines(fx.cfg, fx.cfg.StateCapLines)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, atCap)

	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

func Test_accepts_a_state_body_of_exactly_the_configured_cap_with_no_trailing_newline(t *testing.T) {
	fx := newFinishFixtureFS(t)
	atCap := stateBodyOfLines(fx.cfg, fx.cfg.StateCapLines)
	atCap = []byte(strings.TrimSuffix(string(atCap), "\n"))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, atCap)

	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

func Test_a_refused_over_cap_state_body_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	overCap := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCap)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overHandoff := bodyOfLines(fx.cfg.HandoffCapLines + 1)
	overState := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", overHandoff, overState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.HandoffSource, refusal.Path)
	assert.Contains(t, err.Error(), "handoff is")
}

func Test_reports_the_state_cap_before_the_state_s_unclosed_fence(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overState := append(bodyOfLines(fx.cfg.StateCapLines), []byte("```\n")...)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrUnterminatedFence)
}

func Test_an_over_cap_state_body_on_a_done_step_reports_the_cap_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	overCap := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCap)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}
