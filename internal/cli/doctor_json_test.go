package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newDoctorJSONFixture writes a valid ".brief.yaml", its default feature
// root and a ".git" directory under wd — a repository doctor reports
// clean of every ERROR: config-file through root-dir and env-git are all
// OK, leaving only env-path's own severity (OK or WARN, never ERROR
// without a seam to control it) undetermined by this fixture.
func newDoctorJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".git"), 0o755))

	return wd
}
