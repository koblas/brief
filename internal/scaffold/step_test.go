package scaffold_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_writes_the_step_file_with_frontmatter_a_title_and_an_empty_checklist
// renamed from …_and_a_handoff_anchor: the tool writes no handoff file and
// no handoff heading, so a fresh step file ends with an empty checklist.
func Test_writes_the_step_file_with_frontmatter_a_title_and_an_empty_checklist(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	res, err := srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	got, readErr := os.ReadFile(res.Path)
	require.NoError(t, readErr)

	want := "---\n" +
		"id: STEP-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-01\n" +
		"\n" +
		cfg.ChecklistHeading + "\n"
	assert.Equal(t, want, string(got))
}

// stepHandoffPath returns the path of feature's step "STEP-01"'s handoff
// file under root, using cfg's configured feature directory, step-file
// pattern id and handoff-file suffix, deriving the name by literal string
// concatenation rather than through stepfile.CompileHandoff, so a
// production naming bug cannot hide behind the helper's own derivation.
func stepHandoffPath(root string, cfg config.Config, feature string) string {
	return filepath.Join(root, cfg.FeatureDirectory, feature, "STEP-01"+cfg.HandoffFileSuffix)
}

// Test_writes_no_handoff_file pins the amended SCENARIO-03 contract: only
// finish writes a handoff file. Its non-vacuity is
// Test_the_handoff_probe_sees_a_handoff_file_after_a_finish's job.
func Test_writes_no_handoff_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	_, statErr := os.Stat(stepHandoffPath(root, cfg, "widgets"))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

// Test_the_handoff_probe_sees_a_handoff_file_after_a_finish is the control
// arm for Test_writes_no_handoff_file: the same probe, against the same
// feature, after a Finish call, must see the file — proving the probe
// above tests the right path rather than passing vacuously.
func Test_the_handoff_probe_sees_a_handoff_file_after_a_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	_, err = srv.Finish(context.Background(), "widgets", "STEP-01", []byte("HANDOFF"), []byte(state))
	require.NoError(t, err)

	_, statErr := os.Stat(stepHandoffPath(root, cfg, "widgets"))
	assert.NoError(t, statErr)
}

// Test_a_handoff_file_does_not_advance_the_next_step_number pins
// Pattern.Number's digits-only scan: a handoff file beside STEP-01 must
// never be counted as a step file when NewStep numbers the next one.
func Test_a_handoff_file_does_not_advance_the_next_step_number(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDir := filepath.Join(root, "specs", "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(seed), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01"+cfg.HandoffFileSuffix), []byte(""), 0o600))

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(featureDir, "STEP-02.md"), res.Path)
}

// Test_new_step_reports_the_step_id_and_only_the_step_file_as_created pins
// NewStep's result shape: Step is the id the created file's own name
// carries, and Created lists only the step file — never the specification,
// even though NewStep also modifies it by appending a progress entry. The
// control arm is the specification's own bytes: they differ from what
// NewFeature wrote (Test_writes_the_specification_skeleton_with_the_
// configured_progress_heading_and_nothing_under_it pins that baseline), so
// this test proves a real write is nonetheless excluded from Created,
// rather than Created being empty because nothing happened.
func Test_new_step_reports_the_step_id_and_only_the_step_file_as_created(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	featureRes, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)
	specBefore, err := os.ReadFile(filepath.Join(featureRes.Path, cfg.SpecificationFile))
	require.NoError(t, err)

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	stepPath := filepath.Join(featureRes.Path, "STEP-01.md")
	assert.Equal(t, "widgets", res.Feature)
	assert.Equal(t, "STEP-01", res.Step)
	assert.Equal(t, stepPath, res.Path)
	assert.Equal(t, []string{stepPath}, res.Created)

	specAfter, err := os.ReadFile(filepath.Join(featureRes.Path, cfg.SpecificationFile))
	require.NoError(t, err)
	assert.NotEqual(t, string(specBefore), string(specAfter), "the specification must have actually changed")
	assert.NotContains(t, res.Created, filepath.Join(featureRes.Path, cfg.SpecificationFile))
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

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "specs", "widgets", "STEP-01.md"), res.Path)
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

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(featureDir, "STEP-04.md"), res.Path)
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

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(featureDir, "STEP-02.md"), res.Path)
}

// Test_reports_a_specification_write_that_cannot_be_committed_on_new_step
// covers NewStep's own specification write, now that it shares
// replaceString with Finish rather than inlining Create/WriteString/Close:
// a directory planted at the specification's temp sibling blocks the
// rename. The step file is written before the specification (scaffold.go's
// NewStep doc), so the failure this induces is expected to leave an orphan
// step file behind — visible and repairable — rather than losing it.
func Test_reports_a_specification_write_that_cannot_be_committed_on_new_step(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	blocked := filepath.Join(featureDir, "."+cfg.SpecificationFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err = srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the step file ahead of the blocked specification write already landed")
	assert.FileExists(t, filepath.Join(featureDir, "STEP-01.md"),
		"the step file lands before the specification write, so it must survive the specification write's failure")
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

// Test_refuses_an_empty_feature_argument pins that an empty feature
// argument is refused the same way a traversal attempt is —
// validFeatureArgument rejects it before openFeatureDir's first
// os.Root.OpenRoot call — rather than reaching topRoot.OpenRoot("") and
// surfacing its own opaque "empty path" failure, which names no feature and
// suggests no fix.
func Test_refuses_an_empty_feature_argument(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "")

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
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
