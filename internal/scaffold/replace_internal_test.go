package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openReplaceRoot returns an *os.Root over a fresh directory that already
// holds a directory named "blocked" — a target no rename can land on, which
// is how these tests reach the failure that only Close can report.
func openReplaceRoot(t *testing.T) *os.Root {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "blocked"), 0o755))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	return root
}

// Test_replaceBytes_reports_a_rename_it_could_not_commit and its WriteString
// twin exist because the write sites no longer share a single WriteFile:
// each helper checks Close by hand, and a helper that returned the write's
// error instead would report success for a file that was never replaced.
// Reaching this through Finish is only possible at the handoff position, so
// the helpers are exercised directly.
func Test_replaceBytes_reports_a_rename_it_could_not_commit(t *testing.T) {
	root := openReplaceRoot(t)

	err := replaceBytes(root, "blocked", []byte("new"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func Test_replaceString_reports_a_rename_it_could_not_commit(t *testing.T) {
	root := openReplaceRoot(t)

	err := replaceString(root, "blocked", "new")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

// Test_replaceBytes_and_replaceString_write_the_same_bytes is the control
// that the two helpers differ only in the type they accept, so the
// io.StringWriter path is not quietly writing something else.
func Test_replaceBytes_and_replaceString_write_the_same_bytes(t *testing.T) {
	root := openReplaceRoot(t)

	require.NoError(t, replaceBytes(root, "from-bytes", []byte("identical payload")))
	require.NoError(t, replaceString(root, "from-string", "identical payload"))

	fromBytes, bytesErr := root.ReadFile("from-bytes")
	require.NoError(t, bytesErr)
	fromString, stringErr := root.ReadFile("from-string")
	require.NoError(t, stringErr)

	assert.Equal(t, string(fromBytes), string(fromString))
	assert.Equal(t, "identical payload", string(fromString))
}
