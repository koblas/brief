package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/rwfs/rwfstest"
	"github.com/stretchr/testify/require"
)

// diskFSUnderTempDir adapts diskFS to a name space rooted at a fresh
// t.TempDir() rather than diskFS's own production "/" root: base is
// fsName(tempDir), and every method joins it onto the contract's own
// relative name — via plain string concatenation, never filepath.Join or
// path.Clean, so a name the contract deliberately makes invalid (a
// dot-dot, a doubled separator) still reaches diskFS's real abs() mapping
// unmodified rather than being normalized away by the wrapper itself.
// diskFS.abs is the exact inverse of fsName (fs.go's own doc comment), so
// this join, followed by diskFS's own abs(), reconstructs the real,
// absolute temp-dir path the contract's writes and reads actually land at
// — this is the only way to contract-test diskFS at all without touching
// the real "/" its zero value is otherwise rooted at.
type diskFSUnderTempDir struct {
	diskFS

	base string
}

func (d diskFSUnderTempDir) join(name string) string {
	if name == "." {
		return d.base
	}

	return d.base + "/" + name
}

func (d diskFSUnderTempDir) Open(name string) (fs.File, error) { return d.diskFS.Open(d.join(name)) }

func (d diskFSUnderTempDir) ReadFile(name string) ([]byte, error) {
	return d.diskFS.ReadFile(d.join(name))
}

func (d diskFSUnderTempDir) ReadDir(name string) ([]fs.DirEntry, error) {
	return d.diskFS.ReadDir(d.join(name))
}

func (d diskFSUnderTempDir) Stat(name string) (fs.FileInfo, error) {
	return d.diskFS.Stat(d.join(name))
}

func (d diskFSUnderTempDir) Lstat(name string) (fs.FileInfo, error) {
	return d.diskFS.Lstat(d.join(name))
}

func (d diskFSUnderTempDir) ReadLink(name string) (string, error) {
	return d.diskFS.ReadLink(d.join(name))
}

func (d diskFSUnderTempDir) Mkdir(name string, perm fs.FileMode) error {
	return d.diskFS.Mkdir(d.join(name), perm)
}

func (d diskFSUnderTempDir) MkdirAll(name string, perm fs.FileMode) error {
	return d.diskFS.MkdirAll(d.join(name), perm)
}

func (d diskFSUnderTempDir) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return d.diskFS.WriteFile(d.join(name), data, perm)
}

func (d diskFSUnderTempDir) Remove(name string) error { return d.diskFS.Remove(d.join(name)) }

func (d diskFSUnderTempDir) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	return d.diskFS.CreateExclusive(d.join(name), data, perm)
}

func (d diskFSUnderTempDir) OpenRoot(name string) (rwfs.FS, error) {
	return d.diskFS.OpenRoot(d.join(name))
}

var _ rwfs.FS = diskFSUnderTempDir{}

// newDiskFSContractFS returns diskFS, viewed through a fresh t.TempDir(),
// pre-seeded with the same symlink-to-file and symlink-to-directory
// fixtures rwfs' own contract fixtures use, created with plain os.Symlink
// since diskFS (like OS) has no Symlink-creating method of its own.
func newDiskFSContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	dir := t.TempDir()
	d := diskFSUnderTempDir{base: fsName(dir)}

	require.NoError(t, d.WriteFile("symlink-target.txt", []byte("target contents"), 0o600))
	require.NoError(t, os.Symlink("symlink-target.txt", filepath.Join(dir, "a-symlink")))

	require.NoError(t, d.MkdirAll("dir-target", 0o755))
	require.NoError(t, os.Symlink("dir-target", filepath.Join(dir, "dir-symlink")))

	return d
}

// Test_diskFS_contract runs rwfs' own read/write contract against diskFS,
// proving it satisfies rwfs.FS beyond the compile-time var _ rwfs.FS
// assertion (fs.go) — every documented divergence declared through its own
// Option, cited to fs.go's and doc.go's own doc comments rather than
// re-derived here.
func Test_diskFS_contract(t *testing.T) {
	rwfstest.Contract(t, newDiskFSContractFS,
		rwfstest.SkipInvalidNames(
			"diskFS.abs never checks fs.ValidPath: setup deliberately keeps every write against the raw, "+
				"unconfined \"/\"-rooted os.* call it always made (fs.go's own diskFS doc comment); "+
				"Mkdir(\"\") maps to os.Mkdir(\"/\"), which reports EEXIST rather than fs.ErrInvalid",
		),
		rwfstest.PathIsAbsolute(
			"diskFS's *fs.PathError.Path is the absolute OS path abs() built, never the caller's own name; "+
				"changing it would change setup.go's writeThrough and uninstall.go's fsys.Remove wraps, both of "+
				"which reach cli stderr/--json output (fs.go's own doc comment)",
		),
		rwfstest.NoOpenRoot(
			"diskFS.OpenRoot is unimplemented (returns errors.ErrUnsupported); no Init or Uninstall call site "+
				"ever calls it — bound_agent.go's confinedAgentFile opens its own os.Root directly (fs.go's own "+
				"diskFS.OpenRoot doc comment)",
		),
		rwfstest.MkdirAllLeafSentinel(
			"diskFS.MkdirAll goes through os.MkdirAll directly, not os.Root.MkdirAll: confirmed on go1.27.1/darwin "+
				"that os.MkdirAll on a leaf that already exists as a file reports syscall.ENOTDIR, not fs.ErrExist",
		),
		rwfstest.WriteFileAncestorSentinel(
			"diskFS.WriteFile opens name's parent via os.OpenRoot(dir): confirmed on go1.27.1/darwin that "+
				"os.OpenRoot on a dir that exists as a file reports a bare, unwrapped \"not a directory\" string, "+
				"not syscall.ENOTDIR",
		),
	)
}
