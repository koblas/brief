package rwfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/require"
)

// The two tests in this file pin OS-only guarantees rwfs.Mem does not, and
// cannot economically, reproduce — see the divergences listed in doc.go.
// They have no Mem counterpart.

// Test_OS_refuses_a_symlink_that_escapes_the_root pins os.Root's
// confinement: a symlink entry inside the root whose target resolves
// outside it is refused rather than followed, protecting a caller that
// trusts "every read stays inside dir" even when the tree was not built by
// brief itself.
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

// skipIfRoot skips t when running as the root user, for a test whose
// premise is a permission check that root bypasses entirely — reporting a
// false pass rather than exercising the check.
func skipIfRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
}

// Test_OS_reports_a_permission_error_when_the_directory_forbids_writing
// pins that the OS adapter surfaces a real permission failure rather than
// silently succeeding, unlike Mem which never consults the mode it records.
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
