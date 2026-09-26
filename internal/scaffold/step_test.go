package scaffold_test

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStepFS calls NewStepFS against a view already opened on feature,
// requiring success.
func newStepFS(t *testing.T, srv *scaffold.Server, view rwfs.FS, cfg config.Config, feature string) scaffold.Result {
	t.Helper()

	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)

	res, err := srv.NewStepFS(view, filepath.Join(testSpecsRoot, feature), feature, pattern)
	require.NoError(t, err)

	return res
}

func Test_writes_the_step_file_with_frontmatter_a_title_an_acceptance_heading_and_an_empty_checklist(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")

	res := newStepFS(t, srv, view, cfg, "widgets")

	got, readErr := view.ReadFile("STEP-01.md")
	require.NoError(t, readErr)
	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets", "STEP-01.md"), res.Path)

	want := "---\n" +
		"id: STEP-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-01\n" +
		"\n" +
		cfg.AcceptanceHeading + "\n" +
		"\n" +
		cfg.ChecklistHeading + "\n"
	assert.Equal(t, want, string(got))
}

// newTickedStepFS calls newStepFS then appends one ticked checklist item
// under its checklist heading, so a Finish call against it succeeds.
func newTickedStepFS(t *testing.T, srv *scaffold.Server, view rwfs.FS, cfg config.Config, feature string) {
	t.Helper()

	res := newStepFS(t, srv, view, cfg, feature)

	name := filepath.Base(res.Path)
	body, err := view.ReadFile(name)
	require.NoError(t, err)
	require.NoError(t, view.WriteFile(name, append(body, []byte("\n- [x] done\n")...), 0o600))
}

// Non-vacuity proven by Test_the_handoff_probe_sees_a_handoff_file_after_a_finish.
func Test_writes_no_handoff_file(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	newTickedStepFS(t, srv, view, cfg, "widgets")

	_, statErr := view.Stat("STEP-01" + cfg.HandoffFileSuffix)
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

// Control arm for Test_writes_no_handoff_file: same probe, same feature,
// after a Finish call, must see the file.
func Test_the_handoff_probe_sees_a_handoff_file_after_a_finish(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	newTickedStepFS(t, srv, view, cfg, "widgets")

	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	handoffPattern, err := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, err)

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	_, err = srv.FinishFS(view, filepath.Join(testSpecsRoot, "widgets"), "widgets", "STEP-01", []byte("HANDOFF"), []byte(state), pattern, handoffPattern)
	require.NoError(t, err)

	_, statErr := view.Stat("STEP-01" + cfg.HandoffFileSuffix)
	assert.NoError(t, statErr)
}

func Test_a_handoff_file_does_not_advance_the_next_step_number(t *testing.T) {
	cfg := fixtureConfig()
	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n"

	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile:             &fstest.MapFile{Data: []byte(seed), Mode: 0o600},
		cfg.StateFile:                     &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-01.md":                      &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-01" + cfg.HandoffFileSuffix: &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	res := newStepFS(t, srv, mem, cfg, "widgets")

	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets", "STEP-02.md"), res.Path)
}

// Also asserts the specification's own bytes changed, proving a real
// write is nonetheless excluded from Created, not merely that nothing ran.
func Test_new_step_reports_the_step_id_and_only_the_step_file_as_created(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	featureRes := createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	specBefore, err := view.ReadFile(cfg.SpecificationFile)
	require.NoError(t, err)

	res := newStepFS(t, srv, view, cfg, "widgets")

	stepPath := filepath.Join(featureRes.Path, "STEP-01.md")
	assert.Equal(t, "widgets", res.Feature)
	assert.Equal(t, "STEP-01", res.Step)
	assert.Equal(t, stepPath, res.Path)
	assert.Equal(t, []string{stepPath}, res.Created)

	specAfter, err := view.ReadFile(cfg.SpecificationFile)
	require.NoError(t, err)
	assert.NotEqual(t, string(specBefore), string(specAfter), "the specification must have actually changed")
	assert.NotContains(t, res.Created, filepath.Join(featureRes.Path, cfg.SpecificationFile))
}

func Test_appends_the_progress_entry_under_the_progress_heading_when_the_list_is_empty(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	newStepFS(t, srv, view, cfg, "widgets")

	got, readErr := view.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Equal(t, "# widgets\n\n## Progress\n\n- [ ] STEP-01\n", string(got))
}

func Test_returns_the_path_of_the_created_step_file(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")

	res := newStepFS(t, srv, view, cfg, "widgets")

	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets", "STEP-01.md"), res.Path)
}

func Test_leaves_no_temp_file_in_the_feature_directory(t *testing.T) {
	top := newFeatureRootFS(t)
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, "")
	createFeatureFS(t, srv, top, "widgets")
	view := openFeatureViewFS(t, top, "widgets")
	newStepFS(t, srv, view, cfg, "widgets")

	entries, readErr := view.ReadDir(".")
	require.NoError(t, readErr)

	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile, "STEP-01.md"}, namesOf(entries))
}

func Test_NewStep_finds_a_progress_heading_terminated_by_a_carriage_return(t *testing.T) {
	cfg := fixtureConfig()
	spec := "# widgets\r\n\r\n" + cfg.ProgressHeading + "\r\n"

	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	newStepFS(t, srv, mem, cfg, "widgets")

	got, readErr := mem.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [ ] STEP-01")
}

func Test_numbers_the_next_step_from_the_highest_existing_step_file(t *testing.T) {
	cfg := fixtureConfig()
	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-03\n"

	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(seed), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-01.md":          &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-03.md":          &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	res := newStepFS(t, srv, mem, cfg, "widgets")

	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets", "STEP-04.md"), res.Path)
	_, statErr := mem.Stat("STEP-04.md")
	assert.NoError(t, statErr)
}

func Test_ignores_files_that_do_not_match_the_step_file_pattern_when_numbering(t *testing.T) {
	cfg := fixtureConfig()
	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n"

	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(seed), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-01.md":          &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-7.md":           &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	res := newStepFS(t, srv, mem, cfg, "widgets")

	assert.Equal(t, filepath.Join(testSpecsRoot, "widgets", "STEP-02.md"), res.Path)
}

func Test_refuses_a_feature_with_a_missing_specification(t *testing.T) {
	cfg := fixtureConfig()
	mem := rwfs.NewMem(fstest.MapFS{
		cfg.StateFile: &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	srv := scaffold.NewServer(cfg, "")

	_, err = srv.NewStepFS(mem, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)

	entries, readErr := mem.ReadDir(".")
	require.NoError(t, readErr)
	assert.ElementsMatch(t, []string{cfg.StateFile}, namesOf(entries))
}

func Test_refuses_a_specification_with_no_progress_heading_and_writes_nothing(t *testing.T) {
	cfg := fixtureConfig()
	sentinel := "# widgets\n\n## Something Else Entirely\n\nno progress list here.\n"
	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(sentinel), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	pattern, err := stepfilePattern(cfg)
	require.NoError(t, err)
	srv := scaffold.NewServer(cfg, "")

	_, err = srv.NewStepFS(mem, filepath.Join(testSpecsRoot, "widgets"), "widgets", pattern)

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)

	got, readErr := mem.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Equal(t, sentinel, string(got))

	entries, readErr := mem.ReadDir(".")
	require.NoError(t, readErr)
	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile}, namesOf(entries))
}

// namesOf returns the names of a slice of directory entries.
func namesOf(entries []fs.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	return names
}

func Test_appends_the_progress_entry_after_the_last_existing_checklist_item(t *testing.T) {
	cfg := fixtureConfig()
	seed := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-02\n\n## Notes\n\nSomething else entirely.\n"

	mem := rwfs.NewMem(fstest.MapFS{
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(seed), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-01.md":          &fstest.MapFile{Data: []byte(""), Mode: 0o600},
		"STEP-02.md":          &fstest.MapFile{Data: []byte(""), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	newStepFS(t, srv, mem, cfg, "widgets")

	got, readErr := mem.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)

	want := "# widgets\n\n## Progress\n\n- [ ] STEP-01\n- [x] STEP-02\n- [ ] STEP-03\n\n## Notes\n\nSomething else entirely.\n"
	assert.Equal(t, want, string(got))
}
