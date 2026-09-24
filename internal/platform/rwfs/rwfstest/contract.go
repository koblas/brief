package rwfstest

import (
	"io/fs"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertPathError requires err to be a *fs.PathError with a non-empty Op,
// naming name — the wrapping every rwfs.FS write method promises — unless
// o carries PathIsAbsolute, in which case Path is only required to be a
// non-empty absolute path, per that Option's own doc comment.
func assertPathError(t *testing.T, o options, err error, name string) {
	t.Helper()

	var pe *fs.PathError
	require.ErrorAs(t, err, &pe)
	assert.NotEmpty(t, pe.Op)

	if o.pathIsAbsolute != "" {
		assert.True(t, filepath.IsAbs(pe.Path), "PathIsAbsolute: pe.Path %q must be an absolute path", pe.Path)

		return
	}

	assert.Equal(t, name, pe.Path)
}

// requireENOTDIR requires err to be a *fs.PathError naming name whose chain
// matches syscall.ENOTDIR — the portable POSIX errno both os.Root.FS() (
// confirmed on darwin) and rwfs.Mem report when a read reaches through an
// ancestor that exists but is not a directory.
func requireENOTDIR(t *testing.T, o options, err error, name string) {
	t.Helper()

	require.Error(t, err)
	require.ErrorIs(t, err, syscall.ENOTDIR)
	assertPathError(t, o, err, name)
}

// Contract exercises the rwfs.FS contract against a freshly constructed
// filesystem from mk, called once per subtest. rwfs.OS and rwfs.Mem run it
// with no opts and must pass every row unmodified; an adapter that
// deliberately diverges from part of the contract declares each divergence
// through opts instead of failing or being silently narrowed.
func Contract(t *testing.T, mk func(t *testing.T) rwfs.FS, opts ...Option) {
	t.Helper()

	o := resolve(opts)
	logExceptions(t, o)

	// atomicfile's fresh-create path lets the process umask mask perm, the
	// same as os.OpenFile. Pinning the umask keeps the perm assertions
	// below from depending on the ambient value; nothing in this package
	// calls t.Parallel, so the pin is not visible to any concurrent test.
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	testContractWriteFile(t, mk, o)
	testContractMkdir(t, mk, o)
	testContractRemove(t, mk, o)
	testContractRead(t, mk)
	testContractReadThroughFileAncestor(t, mk, o)
	testContractCreateExclusive(t, mk, o)
	testContractOpenRootSection(t, mk, o)
	testContractInvalidNames(t, mk, o)

	t.Run("fs.TestFS validates a seeded tree", func(t *testing.T) {
		if o.skipInvalidNames != "" {
			t.Skip(o.skipInvalidNames)
		}

		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("dir/sub", 0o755))
		require.NoError(t, fsys.WriteFile("dir/sub/file.txt", []byte("x"), 0o600))
		require.NoError(t, fsys.WriteFile("root.txt", []byte("x"), 0o600))

		assert.NoError(t, fstest.TestFS(fsys, "dir/sub/file.txt", "root.txt"))
	})
}

// testContractWriteFile covers WriteFile: create, replace, the perm rule on
// each, and the missing/blocked-parent error cases.
func testContractWriteFile(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
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
		assertPathError(t, o, err, "d")
		info, statErr := fsys.Stat("d")
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("WriteFile fails with a missing parent", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.WriteFile("missing/child.txt", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, o, err, "missing/child.txt")
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
			if o.writeFileAncestorSentinel == "" {
				require.ErrorIs(t, err, syscall.ENOTDIR)
			}
			assertPathError(t, o, err, c.path)
		})
	}
}

// testContractMkdir covers Mkdir and MkdirAll: idempotency, nesting, and
// the distinct error each returns when a segment already exists.
func testContractMkdir(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
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

		if o.mkdirAllLeafSentinel != "" {
			require.ErrorIs(t, err, syscall.ENOTDIR)
		} else {
			require.ErrorIs(t, err, fs.ErrExist)
		}

		assertPathError(t, o, err, "f.txt")
	})

	t.Run("MkdirAll fails when an intermediate segment is a file", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))

		err := fsys.MkdirAll("f.txt/b/c", 0o755)

		require.Error(t, err)
		require.NotErrorIs(t, err, fs.ErrExist)
		require.ErrorIs(t, err, syscall.ENOTDIR)
		assertPathError(t, o, err, "f.txt/b/c")
	})

	t.Run("Mkdir fails when the name already exists", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.Mkdir("d", 0o755))

		err := fsys.Mkdir("d", 0o755)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assertPathError(t, o, err, "d")
	})

	t.Run("Mkdir fails when the parent does not exist", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.Mkdir("missing/child", 0o755)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, o, err, "missing/child")
	})
}

// testContractRemove covers Remove: a file, an empty directory, a
// non-empty directory, and a missing name.
func testContractRemove(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
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
		assertPathError(t, o, err, "d")
		info, statErr := fsys.Stat("d")
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("Remove fails when the name does not exist", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.Remove("nope.txt")

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, o, err, "nope.txt")
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
func testContractReadThroughFileAncestor(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
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
			requireENOTDIR(t, o, err, c.path)

			_, err = fsys.Lstat(c.path)
			requireENOTDIR(t, o, err, c.path)

			_, err = fsys.ReadFile(c.path)
			requireENOTDIR(t, o, err, c.path)

			_, err = fsys.ReadDir(c.path)
			requireENOTDIR(t, o, err, c.path)

			_, err = fsys.Open(c.path)
			requireENOTDIR(t, o, err, c.path)
		})
	}
}

// testContractCreateExclusive covers CreateExclusive: fresh create, refusing
// an existing entry of any type, and the missing-parent case. Confirmed
// empirically against os.Root.OpenFile with O_EXCL: an existing file, an
// existing directory, and a missing parent each report the same sentinel
// this pins.
func testContractCreateExclusive(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
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
		assertPathError(t, o, err, "a.txt")

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
		assertPathError(t, o, err, "d")
	})

	t.Run("fails with fs.ErrNotExist when the parent is missing", func(t *testing.T) {
		fsys := mk(t)

		err := fsys.CreateExclusive("missing/child.txt", []byte("x"), 0o600)

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, o, err, "missing/child.txt")
	})
}

// testContractInvalidNames covers every write method's response to a name
// fs.ValidPath rejects. SkipInvalidNames skips the whole row; NoOpenRoot
// alone, without SkipInvalidNames, skips only the OpenRoot assertion inside
// it, so the other five write methods are still checked.
func testContractInvalidNames(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
	t.Helper()

	if o.skipInvalidNames != "" {
		t.Run("invalid names", func(t *testing.T) {
			t.Skip(o.skipInvalidNames)
		})

		return
	}

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

			if o.noOpenRoot == "" {
				_, err := fsys.OpenRoot(c.name)
				require.ErrorIs(t, err, fs.ErrInvalid)
			}
		})
	}
}

// testContractOpenRootSection runs testContractOpenRoot unless NoOpenRoot
// declares the adapter under test does not implement OpenRoot, in which
// case the whole row is skipped instead.
func testContractOpenRootSection(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
	t.Helper()

	if o.noOpenRoot != "" {
		t.Run("OpenRoot", func(t *testing.T) {
			t.Skip(o.noOpenRoot)
		})

		return
	}

	testContractOpenRoot(t, mk, o)
}

// testContractOpenRoot covers OpenRoot: the missing, file-in-place and
// symlink-to-non-directory error cases; a write through a view landing in
// the parent's own tree and vice versa; nesting a view inside a view;
// OpenRoot("."); following a symlink to a directory; and that a view's read
// methods use names relative to the view rather than the parent.
func testContractOpenRoot(t *testing.T, mk func(t *testing.T) rwfs.FS, o options) {
	t.Helper()

	t.Run("fails with fs.ErrNotExist when name does not exist", func(t *testing.T) {
		fsys := mk(t)

		_, err := fsys.OpenRoot("missing")

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
		assertPathError(t, o, err, "missing")
	})

	t.Run("fails when name exists as a file", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.WriteFile("f.txt", []byte("x"), 0o600))

		_, err := fsys.OpenRoot("f.txt")

		requireENOTDIR(t, o, err, "f.txt")
	})

	t.Run("fails when name is a symlink to a non-directory", func(t *testing.T) {
		fsys := mk(t) // pre-seeded with a-symlink -> symlink-target.txt, a regular file

		_, err := fsys.OpenRoot("a-symlink")

		requireENOTDIR(t, o, err, "a-symlink")
	})

	t.Run("follows a symlink to a directory", func(t *testing.T) {
		fsys := mk(t) // pre-seeded with dir-symlink -> dir-target, a directory

		view, err := fsys.OpenRoot("dir-symlink")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		require.NoError(t, view.WriteFile("a.txt", []byte("via symlink"), 0o600))

		got, err := fsys.ReadFile("dir-target/a.txt")
		require.NoError(t, err)
		assert.Equal(t, "via symlink", string(got))
	})

	t.Run("a write through the view is visible from the parent and reads back through the view", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("sub", 0o755))

		view, err := fsys.OpenRoot("sub")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		require.NoError(t, view.WriteFile("a.txt", []byte("from view"), 0o600))

		fromParent, err := fsys.ReadFile("sub/a.txt")
		require.NoError(t, err)
		assert.Equal(t, "from view", string(fromParent))

		fromView, err := view.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "from view", string(fromView))
	})

	t.Run("a write through the parent is visible through an existing view", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("sub", 0o755))
		view, err := fsys.OpenRoot("sub")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		require.NoError(t, fsys.WriteFile("sub/a.txt", []byte("from parent"), 0o600))

		got, err := view.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "from parent", string(got))
	})

	t.Run("OpenRoot of a view opens relative to the view, not the parent", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("a/b", 0o755))
		viewA, err := fsys.OpenRoot("a")
		require.NoError(t, err)
		t.Cleanup(func() { _ = viewA.Close() })

		viewB, err := viewA.OpenRoot("b")
		require.NoError(t, err)
		t.Cleanup(func() { _ = viewB.Close() })
		require.NoError(t, viewB.WriteFile("c.txt", []byte("nested"), 0o600))

		got, err := fsys.ReadFile("a/b/c.txt")
		require.NoError(t, err)
		assert.Equal(t, "nested", string(got))
	})

	t.Run(`OpenRoot(".") returns a view equivalent to the parent's own root`, func(t *testing.T) {
		fsys := mk(t)

		view, err := fsys.OpenRoot(".")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		require.NoError(t, view.WriteFile("a.txt", []byte("via dot"), 0o600))

		got, err := fsys.ReadFile("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "via dot", string(got))
	})

	t.Run("a view's ReadDir, Stat and Lstat report names relative to the view", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("sub/child", 0o755))
		require.NoError(t, fsys.WriteFile("sub/a.txt", []byte("x"), 0o600))

		view, err := fsys.OpenRoot("sub")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		entries, err := view.ReadDir(".")
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Equal(t, []string{"a.txt", "child"}, names)

		statInfo, err := view.Stat("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "a.txt", statInfo.Name())

		lstatInfo, err := view.Lstat("a.txt")
		require.NoError(t, err)
		assert.Equal(t, "a.txt", lstatInfo.Name())
	})

	t.Run("fs.TestFS validates a seeded view", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("sub/child", 0o755))
		view, err := fsys.OpenRoot("sub")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })
		require.NoError(t, view.WriteFile("root.txt", []byte("x"), 0o600))
		require.NoError(t, view.WriteFile("child/file.txt", []byte("x"), 0o600))

		assert.NoError(t, fstest.TestFS(view, "root.txt", "child/file.txt"))
	})

	t.Run("a view rejects an invalid name the same way the root does", func(t *testing.T) {
		fsys := mk(t)
		require.NoError(t, fsys.MkdirAll("sub", 0o755))
		view, err := fsys.OpenRoot("sub")
		require.NoError(t, err)
		t.Cleanup(func() { _ = view.Close() })

		require.ErrorIs(t, view.WriteFile("..", []byte("x"), 0o600), fs.ErrInvalid)
	})
}
