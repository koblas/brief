package writable_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/writable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Probe_reports_true_for_a_writable_directory(t *testing.T) {
	dir := t.TempDir()

	assert.True(t, writable.Probe(dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Skipped under root, which ignores directory write permission.
func Test_Probe_reports_false_for_an_unwritable_directory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	assert.False(t, writable.Probe(dir))
}

func Test_Probe_reports_false_for_a_path_that_is_not_a_directory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-dir")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	assert.False(t, writable.Probe(path))
}
