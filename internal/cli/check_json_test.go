// check --json scenarios run against rwfs.Mem in check_internal_test.go;
// newCheckJSONFixture stays here since help_test.go still calls it.

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newCheckJSONFixture writes two features under wd: "alpha" in flight
// with an over-cap state file, "beta" fully done with its state missing.
func newCheckJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, alphaDir, "SCENARIO-01", "open", nil)

	betaDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "specification.md"), []byte(conformingSpec), 0o600))
	writeCheckStep(t, betaDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	return wd
}
