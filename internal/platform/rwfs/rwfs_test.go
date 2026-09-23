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

// requireENOTDIR requires err to be a *fs.PathError naming name whose chain
// matches syscall.ENOTDIR — the portable POSIX errno both os.Root.FS() (
// confirmed on darwin) and rwfs.Mem report when a read reaches through an
// ancestor that exists but is not a directory.
func requireENOTDIR(t *testing.T, err error, name string) {
	t.Helper()

	require.Error(t, err)
	require.ErrorIs(t, err, syscall.ENOTDIR)
	assertPathError(t, err, name)
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
	testContractReadThroughFileAncestor(t, mk)
	testContractCreateExclusive(t, mk)
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

	// replacing a symlink applies the given perm rather than the symlink's
	// own — confirmed empirically against os.Root: renaming a fresh regular
	// file over a symlink's name replaces the link with the new file, so
	// replaceMode's IsRegular() gate must not treat a symlink as the
	// "existing" case WriteFile otherwise preserves.
	t.Run("replace over a symlink applies the given perm and turns it into a regular file", func(t *testing.T) {
		fsys := mk(t) // pre-seeded with a-symlink -> symlink-target.txt, mode ModeSymlink|0o777

		require.NoError(t, fsys.WriteFile("a-symlink", []byte("new contents"), 0o600))

		info, err := fsys.Lstat("a-symlink")
		require.NoError(t, err)
		assert.Zero(t, info.Mode()&fs.ModeSymlink, "replace must not leave the entry a symlink")
		assert.True(t, info.Mode().IsRegular())
		assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())

		got, err := fsys.ReadFile("a-symlink")
		require.NoError(t, err)
		assert.Equal(t, "new contents", string(got))

		// the symlink's own former target is untouched.
		target, err := fsys.ReadFile("symlink-target.txt")
		require.NoError(t, err)
		assert.Equal(t, "target contents", string(target))
	})

	// confirmed empirically against os.Root: renaming a regular file over an
	// existing directory (empty or not) fails with EEXIST, not EISDIR.
	t.Run("WriteFile fails when name already exists as a directory", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("d", 0o755))

		err := fsys.WriteFile("d", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, err, "d")
		info, statErr := fsys.Stat("d")
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
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
		name    string
		seed    func(t *testing.T, fsys rwfs.FS)
		path    string
		wantDir bool
	}{
		{
			name: "a regular file",
			seed: func(t *testing.T, fsys rwfs.FS) {
				t.Helper()
				require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))
			},
			path:    "f.txt",
			wantDir: false,
		},
		{
			name: "a directory",
			seed: func(t *testing.T, fsys rwfs.FS) {
				t.Helper()
				require.NoError(t, fsys.MkdirAll("d", 0o755))
			},
			path:    "d",
			wantDir: true,
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

			assert.Equal(t, c.wantDir, statInfo.IsDir(), "Stat().IsDir()")
			assert.Equal(t, c.wantDir, lstatInfo.IsDir(), "Lstat().IsDir()")
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

// testContractReadThroughFileAncestor covers every read method's response
// to a name that reaches through a proper ancestor already occupied by a
// regular file — confirmed empirically against os.Root.FS() on darwin to
// report syscall.ENOTDIR, the same sentinel rwfs.Mem's notDirAncestor
// already reports for the write side.
func testContractReadThroughFileAncestor(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	ancestorCases := []struct {
		name string
		path string
	}{
		{name: "immediate parent is a file", path: "f.txt/sub"},
		{name: "grandparent is a file", path: "f.txt/sub/deep"},
	}
	for _, c := range ancestorCases {
		t.Run("reads fail when "+c.name, func(t *testing.T) {
			fsys := mk(t)
			require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))

			_, err := fsys.Stat(c.path)
			requireENOTDIR(t, err, c.path)

			_, err = fsys.Lstat(c.path)
			requireENOTDIR(t, err, c.path)

			_, err = fsys.ReadFile(c.path)
			requireENOTDIR(t, err, c.path)

			_, err = fsys.ReadDir(c.path)
			requireENOTDIR(t, err, c.path)

			_, err = fsys.Open(c.path)
			requireENOTDIR(t, err, c.path)
		})
	}
}

// testContractCreateExclusive covers CreateExclusive: fresh create, refusing
// an existing entry of any type, and the missing-parent case. Confirmed
// empirically against os.Root.OpenFile with O_EXCL: an existing file, an
// existing directory, and a missing parent each report the same sentinel
// this pins.
func testContractCreateExclusive(t *testing.T, mk func(t *testing.T) rwfs.FS) {
	t.Helper()

	t.Run("creates a file with the given content and permission", func(t *testing.T) {
		fsys := mk(t)

		require.NoError(t, fsys.CreateExclusive("a.txt", []byte("hello"), 0o600))

		got, err := fsys.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))

		info, err := fsys.Stat("a.txt")
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("fails with fs.ErrExist when a file already exists", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.CreateExclusive("a.txt", []byte("first"), 0o600))

		err := fsys.CreateExclusive("a.txt", []byte("second"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, err, "a.txt")

		got, readErr := fsys.ReadFile("a.txt")
		require.NoError(t, readErr)
		assert.Equal(t, "first", string(got), "a refused create must not touch the existing content")
	})

	t.Run("fails with fs.ErrExist when a directory already exists at name", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("d", 0o755))

		err := fsys.CreateExclusive("d", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, err, "d")
	})

	t.Run("fails with fs.ErrNotExist when the parent is missing", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.CreateExclusive("missing/child.txt", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, err, "missing/child.txt")
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
			require.ErrorIs(t, fsys.CreateExclusive(c.name, []byte("x"), 0o600), fs.ErrInvalid)
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
