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
// still cannot publish a truncated file.
func Test_close_reports_a_failed_write_and_refuses_to_commit(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	t.Cleanup(func() { _ = root.Close() })

	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))

	w, createErr := Create(root, "target.txt", 0o644)
	require.NoError(t, createErr)

	pending, ok := w.(*pendingFile)
	require.True(t, ok)
	require.NoError(t, pending.file.Close())

	_, writeErr := pending.Write([]byte("new"))
	require.Error(t, writeErr)

	closeErr := pending.Close()

	require.Error(t, closeErr)
	assert.Equal(t, writeErr, closeErr, "Close must report the first write failure")

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(got), "a failed write must not be committed over the target")
	assert.NoFileExists(t, filepath.Join(dir, ".target.txt.brief-tmp"))
}
