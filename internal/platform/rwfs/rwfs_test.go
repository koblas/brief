package rwfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/rwfs/rwfstest"
	"github.com/stretchr/testify/require"
)

func Test_os_adapter_contract(t *testing.T) {
	rwfstest.Contract(t, newOSContractFS)
}

func Test_mem_adapter_contract(t *testing.T) {
	rwfstest.Contract(t, newMemContractFS)
}

// newOSContractFS seeds a symlink-to-file and a symlink-to-directory
// fixture via os.Symlink, since rwfs.FS has no Symlink-creating method.
func newOSContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	dir := t.TempDir()
	fsys, err := rwfs.OpenOS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })

	require.NoError(t, fsys.WriteFile("symlink-target.txt", []byte("target contents"), 0o600))
	require.NoError(t, os.Symlink("symlink-target.txt", filepath.Join(dir, "a-symlink")))

	require.NoError(t, fsys.MkdirAll("dir-target", 0o755))
	require.NoError(t, os.Symlink("dir-target", filepath.Join(dir, "dir-symlink")))

	return fsys
}

// newMemContractFS seeds the same fixtures as newOSContractFS, via literal
// fstest.MapFS entries.
func newMemContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{
		"symlink-target.txt": {Data: []byte("target contents"), Mode: 0o600},
		"a-symlink":          {Data: []byte("symlink-target.txt"), Mode: fs.ModeSymlink | 0o777},
		"dir-target":         {Mode: fs.ModeDir | 0o755},
		"dir-symlink":        {Data: []byte("dir-target"), Mode: fs.ModeSymlink | 0o777},
	})
}
