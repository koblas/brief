// Every check --json scenario moved onto rwfs.Mem in check_internal_test.go.
// newCheckJSONFixture stays here rather than moving with them: help_test.go,
// out of this pass's scope, still calls it.

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newCheckJSONFixture writes two features under wd, named so fs.ReadDir's
// byte order is also Check's and the golden's own feature order: "alpha"
// is still in flight (an open step) with an over-cap state file — one
// ERROR finding carrying a line — and "beta" is fully done, with its
// state file missing entirely — one WARN, whole-file finding (line 0,
// "in_flight":false, the "missing STATE" shape rather than a feature-level
// producer, which always hard-codes in_flight true).
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
