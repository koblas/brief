package rwfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin OS-only guarantees rwfs.Mem does not
// reproduce — see doc.go. They have no Mem counterpart.

func Test_OS_refuses_a_symlink_that_escapes_the_root(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600))

	dir := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "escape")))

	fsys, err := rwfs.OpenOS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })

	_, err = fsys.ReadFile("escape")

	require.Error(t, err)
}

// The symlink target is relative, so escaping the root is the one variable
// this test isolates from the shared contract's "follows a symlink to a
// directory" case.
func Test_OS_OpenRoot_refuses_a_symlink_that_escapes_the_root(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outside, "secret"), 0o755))

	relTarget, err := filepath.Rel(dir, filepath.Join(outside, "secret"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relTarget, filepath.Join(dir, "escape")))

	fsys, err := rwfs.OpenOS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })

	_, err = fsys.OpenRoot("escape")

	require.Error(t, err)
	assert.NotErrorIs(t, err, syscall.ENOTDIR) // must not be mistaken for the file-in-place case
}

// skipIfRoot skips t when running as root, which bypasses permission checks.
func skipIfRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
}

func Test_OS_reports_a_permission_error_when_the_directory_forbids_writing(t *testing.T) {
	skipIfRoot(t)

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "locked"), 0o500))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "locked"), 0o700) })

	fsys, err := rwfs.OpenOS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })

	err = fsys.WriteFile("locked/child.txt", []byte("x"), 0o600)

	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrPermission)
}
