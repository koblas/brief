package assemble_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureStepWithDeps renders a step file's body with an explicit
// depends-on list, unlike fixtureStep (assemble_test.go), which always
// writes "depends-on: []".
func fixtureStepWithDeps(cfg config.Config, id, status, title string, dependsOn []string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("id: " + id + "\n")
	sb.WriteString("status: " + status + "\n")

	if len(dependsOn) == 0 {
		sb.WriteString("depends-on: []\n")
	} else {
		sb.WriteString("depends-on:\n")
		for _, dep := range dependsOn {
			sb.WriteString("  - " + dep + "\n")
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString(cfg.AcceptanceHeading + "\n\nsome acceptance text\n\n")
	sb.WriteString(cfg.ChecklistHeading + "\n\n- [ ] a task\n")

	return sb.String()
}

// statusFSRow builds a Server (root unused: StatusFS never reads it) and
// runs StatusFS against files, wrapped as a FeatureFS rooted at
// testFeaturePath.
func statusFSRow(t *testing.T, cfg config.Config, files map[string]string) assemble.FeatureStatus {
	t.Helper()

	srv := assemble.NewServer(cfg, "")

	return srv.StatusFS(featureFS(files), statusPattern(t, cfg))
}

func Test_status_counts_done_over_total_and_names_the_next_step(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil)
	files["STEP-02.md"] = fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", nil)
	files["STEP-03.md"] = fixtureStepWithDeps(cfg, "STEP-03", "open", "STEP-03", nil)

	row := statusFSRow(t, cfg, files)

	assert.Equal(t, assemble.FeatureStatus{
		Name:  "demo",
		Done:  1,
		Total: 3,
		Next:  &assemble.NextStep{ID: "STEP-02", Title: "STEP-02", Path: filepath.Join(testFeaturePath, "STEP-02.md")},
		Path:  testFeaturePath,
	}, row)
}

// Unlike every other fixture in this file, the heading text here differs
// from the frontmatter id, so Title cannot be the id echoed twice.
func Test_status_names_the_next_step_s_title_and_path(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "open", "Handle the widget", nil)

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Next)
	assert.Equal(t, "STEP-01", row.Next.ID)
	assert.Equal(t, "Handle the widget", row.Next.Title)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), row.Next.Path)
}

// FeatureStatus.Path is the feature's own directory, distinct from
// Next.Path, which names the open step file inside it.
func Test_status_reports_the_feature_directory_path(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil)

	row := statusFSRow(t, cfg, files)

	assert.Equal(t, testFeaturePath, row.Path)
}

// Seeds two independently blocked open steps rather than one: a single
// blocked step cannot discriminate an inverted unmet condition from the
// correct predicate, since blame could silently swap while Blocked stays 1.
func Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)

	// STEP-01 is open and has no dependency, so it is Next.
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil)
	// STEP-02 is open and depends on STEP-01, which is itself open: blocked.
	files["STEP-02.md"] = fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", []string{"STEP-01"})
	// STEP-03 is open and depends on STEP-04, which is done: not blocked.
	files["STEP-03.md"] = fixtureStepWithDeps(cfg, "STEP-03", "open", "STEP-03", []string{"STEP-04"})
	files["STEP-04.md"] = fixtureStepWithDeps(cfg, "STEP-04", "done", "STEP-04", nil)
	// STEP-05 is done with an unfinished dependency: a done step is never blocked.
	files["STEP-05.md"] = fixtureStepWithDeps(cfg, "STEP-05", "done", "STEP-05", []string{"STEP-01"})
	// STEP-06 is open and depends on open STEP-07: a second, independent blocked step.
	files["STEP-06.md"] = fixtureStepWithDeps(cfg, "STEP-06", "open", "STEP-06", []string{"STEP-07"})
	files["STEP-07.md"] = fixtureStepWithDeps(cfg, "STEP-07", "open", "STEP-07", nil)

	row := statusFSRow(t, cfg, files)

	assert.Equal(t, 2, row.Blocked)
	require.NotNil(t, row.Next)
	assert.Equal(t, "STEP-01", row.Next.ID)
}

func Test_status_reports_an_unknown_dependency_id_as_blocking(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", []string{"STEP-99"})

	row := statusFSRow(t, cfg, files)

	assert.Equal(t, 1, row.Blocked)
}

func Test_status_reports_no_next_step_for_a_completed_feature(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil)
	files["STEP-02.md"] = fixtureStepWithDeps(cfg, "STEP-02", "done", "STEP-02", nil)

	row := statusFSRow(t, cfg, files)

	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 2, Total: 2, Next: nil, Blocked: 0, Path: testFeaturePath}, row)
}

func Test_status_reports_no_next_step_for_a_feature_with_no_step_files(t *testing.T) {
	cfg := fixtureConfig()

	row := statusFSRow(t, cfg, conformingFeatureFiles(cfg))

	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 0, Total: 0, Next: nil, Blocked: 0, Path: testFeaturePath}, row)
}

// A feature with two unparseable step files still yields exactly one
// Problem: the first failure readSteps meets wins.
func Test_status_reports_one_problem_per_feature_not_one_per_file(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = "no frontmatter here either\n"
	files["STEP-02.md"] = "still no frontmatter\n"

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), row.Problem.Path)
}

// An id/filename mismatch is conforming, not malformed: that is Check's
// finding to flag, not Status's.
func Test_status_leaves_a_feature_whose_frontmatter_id_disagrees_with_its_filename_unmarked(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-99", "open", "STEP-01", nil)

	row := statusFSRow(t, cfg, files)

	assert.Nil(t, row.Problem)
	require.NotNil(t, row.Next)
	assert.Equal(t, "STEP-01", row.Next.ID)
}

// An empty feature directory is a conforming "0/0" feature, not a malformed one.
func Test_status_leaves_a_feature_with_no_step_files_unmarked(t *testing.T) {
	cfg := fixtureConfig()

	row := statusFSRow(t, cfg, conformingFeatureFiles(cfg))

	assert.Nil(t, row.Problem)
	assert.Equal(t, 0, row.Total)
}

// A feature with a conforming state file and a well-formed step, but no
// specification, must not read as a clean row.
func Test_status_marks_a_feature_whose_specification_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.StateFile: checkConformingState(cfg),
		"STEP-01.md":  fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, 0, row.Done)
	assert.Equal(t, 0, row.Total)
	assert.Nil(t, row.Next)
	assert.Equal(t, testFeaturePath, row.Path)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), row.Problem.Path)
	assert.Equal(t, cfg.SpecificationFile+" not found", row.Problem.Detail)
	assert.Equal(t, fmt.Sprintf("write a %s with a %q heading and re-run", cfg.SpecificationFile, cfg.ProgressHeading), row.Problem.Fix)
}

// A specification that reads fine but carries none of cfg.ProgressHeading.
func Test_status_marks_a_feature_whose_specification_has_no_progress_heading(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: "# demo\n\nno progress list here\n",
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md":          fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), row.Problem.Path)
	assert.Equal(t, fmt.Sprintf("no %q heading found", cfg.ProgressHeading), row.Problem.Detail)
	assert.Equal(t, fmt.Sprintf("add a %q heading to the specification", cfg.ProgressHeading), row.Problem.Fix)
}

// "file does not exist" is fs.ErrNotExist's own fixed, cross-platform text.
func Test_status_marks_a_feature_whose_state_file_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		"STEP-01.md":          fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), row.Problem.Path)
	assert.Equal(t, "file does not exist", row.Problem.Detail)
	assert.Equal(t, "make it readable and re-run", row.Problem.Fix)
}

// Dropping both specification and state leaves only the check order to
// decide which Problem wins.
func Test_status_prefers_the_specification_fault_over_a_missing_state_file(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		"STEP-01.md": fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), row.Problem.Path)
	assert.Equal(t, cfg.SpecificationFile+" not found", row.Problem.Detail)
}

// Control arm for the three tests above: the same shape, minus the one
// fault each removes, stays a clean row.
func Test_status_leaves_a_feature_with_a_conforming_specification_and_state_unmarked(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil)

	row := statusFSRow(t, cfg, files)

	assert.Nil(t, row.Problem)
	assert.Equal(t, 1, row.Total)
}

// A feature with no specification and a step file whose frontmatter does
// not parse must report the specification's own Problem, not the step's.
func Test_status_prefers_the_specification_fault_over_a_step_file_fault(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.StateFile: checkConformingState(cfg),
		"STEP-01.md":  "no frontmatter here\n",
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), row.Problem.Path)
	assert.Equal(t, cfg.SpecificationFile+" not found", row.Problem.Detail)
}

// A conforming specification beside a missing state file and a step file
// whose frontmatter does not parse must report the state file's Problem.
func Test_status_prefers_the_state_fault_over_a_step_file_fault(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		"STEP-01.md":          "no frontmatter here\n",
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), row.Problem.Path)
	assert.Equal(t, "file does not exist", row.Problem.Detail)
}

// Status and Check must report the same line for the same fence; both
// run against the exact same fsys here.
func Test_status_reports_the_line_of_a_state_file_s_unclosed_fence(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateUnterminatedFence(cfg),
		"STEP-01.md":          fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}
	fsys := featureFS(files)

	row := assemble.NewServer(cfg, "").StatusFS(fsys, statusPattern(t, cfg))

	require.NotNil(t, row.Problem)
	assert.Equal(t, 17, row.Problem.Line)

	findings, err := checkFSRunner(t, cfg, fsys)()
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, 17, findings[0].Line, "Status and Check must report the same line for the same fence")
}

// A fault with no particular line, here a missing specification, reports
// Line 0, the "whole file" sentinel.
func Test_status_leaves_line_at_zero_for_a_whole_file_fault(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.StateFile: checkConformingState(cfg),
		"STEP-01.md":  fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
	}

	row := statusFSRow(t, cfg, files)

	require.NotNil(t, row.Problem)
	assert.Equal(t, 0, row.Problem.Line)
}

// The feature directory opens fine but its step files cannot be listed.
func Test_status_marks_a_feature_directory_that_cannot_be_listed(t *testing.T) {
	cfg := fixtureConfig()
	fsys := featureFS(nil)
	fsys.FS = failFS{FS: fsys.FS, failReadDir: ".", err: syscall.EACCES}

	srv := assemble.NewServer(cfg, "")
	row := srv.StatusFS(fsys, statusPattern(t, cfg))

	require.NotNil(t, row.Problem)
	assert.Equal(t, testFeaturePath, row.Problem.Path)
	assert.Equal(t, "permission denied", row.Problem.Detail)
	assert.Equal(t, "make it readable and re-run", row.Problem.Fix)
}

// The malformed case sets Done == Total == 3 so a mutant dropping the
// Problem == nil check reads it as complete.
func Test_complete(t *testing.T) {
	cases := []struct {
		name string
		row  assemble.FeatureStatus
		want bool
	}{
		{name: "in progress", row: assemble.FeatureStatus{Total: 3, Done: 1}, want: false},
		{name: "all steps done", row: assemble.FeatureStatus{Total: 3, Done: 3}, want: true},
		{name: "zero step files", row: assemble.FeatureStatus{Total: 0, Done: 0}, want: false},
		{name: "malformed despite done equaling total", row: assemble.FeatureStatus{Total: 3, Done: 3, Problem: &assemble.Problem{Detail: "x", Fix: "y"}}, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.row.Complete())
		})
	}
}
