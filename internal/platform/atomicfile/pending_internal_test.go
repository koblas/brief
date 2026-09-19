package atomicfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_close_reports_a_failed_write_and_refuses_to_commit exercises the one
// branch no black-box test can reach: a Write that fails after the temp
// sibling was opened. It closes the underlying descriptor so the next Write
// hits a real EBADF from the kernel rather than a fake, then proves Close
// reports that failure, leaves the target's old content in place, and does
// not leave the temp sibling behind.
//
// This is white-box by necessity — there is no portable way to make a write
// to an open, valid descriptor fail from outside the package — and it is the
// control for the claim in Create's doc comment that an ignored Write error
// still cannot publish a truncated file. WriteFile depends on that claim: it
// returns only Close's error.
func Test_close_reports_a_failed_write_and_refuses_to_commit(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	t.Cleanup(func() { _ = root.Close() })

	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))

	w, createErr := Create(root, "target.txt", 0o644)
	require.NoError(t, createErr)
	require.NoError(t, w.file.Close())

	_, writeErr := w.Write([]byte("new"))
	require.Error(t, writeErr)

	closeErr := w.Close()

	require.Error(t, closeErr)
	require.ErrorIs(t, closeErr, writeErr, "Close must report the first write failure")
	assert.Contains(t, closeErr.Error(), "target.txt not replaced",
		"Close must say it refused to commit, not merely echo the write error")

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(got), "a failed write must not be committed over the target")
	assert.NoFileExists(t, filepath.Join(dir, ".target.txt.brief-tmp"))
}

// Test_close_reports_a_leftover_temp_file_it_could_not_remove proves the
// cleanup failure is surfaced rather than swallowed. Making the directory
// read-only after the temp sibling is open fails both the rename and the
// removal that follows it, and the joined error has to carry each: the
// rename is why the write failed, and the leftover sibling contradicts this
// package's promise to leave none behind.
func Test_close_reports_a_leftover_temp_file_it_could_not_remove(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	t.Cleanup(func() { _ = root.Close() })

	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.Mkdir(target, 0o755))

	w, createErr := Create(root, "target.txt", 0o644)
	require.NoError(t, createErr)

	_, writeErr := w.Write([]byte("new"))
	require.NoError(t, writeErr)

	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	closeErr := w.Close()

	require.Error(t, closeErr)
	require.ErrorIs(t, closeErr, os.ErrPermission, "the failed removal must reach the caller")
	assert.Contains(t, closeErr.Error(), "remove temp file for target.txt")
	assert.Contains(t, closeErr.Error(), "write target.txt")
	assert.FileExists(t, filepath.Join(dir, ".target.txt.brief-tmp"), "the sibling the error reports must really be there")
}
