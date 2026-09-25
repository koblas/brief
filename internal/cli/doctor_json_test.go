package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newDoctorJSONFixture writes a valid ".brief.yaml", its default feature
// root and a ".git" directory under wd — a repository doctor reports clean.
func newDoctorJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".git"), 0o755))

	return wd
}
