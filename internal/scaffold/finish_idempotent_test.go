package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinnedModTime is the fixed, whole-second past timestamp every mtime
// assertion in this file chtimes onto before a second Finish call: any
// write at all, at any filesystem mtime granularity, moves a file's
// ModTime to now — an hour away — so equality against this constant cannot
// pass by accident. A stat-before/stat-after delta cannot make that claim,
// since two writes inside one filesystem tick look unchanged.
var pinnedModTime = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

// newFinishedFixture builds newFinishFixture's "widgets" feature and runs
// one Finish call with its handoff and state, so every test below opens on
// a step that is already done and shows only its own divergence from that
// baseline.
func newFinishedFixture(t *testing.T) finishFixture {
	t.Helper()

	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	return fx
}

// pinModTimes sets the access and modification time of each named file
// directly under dir to when.
func pinModTimes(t *testing.T, dir string, names []string, when time.Time) {
	t.Helper()

	for _, name := range names {
		require.NoError(t, os.Chtimes(filepath.Join(dir, name), when, when))
	}
}

// modTimes returns the current modification time of each named file
// directly under dir, keyed by name.
func modTimes(t *testing.T, dir string, names []string) map[string]time.Time {
	t.Helper()

	times := make(map[string]time.Time, len(names))

	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err)

		times[name] = info.ModTime()
	}

	return times
}

// readFileString reads path's full contents as a string. Kept as its own
// helper, rather than inlining os.ReadFile at each call site, so that the
// tests below which rewrite a fixture file — reading it, editing one
// line, writing the result back — do so through two distinct calls rather
// than one straight-line read-derive-write in the same function, which is
// exactly the shape gosec's taint analysis flags as a path-traversal risk
// even though path here is always a fixed t.TempDir() location.
func readFileString(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}

// Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it is the
// arm that reddens if the handoff conjunct is dropped from identity;
// SCENARIO-16 later inverts it into a refusal over the same conjunct.
func Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it(t *testing.T) {
	fx := newFinishedFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	differentHandoff := []byte("DIFFERENT-HANDOFF-02\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", differentHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)

	assert.Equal(t, string(differentHandoff), string(got))
}

// Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it
// is the single-variable proof that "the handoff file exists" is part of
// the identity conjunct, not merely "its bytes match": deleting it before
// an otherwise-identical re-finish must still write, covering the crash-
// after-state retry and a migrated step alike.
func Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it(t *testing.T) {
	fx := newFinishedFixture(t)
	require.NoError(t, os.Remove(fx.handoffPath()))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(got))
}

func Test_re_finishing_a_done_step_with_a_different_state_body_replaces_it(t *testing.T) {
	fx := newFinishedFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	differentState := []byte("DIFFERENT-STATE-BODY\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, differentState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)

	assert.Equal(t, string(differentState), string(got))
}

func Test_re_finishing_a_done_step_whose_progress_entry_was_un_ticked_re_ticks_it(t *testing.T) {
	fx := newFinishedFixture(t)
	specPath := filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile)
	spec := readFileString(t, specPath)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	require.NoError(t, os.WriteFile(specPath, []byte(unticked), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(specPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [x] STEP-02: Assemble the thing")
}

// Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges
// is SCENARIO-16's control arm for the mtime probe: it diverges on the
// specification's checkbox, not on a caller input, so it stays valid once
// SCENARIO-16 turns differing-input divergence into a refusal.
func Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges(t *testing.T) {
	fx := newFinishedFixture(t)
	specPath := filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile)
	spec := readFileString(t, specPath)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	require.NoError(t, os.WriteFile(specPath, []byte(unticked), 0o600))
	names := []string{"STEP-02.md", fx.cfg.StateFile, fx.cfg.SpecificationFile, "STEP-02" + fx.cfg.HandoffFileSuffix}
	pinModTimes(t, fx.featureDir(), names, pinnedModTime)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	after := modTimes(t, fx.featureDir(), names)
	assert.False(t, after["STEP-02.md"].Equal(pinnedModTime), "step file kept the pinned mtime")
	assert.False(t, after[fx.cfg.StateFile].Equal(pinnedModTime), "state file kept the pinned mtime")
	assert.False(t, after[fx.cfg.SpecificationFile].Equal(pinnedModTime), "specification kept the pinned mtime")
	assert.False(t, after["STEP-02"+fx.cfg.HandoffFileSuffix].Equal(pinnedModTime), "handoff file kept the pinned mtime")
}

func Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_modification_time(t *testing.T) {
	fx := newFinishedFixture(t)
	names := []string{"STEP-02.md", fx.cfg.StateFile, fx.cfg.SpecificationFile, "STEP-02" + fx.cfg.HandoffFileSuffix}
	pinModTimes(t, fx.featureDir(), names, pinnedModTime)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	after := modTimes(t, fx.featureDir(), names)
	assert.True(t, after["STEP-02.md"].Equal(pinnedModTime), "step file mtime moved")
	assert.True(t, after[fx.cfg.StateFile].Equal(pinnedModTime), "state file mtime moved")
	assert.True(t, after[fx.cfg.SpecificationFile].Equal(pinnedModTime), "specification mtime moved")
	assert.True(t, after["STEP-02"+fx.cfg.HandoffFileSuffix].Equal(pinnedModTime), "handoff file mtime moved")
}

func Test_re_finishing_a_done_step_with_the_same_inputs_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishedFixture(t)
	before := snapshotTree(t, fx.featureDir())
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))
}

// Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk
// reverts only the status line, leaving the handoff, state file and ticked
// checkbox untouched, so this is the single-variable proof that the
// fm.Done() gate — not a step-body byte comparison — is what forces the
// write. Identity deliberately carries no step-body comparison: adding one
// back would make this test stop discriminating the gate, since a
// reverted status line alone would then force the write for a reason
// other than fm.Done().
func Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk(t *testing.T) {
	fx := newFinishedFixture(t)
	stepPath := fx.stepPath("STEP-02.md")
	body := readFileString(t, stepPath)
	reverted := strings.Replace(body, "status: done\n", "status: open\n", 1)
	require.NoError(t, os.WriteFile(stepPath, []byte(reverted), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(stepPath)
	require.NoError(t, readErr)

	fm, _, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)
	assert.True(t, fm.Done())
}
