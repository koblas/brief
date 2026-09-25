package rwfstest

import (
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DirOpener is the minimal directory-descending read shape ReadOpenRootContract
// exercises. An adapter whose OpenRoot returns a different named interface
// type needs a small local wrapper to satisfy this one; see
// internal/assemble's read_contract_internal_test.go.
type DirOpener interface {
	FS() fs.FS
	Lstat(name string) (fs.FileInfo, error)
	OpenRoot(name string) (DirOpener, error)
}

// ReadContract exercises the read side of rwfs.FS's contract against a
// freshly seeded fs.FS from mk. It is Contract's read-only twin, for an
// adapter that only ever reads.
func ReadContract(t *testing.T, mk func(t *testing.T) fs.FS, opts ...Option) {
	t.Helper()

	o := resolve(opts)
	logExceptions(t, o)

	t.Run("reads a file's content back", func(t *testing.T) {
		fsys := mk(t)

		got, err := fs.ReadFile(fsys, "a.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))
	})

	t.Run("ReadDir lists entries in sorted order", func(t *testing.T) {
		fsys := mk(t)

		entries, err := fs.ReadDir(fsys, "list")
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Equal(t, []string{"a.txt", "b.txt"}, names)
	})

	t.Run("Stat and Lstat agree on a directory", func(t *testing.T) {
		fsys := mk(t)

		statInfo, err := fs.Stat(fsys, "list")
		require.NoError(t, err)
		assert.True(t, statInfo.IsDir())
	})

	t.Run("reads fail when a proper ancestor is a file", func(t *testing.T) {
		fsys := mk(t)

		_, err := fs.ReadFile(fsys, "a.txt/sub")
		require.Error(t, err)
	})
}

// ReadOpenRootContract exercises OpenRoot's read-side contract against a
// fresh DirOpener from mk: descending into an existing subdirectory, and
// the missing-name and file-in-place error cases.
func ReadOpenRootContract(t *testing.T, mk func(t *testing.T) DirOpener, opts ...Option) {
	t.Helper()

	o := resolve(opts)
	logExceptions(t, o)

	t.Run("OpenRoot descends into an existing subdirectory and reads relative to it", func(t *testing.T) {
		d := mk(t)

		sub, err := d.OpenRoot("sub")
		require.NoError(t, err)

		got, err := fs.ReadFile(sub.FS(), "a.txt")
		require.NoError(t, err)
		assert.Equal(t, "nested", string(got))
	})

	t.Run("OpenRoot fails with fs.ErrNotExist when name does not exist", func(t *testing.T) {
		d := mk(t)

		_, err := d.OpenRoot("missing")

		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("OpenRoot fails when name exists as a file", func(t *testing.T) {
		d := mk(t)

		_, err := d.OpenRoot("a.txt")

		require.Error(t, err)
		require.NotErrorIs(t, err, fs.ErrNotExist)
		if o.openRootNotDirUnclassified == "" {
			require.ErrorIs(t, err, syscall.ENOTDIR)
		}
	})

	t.Run("Lstat reports a top-level entry without following it", func(t *testing.T) {
		d := mk(t)

		info, err := d.Lstat("a.txt")
		require.NoError(t, err)
		assert.False(t, info.IsDir())
	})
}
