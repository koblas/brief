package scaffold

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReplaceFS returns a fresh, empty rwfs.Mem: replaceBytes and
// replaceString both go through rwfs.FS.WriteFile, so their own contract —
// applying perm only on create, refusing a directory target — is already
// proven once, against both adapters, by rwfs's own contract test; this
// file only needs to pin that both helpers reach WriteFile identically and
// that peelWriteErr strips rwfs's own Op/Path wrapping off the result.
func newReplaceFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{})
}

// Test_replaceBytes_refuses_a_directory_target and its WriteString twin
// exist because replaceBytes and replaceString are two call-site wrappers
// around one shared WriteFile, not because their error paths differ. Mem
// writes in place rather than through a temp sibling it renames, so the
// error this raises is fs.ErrExist rather than the rename failure the OS
// adapter would raise for the same obstacle — peelWriteErr strips rwfs's
// "writefile blocked: " framing down to that bare sentinel, which is what
// this test pins: replaceBytes does not add its own naming on top of it,
// leaving that to whichever caller wraps the boundary.
func Test_replaceBytes_refuses_a_directory_target(t *testing.T) {
	fsys := newReplaceFS(t)
	require.NoError(t, fsys.Mkdir("blocked", 0o755))

	err := replaceBytes(fsys, "blocked", []byte("new"))

	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrExist)
}

func Test_replaceString_refuses_a_directory_target(t *testing.T) {
	fsys := newReplaceFS(t)
	require.NoError(t, fsys.Mkdir("blocked", 0o755))

	err := replaceString(fsys, "blocked", "new")

	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrExist)
}

// Test_replaceBytes_and_replaceString_write_the_same_bytes is the control
// that the two helpers differ only in the type they accept, so the
// []byte(data) conversion replaceString makes before calling WriteFile is
// not quietly writing something else. It uses a bare filesystem: neither
// write here is expected to fail, so it has no need for the "blocked"
// fixture the two tests above target.
func Test_replaceBytes_and_replaceString_write_the_same_bytes(t *testing.T) {
	fsys := newReplaceFS(t)

	require.NoError(t, replaceBytes(fsys, "from-bytes", []byte("identical payload")))
	require.NoError(t, replaceString(fsys, "from-string", "identical payload"))

	fromBytes, bytesErr := fsys.ReadFile("from-bytes")
	require.NoError(t, bytesErr)
	fromString, stringErr := fsys.ReadFile("from-string")
	require.NoError(t, stringErr)

	assert.Equal(t, string(fromBytes), string(fromString))
	assert.Equal(t, "identical payload", string(fromString))
}
