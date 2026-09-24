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

// Test_os_adapter_contract runs the shared contract against rwfs.OS, backed
// by a t.TempDir() directory. mk also seeds a symlink outside the FS
// interface itself — os.Root has no Symlink-creating method exposed through
// rwfs.FS, since no production call site needs one — using os.Symlink
// directly against the same directory rwfs.OpenOS is rooted at. rwfs.OS
// passes every rwfstest.Contract row unmodified: no Option is given.
func Test_os_adapter_contract(t *testing.T) {
	rwfstest.Contract(t, newOSContractFS)
}

// Test_mem_adapter_contract runs the shared contract against rwfs.Mem, the
// in-memory adapter. mk seeds the same symlink fixture as the OS variant,
// via a literal fstest.MapFS entry instead of a filesystem call. rwfs.Mem
// passes every rwfstest.Contract row unmodified: no Option is given.
func Test_mem_adapter_contract(t *testing.T) {
	rwfstest.Contract(t, newMemContractFS)
}

// newOSContractFS returns an *rwfs.OS rooted at a fresh t.TempDir(),
// pre-seeded with the symlink-to-file fixture the contract's symlink case
// expects and a symlink-to-directory fixture OpenRoot's follow case expects.
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

// newMemContractFS returns an *rwfs.Mem pre-seeded with the same symlink
// fixtures as newOSContractFS, via literal fstest.MapFS entries instead of
// filesystem calls.
func newMemContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{
		"symlink-target.txt": {Data: []byte("target contents"), Mode: 0o600},
		"a-symlink":          {Data: []byte("symlink-target.txt"), Mode: fs.ModeSymlink | 0o777},
		"dir-target":         {Mode: fs.ModeDir | 0o755},
		"dir-symlink":        {Data: []byte("dir-target"), Mode: fs.ModeSymlink | 0o777},
	})
}
