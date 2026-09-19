package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_writes_the_step_file_with_frontmatter_a_title_an_empty_checklist_and_a_handoff_anchor(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	path, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)

	want := "---\n" +
		"id: STEP-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-01\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		cfg.HandoffHeading + "\n"
	assert.Equal(t, want, string(got))
}

func Test_appends_the_progress_entry_under_the_progress_heading_when_the_list_is_empty(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(root, "specs", "widgets", cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Equal(t, "# widgets\n\n## Progress\n\n- [ ] STEP-01\n", string(got))
}

func Test_returns_the_path_of_the_created_step_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	path, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "specs", "widgets", "STEP-01.md"), path)
}

func Test_leaves_no_temp_file_in_the_feature_directory(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	entries, readErr := os.ReadDir(filepath.Join(root, "specs", "widgets"))
	require.NoError(t, readErr)

	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile, "STEP-01.md"}, namesOf(entries))
}

// Test_NewStep_finds_a_progress_heading_terminated_by_a_carriage_return
// reproduces the reviewer's finding directly: insertProgressEntry's
// heading match used to right-trim only " \t", so a CRLF specification
// whose progress heading line ends "\r\n" never matched and NewStep
// refused with ErrNoProgressHeading on every CRLF feature.
func Test_NewStep_finds_a_progress_heading_terminated_by_a_carriage_return(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	spec := "# widgets\r\n\r\n" + cfg.ProgressHeading + "\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [ ] STEP-01")
}

func Test_numbers_the_next_step_from_the_highest_existing_step_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-03\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(seed), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	path, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(featureDir, "STEP-04.md"), path)
	assert.FileExists(t, filepath.Join(featureDir, "STEP-04.md"))
}

func Test_ignores_files_that_do_not_match_the_step_file_pattern_when_numbering(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(seed), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-7.md"), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	path, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(featureDir, "STEP-02.md"), path)
}

func Test_refuses_an_unknown_feature(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, "specs", "widgets"))
}

func Test_refuses_a_feature_with_a_missing_specification(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)

	entries, readErr := os.ReadDir(featureDir)
	require.NoError(t, readErr)

	assert.ElementsMatch(t, []string{cfg.StateFile}, namesOf(entries))
}

func Test_refuses_a_specification_with_no_progress_heading_and_writes_nothing(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	sentinel := "# widgets\n\n## Something Else Entirely\n\nno progress list here.\n"
	specPath := filepath.Join(featureDir, cfg.SpecificationFile)
	require.NoError(t, os.WriteFile(specPath, []byte(sentinel), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)

	got, readErr := os.ReadFile(specPath)
	require.NoError(t, readErr)
	assert.Equal(t, sentinel, string(got))

	entries, readErr := os.ReadDir(featureDir)
	require.NoError(t, readErr)

	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile}, namesOf(entries))
}

func Test_refuses_an_invalid_step_file_pattern_and_writes_nothing(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%s.md"
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)

	entries, readErr := os.ReadDir(filepath.Join(root, "specs", "widgets"))
	require.NoError(t, readErr)
	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile}, namesOf(entries))
}

func Test_refuses_a_feature_name_that_escapes_the_feature_root(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "../escaped")

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(root, "escaped"))
}

// namesOf returns the names of a slice of directory entries.
func namesOf(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	return names
}

func Test_appends_the_progress_entry_after_the_last_existing_checklist_item(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-02\n\n## Notes\n\nSomething else entirely.\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(seed), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)

	want := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-02\n- [ ] STEP-03\n\n## Notes\n\nSomething else entirely.\n"
	assert.Equal(t, want, string(got))
}
