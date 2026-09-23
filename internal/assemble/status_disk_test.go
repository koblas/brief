// These Status scenarios stay on real disk: Status's own multi-feature
// enumeration and ordering, the os.Root containment chain a symlinked or
// unreadable feature directory exercises, and the openRoot test-injection
// seam all live at the adapter layer StatusFS sits behind, one level below
// anything an in-memory fs.FS can stand in for. StatusFS's own single-
// feature content rules are pinned against fstest.MapFS in status_test.go.
package assemble_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeStepFile writes name's content under featureDir.
func writeStepFile(t *testing.T, featureDir, name, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o600))
}

// writeConformingFeature creates featureDir and writes a specification and
// state file that pass specFault/readStateFile — the same two checks
// assemble.Start refuses on, so Status must not report a clean row for a
// feature Start would refuse. Every fixture in this file that means to
// exercise step-level behavior, not the spec/state gate itself, must route
// its directory creation through this rather than a bare os.MkdirAll, or the
// spec/state check now ahead of the step read would win the row's Problem
// for the wrong reason.
func writeConformingFeature(t *testing.T, cfg config.Config, featureDir string) {
	t.Helper()

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
}

// Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse is the
// core SCENARIO-11 claim: a malformed feature (delta, whose one step file
// carries no frontmatter at all) gets a Problem-carrying row of its own
// rather than failing the whole call, and — the control that proves the
// tolerance does not blind Status to the rest — the other three features'
// rows are byte-identical to what they would be with no malformed feature
// present. Real disk: the claim is about Status enumerating four sibling
// feature directories, not about one feature's own content.
func Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	alphaDir := filepath.Join(root, cfg.FeatureDirectory, "alpha")
	writeConformingFeature(t, cfg, alphaDir)
	writeStepFile(t, alphaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))
	writeStepFile(t, alphaDir, "STEP-02.md", fixtureStepWithDeps(cfg, "STEP-02", "open", "STEP-02", nil))

	betaDir := filepath.Join(root, cfg.FeatureDirectory, "beta")
	writeConformingFeature(t, cfg, betaDir)
	writeStepFile(t, betaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "done", "STEP-01", nil))

	gammaDir := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	writeConformingFeature(t, cfg, gammaDir)
	writeStepFile(t, gammaDir, "STEP-01.md", fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil))

	deltaDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	writeConformingFeature(t, cfg, deltaDir)
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
	assert.Equal(t, "frontmatter does not parse: no frontmatter found", delta.Problem.Detail)
	assert.Equal(t, "run 'brief check delta' to list every fault", delta.Problem.Fix)
}

// Test_status_marks_a_feature_whose_step_file_cannot_be_read uses a step
// file that is a symlink escaping the feature root: deterministic and
// needs no privileges, unlike a chmod case. os.Root refuses to follow it
// ("path escapes from parent"), and that refusal is what Status must
// degrade into a Problem rather than propagate. Real disk: the refusal is
// os.Root's own containment behavior.
func Test_status_marks_a_feature_whose_step_file_cannot_be_read(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "delta")
	writeConformingFeature(t, cfg, featureDir)

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
// os.Geteuid would let the branch go untested there with no signal. Real
// disk: openRoot is the adapter-level containment hook StatusFS never
// touches.
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

// Test_status_marks_a_symlinked_feature_directory_rather_than_dropping_it
// is the change of behaviour SCENARIO-11 owns: a symlink named like a
// feature, sitting in the feature root, must not vanish from status the
// way it did before this scenario (no row at all). Its target is a
// perfectly good feature directory — the row is marked because brief does
// not follow the link, not because anything about the target is wrong.
// Real disk: the subject is the top-level symlink itself.
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
// every non-directory entry. Real disk: the top-level entry-type switch.
func Test_status_skips_a_regular_file_in_the_feature_directory_without_a_row(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		featureDir := filepath.Join(root, cfg.FeatureDirectory, name)
		writeConformingFeature(t, cfg, featureDir)
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

// Test_status_still_propagates_an_unreadable_top_level_feature_directory
// is the control that keeps the malformed-feature tolerance scoped to one
// feature at a time: a failure opening cfg.FeatureDirectory itself — not
// one feature's directory — still fails the whole call. A self-referential
// symlink at that path is used rather than chmod: it makes os.OpenRoot
// fail with ELOOP independent of OS permission bits or effective uid,
// exercising the same propagate branch a chmod-000 root would, under any
// CI identity. Real disk: cfg.FeatureDirectory's own open call.
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
// feature is read, and SCENARIO-11 does not change that. Real disk: the
// claim is about Status's own precompute-before-listing order, which needs
// only a real, empty feature-directory root to exercise.
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
// distinct from the "root does not exist" case below.
func Test_status_returns_nil_not_an_empty_slice_for_zero_features(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	assert.Nil(t, rows)
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

// Test_status_orders_features_in_byte_order_not_case_insensitive_order
// pins fs.ReadDir's documented byte order against a future case-insensitive
// collation. "Zeta", "alpha" and "Beta" differ from each other by more than
// case, so APFS's case-insensitive filesystem cannot fold two of them
// together and mask the ordering claim. Green on arrival is expected here:
// io/fs.ReadDir and os.Root.FS's ReadDirFS both document "sorted by
// filename" byte order, and Status relies on that without adding its own
// sort. Real disk: the claim is about os.Root.FS's own ReadDir order.
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
