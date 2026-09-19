package scaffold

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRoot returns an *os.Root over a fresh, empty temporary directory.
func openRoot(t *testing.T) *os.Root {
	t.Helper()

	dir := t.TempDir()

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
// the helpers are exercised directly. Each plants its own "blocked"
// directory — the target no rename can land on — rather than inheriting one
// from a shared root, so the fixture visibly matches what the test asserts.
func Test_replaceBytes_reports_a_rename_it_could_not_commit(t *testing.T) {
	root := openRoot(t)
	require.NoError(t, root.Mkdir("blocked", 0o755))

	err := replaceBytes(root, "blocked", []byte("new"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func Test_replaceString_reports_a_rename_it_could_not_commit(t *testing.T) {
	root := openRoot(t)
	require.NoError(t, root.Mkdir("blocked", 0o755))

	err := replaceString(root, "blocked", "new")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

// Test_replaceBytes_and_replaceString_write_the_same_bytes is the control
// that the two helpers differ only in the type they accept, so the
// io.StringWriter path is not quietly writing something else. It uses a
// bare root: neither write here is expected to fail, so it has no need for
// the "blocked" fixture the two tests above target.
func Test_replaceBytes_and_replaceString_write_the_same_bytes(t *testing.T) {
	root := openRoot(t)

	require.NoError(t, replaceBytes(root, "from-bytes", []byte("identical payload")))
	require.NoError(t, replaceString(root, "from-string", "identical payload"))

	fromBytes, bytesErr := root.ReadFile("from-bytes")
	require.NoError(t, bytesErr)
	fromString, stringErr := root.ReadFile("from-string")
	require.NoError(t, stringErr)

	assert.Equal(t, string(fromBytes), string(fromString))
	assert.Equal(t, "identical payload", string(fromString))
}
