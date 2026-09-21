package assemble_test

import (
	"io/fs"
	"os"
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
// writes "depends-on: []". Status's blocked computation is the only reader
// that cares what depends-on carries.
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

// writeStepFile writes name's content under featureDir.
func writeStepFile(t *testing.T, featureDir, name, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o600))
}

func Test_status_counts_done_over_total_and_names_the_next_step(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))
	writeStepFile(t, featureDir, "STEP-02.md", fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", nil))
	writeStepFile(t, featureDir, "STEP-03.md", fixtureStepWithDeps(cfg, "STEP-03", "open", "STEP-03", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, assemble.FeatureStatus{
		Name:  "demo",
		Done:  1,
		Total: 3,
		Next:  &assemble.NextStep{ID: "STEP-02", Title: "STEP-02", Path: filepath.Join(featureDir, "STEP-02.md")},
		Path:  featureDir,
	}, rows[0])
}

// Test_status_names_the_next_step_s_title_and_path uses a fixture whose
// heading text differs from its frontmatter id — unlike every other fixture
// in this file, which writes "# <id>" and so cannot discriminate Title
// being the id echoed twice from Title actually being markdown.Title's own
// result.
func Test_status_names_the_next_step_s_title_and_path(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "Handle the widget", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Next)
	assert.Equal(t, "STEP-01", rows[0].Next.ID)
	assert.Equal(t, "Handle the widget", rows[0].Next.Title)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), rows[0].Next.Path)
}

// Test_status_reports_the_feature_directory_path pins FeatureStatus.Path as
// the feature's own absolute directory — distinct from Next.Path, which
// names the open step file inside it.
func Test_status_reports_the_feature_directory_path(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, featureDir, rows[0].Path)
}

// Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked seeds
// two independently, differently blocked open steps (STEP-02 on open
// STEP-01, STEP-06 on open STEP-07) rather than one: a single blocked step
// cannot discriminate inverting FirstUnmet's unmet condition from the
// correct predicate, since blame would silently swap to a different open
// step while Blocked coincidentally stays 1. Two blocked steps, chosen so
// neither is the other's dependency, catch that.
func Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	// STEP-01 is open and has no dependency, so it is Next.
	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))
	// STEP-02 is open and depends on STEP-01, which is itself open: blocked.
	writeStepFile(t, featureDir, "STEP-02.md", fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", []string{"STEP-01"}))
	// STEP-03 is open and depends on STEP-04, which is done: not blocked.
	writeStepFile(t, featureDir, "STEP-03.md", fixtureStepWithDeps(cfg, "STEP-03", "open", "STEP-03", []string{"STEP-04"}))
	writeStepFile(t, featureDir, "STEP-04.md", fixtureStepWithDeps(cfg, "STEP-04", "done", "STEP-04", nil))
	// STEP-05 is done but declares an unfinished dependency (STEP-01, still
	// open): featureStatus's loop skips a done step before it is ever
	// weighed against FirstUnmet, so this must not raise Blocked beyond its
	// independently blocked siblings.
	writeStepFile(t, featureDir, "STEP-05.md", fixtureStepWithDeps(cfg, "STEP-05", "done", "STEP-05", []string{"STEP-01"}))
	// STEP-06 is open and depends on STEP-07, which is itself open: a
	// second, independent blocked step, unrelated to STEP-01/STEP-02.
	writeStepFile(t, featureDir, "STEP-06.md", fixtureStepWithDeps(cfg, "STEP-06", "open", "STEP-06", []string{"STEP-07"}))
	writeStepFile(t, featureDir, "STEP-07.md", fixtureStepWithDeps(cfg, "STEP-07", "open", "STEP-07", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].Blocked)
	require.NotNil(t, rows[0].Next)
	assert.Equal(t, "STEP-01", rows[0].Next.ID)
}

func Test_status_reports_an_unknown_dependency_id_as_blocking(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", []string{"STEP-99"}))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Blocked)
}

func Test_status_reports_no_next_step_for_a_completed_feature(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))
	writeStepFile(t, featureDir, "STEP-02.md", fixtureStepWithDeps(cfg, "STEP-02", "done", "STEP-02", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 2, Total: 2, Next: nil, Blocked: 0, Path: featureDir}, rows[0])
}

func Test_status_reports_no_next_step_for_a_feature_with_no_step_files(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 0, Total: 0, Next: nil, Blocked: 0, Path: featureDir}, rows[0])
}

// Test_status_reports_no_features_when_the_feature_root_does_not_exist is
// R14's "nothing to return is not an error" case: a repository that has
// never run brief has no feature-directory tree at all.
func Test_status_reports_no_features_when_the_feature_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	assert.Empty(t, rows)
}

// Test_status_propagates_a_feature_root_that_is_not_a_directory is the
// control arm for the test above: a feature-root path that exists but is
// a regular file is a misconfiguration, not an empty repository, and must
// stay an error rather than being swallowed by the same ErrNotExist guard.
func Test_status_propagates_a_feature_root_that_is_not_a_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.Error(t, err)
	assert.Nil(t, rows)
}

// Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse is the
// core SCENARIO-11 claim: a malformed feature (delta, whose one step file
// carries no frontmatter at all) gets a Problem-carrying row of its own
// rather than failing the whole call, and — the control that proves the
// tolerance does not blind Status to the rest — the other three features'
// rows are byte-identical to what they would be with no malformed feature
// present.
func Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	alphaDir := filepath.Join(root, cfg.FeatureDirectory, "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	writeStepFile(t, alphaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))
	writeStepFile(t, alphaDir, "STEP-02.md", fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", nil))

	betaDir := filepath.Join(root, cfg.FeatureDirectory, "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	writeStepFile(t, betaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))

	gammaDir := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.MkdirAll(gammaDir, 0o755))
	writeStepFile(t, gammaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))

	deltaDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	require.NoError(t, os.MkdirAll(deltaDir, 0o755))
	writeStepFile(t, deltaDir, "STEP-01.md", "no frontmatter here\n")

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 4)

	byName := make(map[string]assemble.FeatureStatus, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}

	assert.Equal(t, assemble.FeatureStatus{
		Name: "alpha", Done: 1, Total: 2,
		Next: &assemble.NextStep{ID: "STEP-02", Title: "STEP-02", Path: filepath.Join(alphaDir, "STEP-02.md")},
		Path: alphaDir,
	}, byName["alpha"])
	assert.Equal(t, assemble.FeatureStatus{Name: "beta", Done: 1, Total: 1, Next: nil, Blocked: 0, Path: betaDir}, byName["beta"])
	assert.Equal(t, assemble.FeatureStatus{
		Name: "gamma", Done: 0, Total: 1,
		Next: &assemble.NextStep{ID: "STEP-01", Title: "STEP-01", Path: filepath.Join(gammaDir, "STEP-01.md")},
		Path: gammaDir,
	}, byName["gamma"])

	delta := byName["delta"]
	require.NotNil(t, delta.Problem)
	assert.Equal(t, 0, delta.Done)
	assert.Equal(t, 0, delta.Total)
	assert.Nil(t, delta.Next)
	assert.Equal(t, 0, delta.Blocked)
	assert.Equal(t, deltaDir, delta.Path)
	assert.Equal(t, filepath.Join(deltaDir, "STEP-01.md"), delta.Problem.Path)
	assert.Equal(t, "no frontmatter found", delta.Problem.Detail)
	assert.Equal(t, "run 'brief check delta' to list every fault", delta.Problem.Fix)
}

// Test_status_reports_one_problem_per_feature_not_one_per_file bounds
// stderr by the feature count, not the file count: a feature with two
// unparseable step files still yields exactly one row and one Problem —
// the first failure readSteps meets, in its sorted iteration order, wins.
func Test_status_reports_one_problem_per_feature_not_one_per_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", "no frontmatter here either\n")
	writeStepFile(t, featureDir, "STEP-02.md", "still no frontmatter\n")

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Problem)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), rows[0].Problem.Path)
}

// Test_status_marks_a_feature_whose_step_file_cannot_be_read uses a step
// file that is a symlink escaping the feature root: deterministic and
// needs no privileges, unlike a chmod case. os.Root refuses to follow it
// ("path escapes from parent"), and that refusal is what Status must
// degrade into a Problem rather than propagate.
func Test_status_marks_a_feature_whose_step_file_cannot_be_read(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	outside := filepath.Join(root, "outside.md")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(featureDir, "STEP-01.md")))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Problem)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), rows[0].Problem.Path)
	assert.Equal(t, "path escapes from parent", rows[0].Problem.Detail)
	assert.Equal(t, "make it readable and re-run", rows[0].Problem.Fix)
}

// Test_status_marks_a_feature_directory_that_cannot_be_opened injects an
// EACCES failure through assemble.SetOpenRootForTest rather than chmod:
// root bypasses ordinary permission checks, so a chmod-000 directory does
// not reproduce this branch under every CI identity, and a skip keyed on
// os.Geteuid would let the branch go untested there with no signal.
func Test_status_marks_a_feature_directory_that_cannot_be_opened(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "delta" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Problem)
	assert.Equal(t, featureDir, rows[0].Problem.Path)
	assert.Equal(t, "permission denied", rows[0].Problem.Detail)
	assert.Equal(t, "make it readable and re-run", rows[0].Problem.Fix)
}

// Test_status_marks_a_feature_directory_that_cannot_be_listed is
// Test_status_marks_a_feature_directory_that_cannot_be_opened's companion
// one layer deeper: the feature directory opens fine but its step files
// cannot be listed (the branch a stricter-than-darwin permission model,
// such as Linux's, takes at a directory readable to enter but not to
// list). The injected failure goes through assemble.SetReadDirForTest,
// distinct from SetOpenRootForTest above, so it reddens only when
// featureStatus's own listing call — not its open call — fails.
func Test_status_marks_a_feature_directory_that_cannot_be_listed(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetReadDirForTest(srv, func(*os.Root) ([]os.DirEntry, error) {
		return nil, &fs.PathError{Op: "readdirent", Path: ".", Err: syscall.EACCES}
	})

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Problem)
	assert.Equal(t, featureDir, rows[0].Problem.Path)
	assert.Equal(t, "permission denied", rows[0].Problem.Detail)
	assert.Equal(t, "make it readable and re-run", rows[0].Problem.Fix)
}

// Test_status_marks_a_symlinked_feature_directory_rather_than_dropping_it
// is the change of behaviour SCENARIO-11 owns: a symlink named like a
// feature, sitting in the feature root, must not vanish from status the
// way it did before this scenario (no row at all). Its target is a
// perfectly good feature directory — the row is marked because brief does
// not follow the link, not because anything about the target is wrong.
func Test_status_marks_a_symlinked_feature_directory_rather_than_dropping_it(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	writeStepFile(t, realDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "gamma", rows[0].Name)
	assert.Equal(t, linkPath, rows[0].Path)
	require.NotNil(t, rows[0].Problem)
	assert.Equal(t, linkPath, rows[0].Problem.Path)
	assert.Equal(t, "is a symbolic link, not read as a feature directory", rows[0].Problem.Detail)
	assert.Equal(t, "replace it with a real directory", rows[0].Problem.Fix)
}

// Test_status_skips_a_regular_file_in_the_feature_directory_without_a_row
// is the control for the symlink test above, differing in exactly one
// variable: a regular file beside three good features yields three rows,
// not four, and no Problem — pinning SCENARIO-10's decision ("Status
// skips non-directories") against the symlink branch widening to catch
// every non-directory entry.
func Test_status_skips_a_regular_file_in_the_feature_directory_without_a_row(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		featureDir := filepath.Join(root, cfg.FeatureDirectory, name)
		require.NoError(t, os.MkdirAll(featureDir, 0o755))
		writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))
	}

	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "README.md"), []byte("not a feature\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 3)

	for _, row := range rows {
		assert.Nil(t, row.Problem)
	}
}

// Test_status_leaves_a_feature_whose_frontmatter_id_disagrees_with_its_filename_unmarked
// pins that an id/filename mismatch is conforming, not malformed: 09's
// pattern.ID(n) rule already governs what Next and Blocked print, and
// SCENARIO-11 must not reclassify the mismatch as an error condition —
// that is check's (22) to flag as a finding, per R18.
func Test_status_leaves_a_feature_whose_frontmatter_id_disagrees_with_its_filename_unmarked(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	writeStepFile(t, featureDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-99", "open", "STEP-01", nil))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Problem)
	require.NotNil(t, rows[0].Next)
	assert.Equal(t, "STEP-01", rows[0].Next.ID)
}

// Test_status_leaves_a_feature_with_no_step_files_unmarked pins the other
// tolerate-silently classification: an empty feature directory is a
// conforming "0/0" feature, not a malformed one.
func Test_status_leaves_a_feature_with_no_step_files_unmarked(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Problem)
	assert.Equal(t, 0, rows[0].Total)
}

// Test_status_still_propagates_an_unreadable_top_level_feature_directory
// is the control that keeps the malformed-feature tolerance scoped to one
// feature at a time: a failure opening cfg.FeatureDirectory itself — not
// one feature's directory — still fails the whole call. A self-referential
// symlink at that path is used rather than chmod: it makes os.OpenRoot
// fail with ELOOP independent of OS permission bits or effective uid,
// exercising the same propagate branch a chmod-000 root would, under any
// CI identity.
func Test_status_still_propagates_an_unreadable_top_level_feature_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	topDir := filepath.Join(root, cfg.FeatureDirectory)
	require.NoError(t, os.Symlink(topDir, topDir))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist, "a symlink loop must not be classified as a missing root")
	assert.Nil(t, rows)
}

// Test_status_still_propagates_an_invalid_step_file_pattern is the other
// top-level control: an invalid configured pattern fails before any
// feature is read, and SCENARIO-11 does not change that.
func Test_status_still_propagates_an_invalid_step_file_pattern(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "no-verb-in-here.md"
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.Error(t, err)
	assert.Nil(t, rows)
}

// Test_status_returns_nil_not_an_empty_slice_for_zero_features pins the
// nil-vs-empty-slice distinction SCENARIO-15's future --json marshaling
// depends on, for a feature root that exists but holds no entries at all —
// distinct from the "root does not exist" case above.
func Test_status_returns_nil_not_an_empty_slice_for_zero_features(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	assert.Nil(t, rows)
}

// Test_status_orders_features_in_byte_order_not_case_insensitive_order
// pins fs.ReadDir's documented byte order against a future case-insensitive
// collation. "Zeta", "alpha" and "Beta" differ from each other by more than
// case, so APFS's case-insensitive filesystem cannot fold two of them
// together and mask the ordering claim. Green on arrival is expected here:
// io/fs.ReadDir and os.Root.FS's ReadDirFS both document "sorted by
// filename" byte order, and Status relies on that without adding its own
// sort.
func Test_status_orders_features_in_byte_order_not_case_insensitive_order(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	for _, name := range []string{"Zeta", "alpha", "Beta"} {
		featureDir := filepath.Join(root, cfg.FeatureDirectory, name)
		require.NoError(t, os.MkdirAll(featureDir, 0o755))
	}

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"Beta", "Zeta", "alpha"}, []string{rows[0].Name, rows[1].Name, rows[2].Name})
}

// Test_complete reports (FeatureStatus).Complete's one rule — Problem ==
// nil && Total > 0 && Done == Total — over its four discriminating shapes.
// The malformed case sets Done == Total == 3 specifically so a mutant that
// drops the Problem == nil check would read this case as complete; the
// zero-step case sets Done == Total == 0 so a mutant that drops Total > 0
// would read it as complete too.
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
