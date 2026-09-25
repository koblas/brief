package scaffold_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureConfig returns a Config whose every field this package reads
// differs from config.Default(). The caps sit above every fixture body's
// own line count, so only the cap tests' own boundary-sized bodies trip
// them.
func fixtureConfig() config.Config {
	cfg := config.Default()
	cfg.FeatureDirectory = "specs"
	cfg.ProgressHeading = "## Progress"
	cfg.ChecklistHeading = "## Fixture Checklist"
	cfg.SpecificationFile = "SPEC.md"
	cfg.StateFile = "NOTES.md"
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffFileSuffix = ".fixture-handoff.md"
	cfg.HandoffCapLines = 10
	cfg.StateCapLines = 20
	cfg.StateHeadings = config.StateHeadings{
		BindingDecisions: "## Decisions Fixture",
		LeftUnbuilt:      "## Left Fixture",
		Traps:            "## Gotchas",
		OpenDebts:        "## Debts Fixture",
	}

	return cfg
}

func Test_creates_the_feature_directory_under_the_configured_feature_directory(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.NoError(t, err)
	_, statErr := top.Stat("widgets")
	require.NoError(t, statErr)
}

func Test_returns_the_path_of_the_created_feature_directory(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	res, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets"), res.Path)
}

func Test_new_feature_reports_the_directory_and_the_files_it_created(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")

	res, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.NoError(t, err)
	featureDir := filepath.Join(testSpecsRoot, "widgets")
	assert.Equal(t, "widgets", res.Feature)
	assert.Empty(t, res.Step)
	assert.Equal(t, featureDir, res.Path)
	assert.Equal(t, []string{
		filepath.Join(featureDir, cfg.SpecificationFile),
		filepath.Join(featureDir, cfg.StateFile),
	}, res.Created)
	_, specErr := top.Stat("widgets/" + cfg.SpecificationFile)
	require.NoError(t, specErr)
	_, stateErr := top.Stat("widgets/" + cfg.StateFile)
	require.NoError(t, stateErr)
}

func Test_writes_the_specification_skeleton_with_the_configured_progress_heading_and_nothing_under_it(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")
	require.NoError(t, err)

	got, readErr := top.ReadFile("widgets/SPEC.md")
	require.NoError(t, readErr)
	assert.Equal(t, "# widgets\n\n## Progress\n", string(got))
}

func Test_writes_the_state_file_with_the_four_configured_headings_and_nothing_under_them(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")
	require.NoError(t, err)

	got, readErr := top.ReadFile("widgets/NOTES.md")
	require.NoError(t, readErr)
	assert.Equal(t, "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n", string(got))
}

func Test_creates_only_the_specification_and_the_state_file(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")
	require.NoError(t, err)

	view := openFeatureViewFS(t, top, "widgets")
	entries, err := view.ReadDir(".")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"SPEC.md", "NOTES.md"}, namesOf(entries))
}

func Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	sentinel := "# handwritten by a person"
	require.NoError(t, top.Mkdir("widgets", 0o755))
	require.NoError(t, top.WriteFile("widgets/"+cfg.SpecificationFile, []byte(sentinel), 0o600))

	srv := scaffold.NewServer(cfg, "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.ErrorIs(t, err, scaffold.ErrFeatureExists)

	got, readErr := top.ReadFile("widgets/" + cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Equal(t, sentinel, string(got))
}

func Test_returns_an_error_when_the_feature_name_escapes_the_feature_root(t *testing.T) {
	top := newFeatureRootFS(t)
	srv := scaffold.NewServer(fixtureConfig(), "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "../escaped")

	require.Error(t, err)
	require.NotErrorIs(t, err, scaffold.ErrFeatureExists)
	_, statErr := top.Stat("../escaped")
	assert.Error(t, statErr)
}

func Test_refuses_an_existing_feature_naming_its_directory(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	view := openFeatureViewFS(t, top, "widgets")
	_, stepErr := srv.NewStepFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)
	require.NoError(t, stepErr)

	_, err = srv.NewFeatureFS(top, testSpecsRoot, "widgets")
	require.Error(t, err)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets"), refusal.Path)
	assert.Equal(t, 0, refusal.Line)
	assert.ErrorIs(t, err, scaffold.ErrFeatureExists)
}

// Snapshot equality also catches an added temp entry, unlike a
// directory-listing-only assertion.
func Test_leaves_an_existing_features_files_byte_identical_when_it_refuses(t *testing.T) {
	top := newFeatureRootFS(t).(*rwfs.Mem) //nolint:forcetypeassert // newFeatureRootFS always returns *rwfs.Mem
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	view := openFeatureViewFS(t, top, "widgets")
	_, stepErr := srv.NewStepFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)
	require.NoError(t, stepErr)

	before := top.Snapshot()

	_, err = srv.NewFeatureFS(top, testSpecsRoot, "widgets")
	require.Error(t, err)

	assert.Equal(t, before, top.Snapshot())
}

// Control arm for the byte-identity claim above: same fixture and probe,
// with NewStepFS in place of the refused NewFeatureFS call.
func Test_the_snapshot_probe_sees_a_change_when_the_scaffold_writes_one(t *testing.T) {
	top := newFeatureRootFS(t).(*rwfs.Mem) //nolint:forcetypeassert // newFeatureRootFS always returns *rwfs.Mem
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	view := openFeatureViewFS(t, top, "widgets")
	_, stepErr := srv.NewStepFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)
	require.NoError(t, stepErr)

	before := top.Snapshot()

	_, err = srv.NewStepFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)
	require.NoError(t, err)

	assert.NotEqual(t, before, top.Snapshot())
}

func Test_reports_a_specification_write_that_cannot_be_committed_on_new_feature(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	cfg.SpecificationFile = filepath.Join("sub", "SPEC.md")
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the feature directory already landed via Mkdir before the specification write could fail")
	_, statErr := top.Stat("widgets")
	assert.NoError(t, statErr)
}

// Configuring the state file with the same name as the specification file
// makes the state write collide with what the specification write just
// created; the read-back below proves that write was never truncated.
func Test_reports_a_state_write_that_cannot_be_committed_on_new_feature(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	cfg.StateFile = cfg.SpecificationFile
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.NewFeatureFS(top, testSpecsRoot, "widgets")

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the specification write already landed before the colliding state write could fail")

	got, readErr := top.ReadFile("widgets/" + cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Equal(t, "# widgets\n\n## Progress\n", string(got),
		"the specification write must not have been truncated by the failed state write")
}
