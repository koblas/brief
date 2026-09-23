package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file holds NewFeature's disk-only tests: validateFeatureName runs
// inside the NewFeature entry point itself, before NewFeatureFS or any
// filesystem call, so every refusal here is decided without touching disk
// at all — there is no FS-taking core to redirect onto rwfs.Mem for it,
// only the real entry point. The owner-only permission test stays on disk
// for the usual reason: rwfs.Mem never applies the process umask
// (rwfs/doc.go).

func Test_refuses_a_feature_name_containing_whitespace(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay ments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
	assert.ErrorContains(t, err, `"pay ments"`)
}

func Test_refuses_a_feature_name_with_a_leading_space(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), " payments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_a_feature_name_with_a_trailing_space(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "payments ")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_a_feature_name_containing_a_tab(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay\tments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_a_feature_name_containing_a_line_feed(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay\nments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_a_feature_name_containing_a_carriage_return(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay\rments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_a_feature_name_containing_a_non_breaking_space(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay ments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_refuses_an_empty_feature_name(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
}

func Test_creates_nothing_at_all_when_the_name_is_refused(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "pay ments")

	require.ErrorIs(t, err, scaffold.ErrInvalidFeatureName)
	assert.NoDirExists(t, filepath.Join(root, "specs", "pay ments"))
	assert.NoDirExists(t, filepath.Join(root, "specs"))
}

// Test_the_scaffolded_files_are_all_created_owner_only pins the mode
// writeExclusive creates the specification, state and step files with.
//
// This is the other half of a claim the scaffold makes in two places.
// replace.go passes rwfs.FS.WriteFile a 0o600 perm so the handoff file --
// the only file Finish creates rather than replaces -- is born no wider
// than the files beside it, and Test_the_handoff_file_is_created_with_the_
// same_mode_as_its_siblings pins that against its fixture's state file. But
// a fixture's mode is chosen by the fixture: without this test, changing
// writeExclusive's own constant to 0o644 leaves the whole suite green while
// real trees grow three 0o644 files beside a 0o600 handoff -- the same
// inconsistency, reintroduced from the other end.
//
// The umask is pinned only so the assertion reads the same way as its
// siblings in this repo; 0o600 carries no bits a conventional umask strips,
// so unlike the atomicfile fresh-create tests this one is umask-stable
// either way.
func Test_the_scaffolded_files_are_all_created_owner_only(t *testing.T) {
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	cfg := fixtureConfig()
	root := t.TempDir()
	srv := scaffold.NewServer(cfg, root)

	featureRes, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	stepRes, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	for _, path := range []string{
		filepath.Join(featureRes.Path, cfg.SpecificationFile),
		filepath.Join(featureRes.Path, cfg.StateFile),
		stepRes.Path,
	} {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr, path)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"%s must be created owner-only, so the handoff file written beside it matches", path)
	}
}
