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
// distinct so a truncation bug cannot hide behind a repeated one, with a
// trailing newline. It carries no configured heading, so it is only used
// where the state argument's caps or fence are under test, never where it
// must also pass SCENARIO-19's heading check — see stateBodyOfLines for
// that case.
func bodyOfLines(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

// stateBodyOfLines returns a state body of exactly n lines that carries
// cfg's four configured headings, each with distinct content, padded with
// distinct filler lines after the last one so a truncation bug cannot hide
// behind a repeated line — bodyOfLines's counterpart for a state argument
// that must also pass SCENARIO-19's heading check. n must be at least 12
// (three lines per heading); every caller in this package uses cfg's own
// StateCapLines or one more, both comfortably above that floor.
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

// Test_accepts_a_handoff_of_exactly_the_configured_cap pins the boundary:
// a body of exactly cfg.HandoffCapLines lines is accepted, both with and
// without a trailing newline.
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

// Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical pairs
// the Mem snapshot probe with the control arm proving it can see a write
// (Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish, in
// finish_idempotent_test.go — not duplicated here).
func Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	overCap := bodyOfLines(fx.cfg.HandoffCapLines + 1)

	_, err := fx.finish(t, "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.Equal(t, before, fx.mem.Snapshot())
}

// Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal
// pins the ordering decision this scenario takes: the cap check runs in
// Finish's argument band, ahead of (refinish).verdict, so an over-cap
// handoff aimed at an already-done step reports the cap rather than
// ErrAlreadyFinished — even though the supplied handoff also differs from
// the one recorded on disk.
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

// Test_accepts_a_state_body_of_exactly_the_configured_cap pins the
// boundary: a body of exactly cfg.StateCapLines lines is accepted, both
// with and without a trailing newline.
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

// Test_a_refused_over_cap_state_body_leaves_every_file_byte_identical pairs
// the Mem snapshot probe with the control arm, exactly as
// Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical does for
// the handoff cap.
func Test_a_refused_over_cap_state_body_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()
	overCap := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCap)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.Equal(t, before, fx.mem.Snapshot())
}

// Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps
// pins the cap band's order: handoff checked before state, matching the
// write order and R14a's "names the first thing wrong". This test is
// vacuous without mutation (d) — swapping the two checkArgumentCap call
// sites — since it also passes were the state check deleted entirely.
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

// Test_reports_the_state_cap_before_the_state_s_unclosed_fence pins the cap
// band's position ahead of checkArgumentFence: a state body that is both
// over cap and opens a fence it never closes reports the cap, not the
// fence.
func Test_reports_the_state_cap_before_the_state_s_unclosed_fence(t *testing.T) {
	fx := newFinishFixtureFS(t)
	overState := append(bodyOfLines(fx.cfg.StateCapLines), []byte("```\n")...)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrUnterminatedFence)
}

// Test_an_over_cap_state_body_on_a_done_step_reports_the_cap_not_the_re_finish_refusal
// is the state twin of
// Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal:
// the cap band runs ahead of (refinish).verdict, so an over-cap state
// aimed at an already-done step reports the cap rather than
// ErrAlreadyFinished, even though the supplied state also differs from the
// one recorded on disk.
func Test_an_over_cap_state_body_on_a_done_step_reports_the_cap_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	overCap := bodyOfLines(fx.cfg.StateCapLines + 1)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, overCap)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}
