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
// blocks via a directory at its own temp-sibling name, a chmod-based
// not-writable directory, and a umask-sensitive mode assertion — real
// filesystem mechanics rwfs.Mem does not reproduce. Every other Finish
// test moved onto rwfs.Mem via FinishFS and finishFixtureFS.

// snapshotTree returns the contents of every regular file directly under
// dir, keyed by name.
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
// through stepfile.CompileHandoff.
func (fx finishFixture) handoffPath() string {
	return filepath.Join(fx.featureDir(), "STEP-02"+fx.cfg.HandoffFileSuffix)
}

// newFinishFixture builds a "widgets" feature under t.TempDir() with three
// step files (STEP-01 done, STEP-02 the target, STEP-03 untouched), a
// state file and a specification carrying a progress entry for each step —
// the disk counterpart of newFinishFixtureFS, whose content it matches
// exactly. It returns the fixture together with the handoff and state
// bytes a happy-path Finish("widgets", "STEP-02", ...) call supplies.
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
// the handoff file finish creates.
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

// Control for Test_finish_leaves_no_temp_file_in_the_feature_directory,
// using the same probe against a decoy leftover temp file.
func Test_the_temp_file_sweep_sees_a_temp_file(t *testing.T) {
	fx := newFinishFixture(t)
	decoy := "." + fx.cfg.StateFile + ".brief-tmp"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), decoy), []byte("leftover"), 0o600))

	entries, err := os.ReadDir(fx.featureDir())
	require.NoError(t, err)

	assert.Contains(t, namesOf(entries), decoy, "the probe must see the decoy")
	assert.NotContains(t, fixtureFileNames(fx), decoy, "the fixture's expected set must not already include it")
}

// A directory planted at the state file's own temp sibling blocks the
// second of Finish's four writes. Also proves convergence: clearing the
// obstruction and retrying with the same arguments finishes the sequence.
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

// A directory planted at the step file's own temp sibling blocks the third
// of Finish's four writes — the last write whose failure still leaves the
// step's frontmatter status "open", the sole doneness authority a retry
// relies on.
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

// A directory planted at the specification's temp sibling blocks the last
// of Finish's four writes; the retry below proves convergence past it.
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

func Test_refuses_an_empty_feature_argument_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
}

// Deliberately does not skip when running as root: a require.Error failing
// to fire under root is the signal this test stopped exercising the
// failure it names.
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

// Pins the mode of the one file Finish creates rather than replaces. The
// umask is pinned at 0o022, the permissive default that would let a wrong,
// wider perm through intact rather than being masked away by a stricter
// ambient umask.
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
