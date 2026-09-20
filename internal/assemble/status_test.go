package assemble_test

import (
	"os"
	"path/filepath"
	"strings"
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
	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 1, Total: 3, Next: "STEP-02", Blocked: 0}, rows[0])
}

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
	// open): a done step is never counted as blocked, whatever its
	// dependencies say, so this must not raise Blocked beyond 1.
	writeStepFile(t, featureDir, "STEP-05.md", fixtureStepWithDeps(cfg, "STEP-05", "done", "STEP-05", []string{"STEP-01"}))

	srv := assemble.NewServer(cfg, root)

	rows, err := srv.Status(t.Context())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Blocked)
	assert.Equal(t, "STEP-01", rows[0].Next)
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
	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 2, Total: 2, Next: "", Blocked: 0}, rows[0])
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
	assert.Equal(t, assemble.FeatureStatus{Name: "demo", Done: 0, Total: 0, Next: "", Blocked: 0}, rows[0])
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
