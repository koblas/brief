package atomicfile_test

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRoot returns an *os.Root rooted at a fresh temporary directory.
func openRoot(t *testing.T) (*os.Root, string) {
	t.Helper()

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	return root, dir
}

func Test_Create_makes_the_content_visible_only_when_close_commits_it(t *testing.T) {
	root, dir := openRoot(t)
	target := filepath.Join(dir, "target.txt")

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)

	_, writeErr := w.Write([]byte("hello"))
	require.NoError(t, writeErr)
	assert.NoFileExists(t, target, "target must not appear before Close commits")

	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "hello", string(got))
}

func Test_Create_concatenates_incremental_writes(t *testing.T) {
	root, dir := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)

	_, firstErr := io.WriteString(w, "first ")
	require.NoError(t, firstErr)
	_, secondErr := io.WriteString(w, "second")
	require.NoError(t, secondErr)
	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "first second", string(got))
}

func Test_Create_interleaves_WriteString_and_Write(t *testing.T) {
	root, dir := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)

	_, stringErr := w.WriteString("from a string, ")
	require.NoError(t, stringErr)
	_, bytesErr := w.Write([]byte("from bytes, "))
	require.NoError(t, bytesErr)
	_, lastErr := w.WriteString("and a string again")
	require.NoError(t, lastErr)
	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "from a string, from bytes, and a string again", string(got))
}

func Test_Create_replaces_an_existing_files_content_on_close(t *testing.T) {
	root, dir := openRoot(t)
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

	onDisk, beforeErr := os.ReadFile(target)
	require.NoError(t, beforeErr)
	assert.Equal(t, "old", string(onDisk), "the old content must survive until Close")

	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "new", string(got))
}

func Test_Create_keeps_the_existing_files_permissions_when_it_replaces_it(t *testing.T) {
	root, dir := openRoot(t)
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o400))

	w, err := atomicfile.Create(root, "target.txt", 0o600)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	info, statErr := os.Stat(target)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o400), info.Mode().Perm())
}

// Unlike 0o400 above, 0o666 carries bits a typical umask would strip, so
// this discriminates Chmod from merely handing the mode to OpenFile.
func Test_Create_preserves_a_mode_the_process_umask_would_otherwise_narrow(t *testing.T) {
	root, dir := openRoot(t)
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o666)) //nolint:gosec // the wide mode is the fixture under test, not a real file
	require.NoError(t, os.Chmod(target, 0o666), "WriteFile's own mode argument is umask-masked too")

	w, err := atomicfile.Create(root, "target.txt", 0o600)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	info, statErr := os.Stat(target)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o666), info.Mode().Perm(),
		"the target's own mode must survive the replace regardless of umask")
}

// name does not exist yet, so replaceMode falls through to perm and
// OpenFile's mode argument — not Chmod — must produce the result.
func Test_Create_honours_the_process_umask_on_a_fresh_create(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	root, dir := openRoot(t)

	w, err := atomicfile.Create(root, "fresh.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	info, statErr := os.Stat(filepath.Join(dir, "fresh.txt"))
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"a fresh create must let the umask narrow perm, not apply perm exactly via Chmod")
}

func Test_Create_abandoned_without_close_leaves_the_target_untouched(t *testing.T) {
	root, dir := openRoot(t)
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "never committed")
	require.NoError(t, writeErr)

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(got))

	assert.FileExists(t, filepath.Join(dir, ".target.txt.brief-tmp"),
		"the abandoned write's temp sibling must still be on disk, not silently dropped")
}

// Pins the umask at 0o022: the temp-sibling assertion sits on the
// existed==false path, where the mode is perm masked by the umask.
func Test_Create_reports_the_failure_and_keeps_the_target_when_the_rename_cannot_land(t *testing.T) {
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	root, dir := openRoot(t)
	targetDir := filepath.Join(dir, "target.txt")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	sentinel := filepath.Join(targetDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep me"), 0o600))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

	// Discriminates replaceMode's IsRegular gate: target.txt is a
	// directory, so the sibling must still get perm's mode, not the directory's.
	tmpInfo, tmpErr := os.Lstat(filepath.Join(dir, ".target.txt.brief-tmp"))
	require.NoError(t, tmpErr)
	assert.Equal(t, os.FileMode(0o644), tmpInfo.Mode().Perm(),
		"a directory target must not hand its own mode to the temp sibling")

	require.Error(t, w.Close())
	assert.FileExists(t, sentinel)

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.ElementsMatch(t, []string{"target.txt"}, names, "the temp sibling must not be left behind")
}

// Control: proves the directory-listing probe above would in fact have
// seen a leftover temp file.
func Test_Create_removes_the_temp_sibling_once_close_has_committed(t *testing.T) {
	root, dir := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

	assert.FileExists(t, filepath.Join(dir, ".target.txt.brief-tmp"), "the temp sibling must exist before Close")

	require.NoError(t, w.Close())

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.ElementsMatch(t, []string{"target.txt"}, names)
}

// Pins the umask: target.txt does not exist, so the committed mode is
// perm masked by the umask, not the stale sibling's own mode.
func Test_Create_overwrites_a_stale_temp_file_left_by_a_crashed_write(t *testing.T) {
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	root, dir := openRoot(t)
	stale := filepath.Join(dir, ".target.txt.brief-tmp")
	require.NoError(t, os.WriteFile(stale, []byte("garbage left by a crashed write"), 0o600))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "new", string(got))

	info, statErr := os.Stat(filepath.Join(dir, "target.txt"))
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(),
		"the stale sibling's own mode must not survive onto the target it never named")
}

// The stale sibling is left at 0o666, coinciding with neither perm nor
// perm masked by the umask, so a missing removal is caught either way.
func Test_Create_removes_a_stale_temp_sibling_so_a_fresh_create_still_honours_the_umask(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	root, dir := openRoot(t)
	stale := filepath.Join(dir, ".target.txt.brief-tmp")
	require.NoError(t, os.WriteFile(stale, []byte("garbage left by a crashed write"), 0o666)) //nolint:gosec // the wide mode is the fixture under test, not a real file
	require.NoError(t, os.Chmod(stale, 0o666), "WriteFile's own mode argument is umask-masked too")

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	info, statErr := os.Stat(filepath.Join(dir, "target.txt"))
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"a stale sibling must not exempt a fresh create from the umask")
}

func Test_Create_a_second_close_neither_errors_nor_undoes_the_commit(t *testing.T) {
	root, dir := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "committed")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	assert.NoError(t, w.Close())

	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "committed", string(got))
}

func Test_Create_a_second_close_replays_the_first_failure(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "target.txt"), 0o755))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

	first := w.Close()
	require.Error(t, first)

	second := w.Close()

	require.Error(t, second)
	assert.Equal(t, first.Error(), second.Error(),
		"a second Close must replay the first failure, not report nil")
}

// Pins that every Close path closes the descriptor before returning, not
// merely that Write refuses after Close.
func Test_Create_refuses_a_write_after_close(t *testing.T) {
	root, _ := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, writeErr := w.Write([]byte("too late"))

	require.Error(t, writeErr)
	assert.ErrorIs(t, writeErr, os.ErrClosed)
}

// The temp sibling is removed out from under Close before it runs, so the
// cleanup that follows the failed rename finds nothing to remove.
func Test_Create_does_not_report_a_missing_sibling_it_never_left_behind(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "target.txt"), 0o755))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

	require.NoError(t, os.Remove(filepath.Join(dir, ".target.txt.brief-tmp")))

	closeErr := w.Close()

	require.Error(t, closeErr)
	assert.NotContains(t, closeErr.Error(), "remove temp file for",
		"there was no sibling left behind for Close to fail to remove")
}

func Test_Create_fails_when_the_temp_sibling_cannot_be_opened(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".target.txt.brief-tmp"), 0o755))

	w, err := atomicfile.Create(root, "target.txt", 0o644)

	require.Error(t, err)
	assert.Nil(t, w)
}

// The target (0o640) and a stale sibling (0o666) both already exist, so
// only Chmod can still produce 0o640 rather than 0o666 or perm's 0o600.
func Test_Create_reapplies_an_existing_targets_mode_onto_a_reused_stale_sibling(t *testing.T) {
	oldMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	root, dir := openRoot(t)

	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o640)) //nolint:gosec // the group-readable mode is the fixture under test
	require.NoError(t, os.Chmod(target, 0o640), "WriteFile's own mode argument is umask-masked too")

	stale := filepath.Join(dir, ".target.txt.brief-tmp")
	require.NoError(t, os.WriteFile(stale, []byte("garbage left by a crashed write"), 0o666)) //nolint:gosec // the wide mode is the fixture under test, not a real file
	require.NoError(t, os.Chmod(stale, 0o666), "WriteFile's own mode argument is umask-masked too")

	staleIno := inodeOf(t, stale)

	w, err := atomicfile.Create(root, "target.txt", 0o600)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "new", string(got), "the replacement must still have committed")

	info, statErr := os.Stat(target)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(),
		"a reused stale sibling must not carry its own mode onto the target, nor let perm override the target's")

	// Proves OpenFile reopened the stale file rather than recreating it.
	assert.Equal(t, staleIno, inodeOf(t, target),
		"the committed file must be the stale sibling's own inode, reused rather than recreated")
}

// inodeOf returns path's inode number, so a test can prove a file was
// reused in place rather than deleted and recreated at the same name.
func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()

	info, err := os.Lstat(path)
	require.NoError(t, err)

	st, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok, "expected a *syscall.Stat_t from Lstat on this platform")

	return st.Ino
}
