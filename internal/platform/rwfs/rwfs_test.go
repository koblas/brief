package rwfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertPathError requires err to be a *fs.PathError naming name — the
// wrapping every rwfs.FS write method promises, not an internal temp name a
// wrapped call happened to use.
func assertPathError(t *testing.T, err error, name string) {
	t.Helper()

	var pe *fs.PathError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, name, pe.Path)
	assert.NotEmpty(t, pe.Op)
}

// testContract exercises the rwfs.FS contract against a freshly constructed
// filesystem from mk, called once per subtest. Both the OS and Mem adapters
// run it and must pass identically. Grouped into one function per method
// family so each stays small enough to read on its own.
func testContract(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	// atomicfile's fresh-create path lets the process umask mask perm, the
	// same as os.OpenFile. Pinning the umask keeps the perm assertions
	// below from depending on the ambient value; nothing in this package
	// calls t.Parallel, so the pin is not visible to any concurrent test.
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	testContractWriteFile(t, mk)
	testContractMkdir(t, mk)
	testContractRemove(t, mk)
	testContractRead(t, mk)
	testContractInvalidNames(t, mk)

	t.Run("fs.TestFS validates a seeded tree", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("dir/sub", 0o755))
		require.NoError(t, fsys.WriteFile("dir/sub/file.txt", []byte("x"), 0o600))
		require.NoError(t, fsys.WriteFile("root.txt", []byte("x"), 0o600))

		assert.NoError(t, fstest.TestFS(fsys, "dir/sub/file.txt", "root.txt"))
	})
}

// testContractWriteFile covers WriteFile: create, replace, the perm rule on
// each, and the missing/blocked-parent error cases.
func testContractWriteFile(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	t.Run("creates a file and reads its content back", func(t *testing.T) {
		fsys := mk(t)

		require.NoError(t, fsys.WriteFile("a.txt", []byte("hello"), 0o600))

		got, err := fsys.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))
	})

	t.Run("replace overwrites the previous content", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("a.txt", []byte("old"), 0o600))

		require.NoError(t, fsys.WriteFile("a.txt", []byte("new"), 0o600))

		got, err := fsys.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "new", string(got))
	})

	t.Run("create applies the given permission", func(t *testing.T) {
		fsys := mk(t)

		require.NoError(t, fsys.WriteFile("a.txt", []byte("hello"), 0o600))

		info, err := fsys.Stat("a.txt")
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("replace keeps the target's existing permission and ignores the new perm argument", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("a.txt", []byte("old"), 0o400))

		require.NoError(t, fsys.WriteFile("a.txt", []byte("new"), 0o644))

		info, err := fsys.Stat("a.txt")
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o400), info.Mode().Perm())
	})

	t.Run("WriteFile fails with a missing parent", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.WriteFile("missing/child.txt", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, err, "missing/child.txt")
	})

	writeFileAncestorCases := []struct {
		name string
		path string
	}{
		{name: "immediate parent is a file", path: "a.txt/child.txt"},
		{name: "grandparent is a file", path: "a.txt/b/child.txt"},
	}
	for _, c := range writeFileAncestorCases {
		t.Run("WriteFile fails when "+c.name, func(t *testing.T) {
			fsys := mk(t)
			require.NoError(t, fsys.WriteFile("a.txt", []byte("x"), 0o600))

			err := fsys.WriteFile(c.path, []byte("x"), 0o600)

			require.Error(t, err)
			require.NotErrorIs(t, err, fs.ErrNotExist)
			require.ErrorIs(t, err, syscall.ENOTDIR)
			assertPathError(t, err, c.path)
		})
	}
}

// testContractMkdir covers Mkdir and MkdirAll: idempotency, nesting, and
// the distinct error each returns when a segment already exists.
func testContractMkdir(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	t.Run("MkdirAll is idempotent on an existing directory", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("d", 0o755))

		err := fsys.MkdirAll("d", 0o755)

		require.NoError(t, err)
		info, statErr := fsys.Stat("d")
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("MkdirAll creates nested directories", func(t *testing.T) {
		fsys := mk(t)

		require.NoError(t, fsys.MkdirAll("a/b/c", 0o755))

		for _, name := range []string{"a", "a/b", "a/b/c"} {
			info, err := fsys.Stat(name)
			require.NoError(t, err, name)
			assert.True(t, info.IsDir(), name)
		}
	})

	t.Run("MkdirAll fails when the leaf already exists as a file", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))

		err := fsys.MkdirAll("f.txt", 0o755)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, err, "f.txt")
	})

	t.Run("MkdirAll fails when an intermediate segment is a file", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))

		err := fsys.MkdirAll("f.txt/b/c", 0o755)

		require.Error(t, err)
		require.NotErrorIs(t, err, fs.ErrExist)
		require.ErrorIs(t, err, syscall.ENOTDIR)
		assertPathError(t, err, "f.txt/b/c")
	})

	t.Run("Mkdir fails when the name already exists", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.Mkdir("d", 0o755))

		err := fsys.Mkdir("d", 0o755)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, err, "d")
	})

	t.Run("Mkdir fails when the parent does not exist", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.Mkdir("missing/child", 0o755)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, err, "missing/child")
	})
}

// testContractRemove covers Remove: a file, an empty directory, a
// non-empty directory, and a missing name.
func testContractRemove(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	t.Run("Remove deletes a file", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("a.txt", []byte("x"), 0o600))

		require.NoError(t, fsys.Remove("a.txt"))

		_, err := fsys.Stat("a.txt")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("Remove deletes an empty directory", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("d", 0o755))

		require.NoError(t, fsys.Remove("d"))

		_, err := fsys.Stat("d")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("Remove fails on a non-empty directory", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("d", 0o755))
		require.NoError(t, fsys.WriteFile("d/f.txt", []byte("x"), 0o600))

		err := fsys.Remove("d")

		require.Error(t, err)
		require.ErrorIs(t, err, syscall.ENOTEMPTY)
		assertPathError(t, err, "d")
		info, statErr := fsys.Stat("d")
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("Remove fails when the name does not exist", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.Remove("nope.txt")

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, err, "nope.txt")
	})
}

// testContractRead covers the read side: sorted directory listing including
// an explicit empty directory, Stat/Lstat agreement on non-symlink entries,
// and Lstat/ReadLink on a symlink entry.
func testContractRead(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	t.Run("ReadDir lists entries in sorted order, including an explicit empty directory", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("list", 0o755))
		require.NoError(t, fsys.MkdirAll("list/z-empty-dir", 0o755))
		require.NoError(t, fsys.WriteFile("list/b.txt", []byte("x"), 0o600))
		require.NoError(t, fsys.WriteFile("list/a.txt", []byte("x"), 0o600))

		entries, err := fsys.ReadDir("list")
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Equal(t, []string{"a.txt", "b.txt", "z-empty-dir"}, names)

		emptyEntries, emptyErr := fsys.ReadDir("list/z-empty-dir")
		require.NoError(t, emptyErr)
		assert.Empty(t, emptyEntries)
	})

	statLstatCases := []struct {
		name string
		seed func(t *testing.T, fsys rwfs.FS)
		path string
	}{
		{
			name: "a regular file",
			seed: func(t *testing.T, fsys rwfs.FS) {
				t.Helper()
				require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))
			},
			path: "f.txt",
		},
		{
			name: "a directory",
			seed: func(t *testing.T, fsys rwfs.FS) {
				t.Helper()
				require.NoError(t, fsys.MkdirAll("d", 0o755))
			},
			path: "d",
		},
	}
	for _, c := range statLstatCases {
		t.Run("Stat and Lstat agree on "+c.name, func(t *testing.T) {
			fsys := mk(t)
			c.seed(t, fsys)

			statInfo, statErr := fsys.Stat(c.path)
			require.NoError(t, statErr)
			lstatInfo, lstatErr := fsys.Lstat(c.path)
			require.NoError(t, lstatErr)

			assert.Equal(t, statInfo.IsDir(), lstatInfo.IsDir())
			assert.Equal(t, statInfo.Name(), lstatInfo.Name())
		})
	}

	t.Run("Lstat and ReadLink report a symlink entry without following it", func(t *testing.T) {
		fsys := mk(t)

		info, err := fsys.Lstat("a-symlink")
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&fs.ModeSymlink)

		target, err := fsys.ReadLink("a-symlink")
		require.NoError(t, err)
		assert.Equal(t, "symlink-target.txt", target)
	})
}

// testContractInvalidNames covers every write method's response to a name
// fs.ValidPath rejects.
func testContractInvalidNames(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	invalidNameCases := []struct {
		label string
		name  string
	}{
		{label: "empty", name: ""},
		{label: "dot-dot", name: ".."},
		{label: "absolute", name: "/abs.txt"},
		{label: "traverses via dot-dot", name: "a/../b.txt"},
		{label: "empty path segment", name: "a//b.txt"},
	}
	for _, c := range invalidNameCases {
		t.Run("invalid name: "+c.label, func(t *testing.T) {
			fsys := mk(t)

			require.ErrorIs(t, fsys.WriteFile(c.name, []byte("x"), 0o600), fs.ErrInvalid)
			require.ErrorIs(t, fsys.Mkdir(c.name, 0o755), fs.ErrInvalid)
			require.ErrorIs(t, fsys.MkdirAll(c.name, 0o755), fs.ErrInvalid)
			require.ErrorIs(t, fsys.Remove(c.name), fs.ErrInvalid)
		})
	}
}

// Test_os_adapter_contract runs the shared contract against rwfs.OS, backed
// by a t.TempDir() directory. mk also seeds a symlink outside the FS
// interface itself — os.Root has no Symlink-creating method exposed through
// rwfs.FS, since no production call site needs one — using os.Symlink
// directly against the same directory rwfs.OpenOS is rooted at.
func Test_os_adapter_contract(t *testing.T) {
	testContract(t, newOSContractFS)
}

// Test_mem_adapter_contract runs the shared contract against rwfs.Mem, the
// in-memory adapter. mk seeds the same symlink fixture as the OS variant,
// via a literal fstest.MapFS entry instead of a filesystem call.
func Test_mem_adapter_contract(t *testing.T) {
	testContract(t, newMemContractFS)
}

// newOSContractFS returns an *rwfs.OS rooted at a fresh t.TempDir(),
// pre-seeded with the symlink fixture the contract's symlink case expects.
func newOSContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	dir := t.TempDir()
	fsys, err := rwfs.OpenOS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })

	require.NoError(t, fsys.WriteFile("symlink-target.txt", []byte("target contents"), 0o600))
	require.NoError(t, os.Symlink("symlink-target.txt", filepath.Join(dir, "a-symlink")))

	return fsys
}

// newMemContractFS returns an *rwfs.Mem pre-seeded with the same symlink
// fixture as newOSContractFS, via a literal fstest.MapFS entry instead of a
// filesystem call.
func newMemContractFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{
		"symlink-target.txt": {Data: []byte("target contents"), Mode: 0o600},
		"a-symlink":          {Data: []byte("symlink-target.txt"), Mode: fs.ModeSymlink | 0o777},
	})
}
