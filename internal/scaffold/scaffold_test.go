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

// fixtureConfig returns a Config whose every field this package reads
// differs from config.Default(), so a hardcoded default cannot pass a test
// built against it.
func fixtureConfig() config.Config {
	cfg := config.Default()
	cfg.FeatureDirectory = "specs"
	cfg.ProgressHeading = "## Progress"
	cfg.ChecklistHeading = "## Fixture Checklist"
	cfg.SpecificationFile = "SPEC.md"
	cfg.StateFile = "NOTES.md"
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffFileSuffix = ".fixture-handoff.md"
	cfg.StateHeadings = config.StateHeadings{
		BindingDecisions: "## Decisions Fixture",
		LeftUnbuilt:      "## Left Fixture",
		Traps:            "## Gotchas",
		OpenDebts:        "## Debts Fixture",
	}

	return cfg
}

func Test_creates_the_feature_directory_under_the_configured_feature_directory(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "widgets")

	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(root, "specs", "widgets"))
}

func Test_returns_the_path_of_the_created_feature_directory(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	path, err := srv.NewFeature(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "specs", "widgets"), path)
}

func Test_writes_the_specification_skeleton_with_the_configured_progress_heading_and_nothing_under_it(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(root, "specs", "widgets", "SPEC.md"))
	require.NoError(t, err)
	assert.Equal(t, "# widgets\n\n## Progress\n", string(got))
}

func Test_writes_the_state_file_with_the_four_configured_headings_and_nothing_under_them(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(root, "specs", "widgets", "NOTES.md"))
	require.NoError(t, err)
	assert.Equal(t, "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n", string(got))
}

func Test_creates_only_the_specification_and_the_state_file(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	entries, err := os.ReadDir(filepath.Join(root, "specs", "widgets"))
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.ElementsMatch(t, []string{"SPEC.md", "NOTES.md"}, names)
}

func Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	sentinel := "# handwritten by a person"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(sentinel), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewFeature(context.Background(), "widgets")

	require.Error(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Equal(t, sentinel, string(got))
}

func Test_returns_an_error_when_the_feature_name_escapes_the_feature_root(t *testing.T) {
	root := t.TempDir()
	srv := scaffold.NewServer(fixtureConfig(), root)

	_, err := srv.NewFeature(context.Background(), "../escaped")

	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(root, "escaped"))
}

// Test_the_scaffolded_files_are_all_created_owner_only pins the mode
// writeExclusive creates the specification, state and step files with.
//
// This is the other half of a claim the scaffold makes in two places.
// replace.go passes atomicfile.Create a 0o600 perm so the handoff file --
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

	featureDir, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	stepPath, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	for _, path := range []string{
		filepath.Join(featureDir, cfg.SpecificationFile),
		filepath.Join(featureDir, cfg.StateFile),
		stepPath,
	} {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr, path)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"%s must be created owner-only, so the handoff file written beside it matches", path)
	}
}
