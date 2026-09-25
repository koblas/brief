package scaffold

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// White-box: replaceBytes/replaceString are unexported wrappers around
// rwfs.FS.WriteFile; this file only pins that they reach it identically.
func newReplaceFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{})
}

// Mem raises fs.ErrExist for this obstacle (not the OS adapter's rename
// failure); peelWriteErr's stripping down to that bare sentinel is what
// this pins.
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
