package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file holds Finish's disk-only tests: the three writes atomicfile
// blocks via a directory at its own temp-sibling name (state, step file,
// specification — the OS adapter's real rename-into-place mechanism, which
// rwfs.Mem does not reproduce since its own WriteFile is one in-memory
// assignment under a mutex, never a temp file and a rename), the temp-file
// sweep those three tests share a decoy convention with, a chmod-based
// not-writable directory, and a umask-sensitive mode assertion — see
// rwfs/doc.go's own list of guarantees "inherent to a real filesystem and
// not, and cannot economically be, reproduced by Mem". Every other Finish
// test moved onto rwfs.Mem via FinishFS and finishFixtureFS
// (finish_test.go, finish_fixture_fs_test.go and their siblings).

// snapshotTree returns the contents of every regular file directly under
// dir, keyed by name, so a test can prove a refusal left the directory
// byte-identical by comparing two snapshots.
func snapshotTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	snap := make(map[string][]byte, len(entries))

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		snap[e.Name()] = data
	}

	return snap
}

// finishFixture is the on-disk feature newFinishFixture builds, plus the
// two inputs a Finish call under test supplies.
type finishFixture struct {
	root       string
	cfg        config.Config
	newHandoff []byte
	newState   []byte
}

// featureDir returns the path of the fixture's "widgets" feature directory.
func (fx finishFixture) featureDir() string {
	return filepath.Join(fx.root, fx.cfg.FeatureDirectory, "widgets")
}

// stepPath returns the path of one of the fixture's step files.
func (fx finishFixture) stepPath(name string) string {
	return filepath.Join(fx.featureDir(), name)
}

// handoffPath returns the path of the fixture's target step's handoff
// file, deriving the name by literal string concatenation rather than
// through stepfile.CompileHandoff, so a production naming bug cannot hide
// behind the test's own derivation.
func (fx finishFixture) handoffPath() string {
	return filepath.Join(fx.featureDir(), "STEP-02"+fx.cfg.HandoffFileSuffix)
}

// newFinishFixture builds a "widgets" feature under t.TempDir() with three
// step files (STEP-01 done, STEP-02 the target, STEP-03 untouched), a
// state file and a specification carrying a progress entry for each step,
// using fixtureConfig so every field this package reads differs from
// config.Default() — the disk counterpart of newFinishFixtureFS, whose
// content it matches exactly. It returns the fixture together with the
// handoff and state bytes a happy-path Finish("widgets", "STEP-02", ...)
// call supplies.
func newFinishFixture(t *testing.T) finishFixture {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	fx := finishFixture{root: root, cfg: cfg}

	require.NoError(t, os.MkdirAll(fx.featureDir(), 0o755))

	step01 := "---\n" +
		"id: STEP-01\n" +
		"status: done\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-01\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] did the first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n" +
		"\n" +
		"OLD-HANDOFF-01\n"
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-01.md"), []byte(step01), 0o600))

	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(step02Body(cfg)), 0o600))

	step03 := "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-03\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [ ] not done yet\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-03.md"), []byte(step03), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.StateFile), []byte(oldStateBody(cfg)), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01: already done\n" +
		"- [ ] STEP-02: Assemble the thing\n" +
		"- [ ] STEP-03\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"Something else entirely.\n"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.SpecificationFile), []byte(spec), 0o600))

	fx.newHandoff = []byte("NEW-HANDOFF-02\n\n```\n# not a heading\n```\n")
	fx.newState = newStateBody(cfg)

	return fx
}

// fixtureFileNames is the exact set of files newFinishFixture writes plus
// the handoff file finish creates, against which the temp-file sweep below
// checks.
func fixtureFileNames(fx finishFixture) []string {
	return []string{
		fx.cfg.SpecificationFile, fx.cfg.StateFile,
		"STEP-01.md", "STEP-02.md", "STEP-03.md",
		"STEP-02" + fx.cfg.HandoffFileSuffix,
	}
}

func Test_finish_leaves_no_temp_file_in_the_feature_directory(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	entries, readErr := os.ReadDir(fx.featureDir())
	require.NoError(t, readErr)

	assert.ElementsMatch(t, fixtureFileNames(fx), namesOf(entries))
}

// Test_the_temp_file_sweep_sees_a_temp_file is the control for
// Test_finish_leaves_no_temp_file_in_the_feature_directory's claim, and uses
// the exact probe that test does — namesOf(entries) compared against
// fixtureFileNames(fx) — rather than a separate helper: a decoy leftover
// temp file shows up in the directory listing but not in the fixture's
// expected set, which is exactly what would fail the real test's
// assert.ElementsMatch if finish ever left one behind.
func Test_the_temp_file_sweep_sees_a_temp_file(t *testing.T) {
	fx := newFinishFixture(t)
	decoy := "." + fx.cfg.StateFile + ".brief-tmp"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), decoy), []byte("leftover"), 0o600))

	entries, err := os.ReadDir(fx.featureDir())
	require.NoError(t, err)

	assert.Contains(t, namesOf(entries), decoy, "the probe must see the decoy")
	assert.NotContains(t, fixtureFileNames(fx), decoy, "the fixture's expected set must not already include it")
}

// Test_reports_a_state_write_that_cannot_be_committed covers the second of
// Finish's four writes: a directory planted at the state file's own temp
// sibling — the same seam Test_the_temp_file_sweep_sees_a_temp_file uses as
// a decoy — blocks Create's OpenFile outright, before Close is ever reached.
//
// It carries the same convergence proof as the specification and step-file
// siblings below it: the handoff write ahead of the blocked one has landed,
// the state write itself has not (the old state is still on disk), and the
// step file and specification — both after the blocked write in the fixed
// order — are untouched. Clearing the obstruction and retrying with the same
// arguments must then finish the whole sequence.
func Test_reports_a_state_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+fx.cfg.StateFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff write ahead of the blocked state write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, oldStateBody(fx.cfg), string(gotState), "the state write itself was blocked, so the old state must still be on disk")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: open",
		"a write that could not be committed must leave the step retryable")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write must not have run past the blocked state write")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotStateAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotStateAfterRetry))

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_reports_a_step_file_write_that_cannot_be_committed covers the third
// of Finish's four writes: a directory planted at the step file's own temp
// sibling blocks it. This is the doc's own load-bearing boundary — the last
// write whose failure still leaves the step's frontmatter status "open", the
// sole doneness authority a retry relies on; a failure one write later (the
// specification) would leave status already "done".
//
// It asserts the same convergence shape as its siblings: the handoff and
// state writes ahead of the blocked one have landed, the step file itself
// still says status: open, the specification is still unticked, and clearing
// the obstruction and retrying finishes the sequence.
func Test_reports_a_step_file_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+"STEP-02.md"+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff and state writes ahead of the blocked step-file write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotState), "the state write ahead of the blocked one must have landed")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: open",
		"the step-file write itself was blocked, so status must still be open — the last prefix that leaves it that way")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write must not have run past the blocked step-file write")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotStepAfterRetry, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStepAfterRetry), "status: done")

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_reports_a_specification_write_that_cannot_be_committed covers the
// io.StringWriter path the same way, at the last write in the sequence: a
// directory planted at the specification's temp sibling blocks it.
//
// It also carries the only proof in this suite that Finish's crash-
// convergence claim holds past the first write: the handoff, state and step
// writes ahead of the blocked one have already landed, so it asserts each
// of those three lands correctly and the specification does not, then
// clears the obstruction and calls Finish again with the same arguments —
// the retry a crash at this exact point would need — and asserts that
// second call both succeeds and finishes the tick the first one left undone.
func Test_reports_a_specification_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+fx.cfg.SpecificationFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff, state and step-file writes ahead of the blocked specification write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotState), "the state write ahead of the blocked one must have landed")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: done", "the step-file write ahead of the blocked one must have landed")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write itself was blocked, so the progress entry must still be unticked")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing",
		"the retry must finish the tick the blocked write left undone")
}

// Test_refuses_an_unknown_feature_on_finish pins openFeatureDir's own
// ErrNoSuchFeature: "ghost" has no directory at all under the configured
// feature directory. Like the traversal and empty-argument cases below,
// this is decided entirely by the entry-point's open chain, before
// FinishFS is ever reached — FinishFS itself is handed an fsys that
// already names a real, contained feature directory and has no notion of
// "no such feature" to refuse with.
func Test_refuses_an_unknown_feature_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "ghost", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, cfg.FeatureDirectory, "ghost"))
}

func Test_refuses_a_feature_name_that_escapes_the_feature_root_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "../escaped", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, "escaped"))
}

// Test_refuses_an_empty_feature_argument_on_finish pins that an empty
// feature argument is refused the same way a traversal attempt is —
// validFeatureArgument rejects it before openFeatureDir's first open —
// rather than reaching top.OpenRoot("") and surfacing its own opaque
// failure, which names no feature and suggests no fix.
func Test_refuses_an_empty_feature_argument_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
}

// Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable
// deliberately does not skip when running as root, unlike a typical
// permission-bit test: a require.Error failing to fire under root is the
// signal that this test stopped exercising the failure it names, and that
// signal is more valuable here than a clean skip. rwfs.Mem never checks
// permission bits (rwfs/doc.go), so this stays on disk.
func Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable(t *testing.T) {
	fx := newFinishFixture(t)
	t.Cleanup(func() { _ = os.Chmod(fx.featureDir(), 0o755) })
	require.NoError(t, os.Chmod(fx.featureDir(), 0o555))

	before := snapshotTree(t, fx.featureDir())
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)

	var refusal *scaffold.RefusalError
	assert.NotErrorAs(t, err, &refusal, "a mid-sequence write failure must not be a *RefusalError")
	assert.Contains(t, err.Error(), "run 'brief finish widgets STEP-02")
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))
	assert.NoFileExists(t, fx.handoffPath(), "the first of the four writes failing must leave nothing partial")
}

// Test_the_handoff_file_is_created_with_the_same_mode_as_its_siblings pins
// the mode of the one file Finish creates rather than replaces. The
// specification, state and step files are all created by writeExclusive at
// 0o600; the handoff file goes through atomicfile.Create, whose perm
// argument applies only on a fresh create. A wider perm there lands a
// world-readable handoff beside three owner-only siblings. rwfs.Mem never
// applies the process umask (rwfs/doc.go), so this stays on disk.
//
// That a wrong mode then sticks — every later Finish finding an existing
// file and preserving whatever mode the first one chose — is a property of
// atomicfile.Create, proven one layer down by
// Test_Create_keeps_the_existing_files_permissions_when_it_replaces_it.
// This test pins only the mode the file is born with.
//
// The umask is pinned at 0o022 — the permissive default, which would let a
// 0o644 perm through intact — so this fails on a wrong perm rather than
// passing because a stricter ambient umask happened to mask the extra bits
// away. syscall.Umask is process-global and no test in this package calls
// t.Parallel, so nothing else observes the pinned value mid-test.
//
// The state file's mode is read rather than asserted against a constant
// alone: the claim is that the handoff matches the siblings Finish writes
// beside it, and a fixture that somehow created those wider would
// otherwise go unnoticed.
func Test_the_handoff_file_is_created_with_the_same_mode_as_its_siblings(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	handoffInfo, err := os.Stat(fx.handoffPath())
	require.NoError(t, err)

	stateInfo, statErr := os.Stat(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, statErr)

	assert.Equal(t, os.FileMode(0o600), stateInfo.Mode().Perm(),
		"the fixture's own siblings must be owner-only, or the comparison below proves nothing")
	assert.Equal(t, stateInfo.Mode().Perm(), handoffInfo.Mode().Perm(),
		"the handoff file must not be created wider than the siblings Finish writes beside it")
}
