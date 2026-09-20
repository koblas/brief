package scaffold_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overCapHandoff returns a handoff body of exactly n lines, each distinct
// so a truncation bug cannot hide behind a repeated one, with a trailing
// newline.
func overCapHandoff(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

func Test_refuses_a_handoff_one_line_over_the_configured_cap(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	overCap := overCapHandoff(fx.cfg.HandoffCapLines + 1)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", overCap, fx.newState)

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
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	atCap := overCapHandoff(fx.cfg.HandoffCapLines)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", atCap, fx.newState)

	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

func Test_accepts_a_handoff_of_exactly_the_configured_cap_with_no_trailing_newline(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	atCap := overCapHandoff(fx.cfg.HandoffCapLines)
	atCap = []byte(strings.TrimSuffix(string(atCap), "\n"))

	err := srv.Finish(context.Background(), "widgets", "STEP-02", atCap, fx.newState)

	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(atCap), string(got))
}

// Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical pairs
// the snapshot probe with the modification-time probe: a byte-snapshot
// alone would still pass a refusal taken after a byte-identical rewrite
// (see Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish and
// Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges
// in finish_idempotent_test.go for the control arms both probes already
// have — not duplicated here).
func Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishFixture(t)
	names := []string{"STEP-02.md", fx.cfg.StateFile, fx.cfg.SpecificationFile}
	pinModTimes(t, fx.featureDir(), names, pinnedModTime)
	before := snapshotTree(t, fx.featureDir())
	srv := scaffold.NewServer(fx.cfg, fx.root)
	overCap := overCapHandoff(fx.cfg.HandoffCapLines + 1)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))

	after := modTimes(t, fx.featureDir(), names)
	for _, name := range names {
		assert.True(t, after[name].Equal(pinnedModTime), "%s mtime moved on a refused over-cap finish", name)
	}
}

// Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal
// pins the ordering decision this scenario takes: the cap check runs in
// Finish's argument band, ahead of (refinish).verdict, so an over-cap
// handoff aimed at an already-done step reports the cap rather than
// ErrAlreadyFinished — even though the supplied handoff also differs from
// the one recorded on disk.
func Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal(t *testing.T) {
	fx := newFinishedFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	overCap := overCapHandoff(fx.cfg.HandoffCapLines + 1)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", overCap, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOverCap)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}
