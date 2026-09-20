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

// Test_Create_makes_the_content_visible_only_when_close_commits_it is the
// whole point of the type: the bytes sit in the temp sibling, and the
// target does not exist at all, until Close renames.
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

// Test_Create_concatenates_incremental_writes covers writing a payload in
// pieces: successive writes append to the temp sibling and the commit sees
// all of them.
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

// Test_Create_interleaves_WriteString_and_Write covers io.StringWriter
// alongside io.Writer: both must append to the same temp sibling, in call
// order, so a caller holding a mix of strings and byte slices does not have
// to convert either.
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

// Test_Create_preserves_a_mode_the_process_umask_would_otherwise_narrow is
// the discriminating arm the test above cannot be: 0o400 has no group or
// other write bit for a typical 022 umask to strip, so it stays correct even
// if the mode were merely handed to OpenFile's masked mode argument instead
// of applied with Chmod. 0o666 does have those bits, so replacing a target
// at exactly 0o666 must still come back 0o666 — not silently narrowed to
// whatever the umask allows a freshly created file to have.
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

// Test_Create_honours_the_process_umask_on_a_fresh_create is the
// discriminating arm the replace-path tests above cannot be: name does not
// exist yet, so replaceMode falls through to perm, and OpenFile's own mode
// argument — not Chmod — must produce the result. perm is 0o644, a mode
// with the group- and other-write bits a 0o077 umask strips; a build that
// still Chmods perm exactly regardless of whether name existed defeats the
// umask and lands 0o644 instead of 0o600.
//
// syscall.Umask is process-global, so this test sets it and restores the
// prior value with t.Cleanup. That makes the test order-sensitive with
// respect to any other test in this package that also touches the umask —
// acceptable here because no test in this package (or file) calls
// t.Parallel, so nothing else observes the mutated umask mid-test.
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

// Test_Create_abandoned_without_close_leaves_the_target_untouched pins the
// safe direction of the failure: dropping the writer on the floor loses the
// new content and keeps the old file, rather than committing a partial one.
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

// Test_Create_reports_the_failure_and_keeps_the_target_when_the_rename_cannot_land
// pins the umask at 0o022 because its temp-sibling assertion sits on the
// existed==false path, where the observed mode is perm masked by the umask
// rather than perm exactly — Chmod does not run on a fresh create. Without
// the pin the assertion false-reds under any umask stricter than 0o022,
// which says nothing about the IsRegular gate it is there to discriminate.
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

	// replaceMode's IsRegular gate is what discriminates here: target.txt is
	// a directory, at 0o755, so the temp sibling must still get perm's mode
	// rather than the directory's — without the gate, the rename below would
	// still fail (a file can never rename over a directory either way) and
	// this test would stay green with the gate deleted.
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

// Test_Create_removes_the_temp_sibling_once_close_has_committed is the
// control for the assertion above: it proves the directory-listing probe
// would in fact have seen a leftover temp file.
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

// Test_Create_overwrites_a_stale_temp_file_left_by_a_crashed_write pins the
// umask for the same reason as the rename test above: target.txt does not
// exist, so this is an existed==false path and the committed mode is perm
// masked by the umask. The pin keeps the ambient umask out of an assertion
// about the stale sibling's mode.
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

// Test_Create_removes_a_stale_temp_sibling_so_a_fresh_create_still_honours_the_umask
// is the test above's discriminating twin: the stale sibling here is left
// at 0o666, a mode neither perm (0o644) nor perm masked by a 0o077 umask
// (0o600) coincides with, so a mutation that stops removing the stale
// sibling (leaving its 0o666 to survive untouched, since a fresh create
// never Chmods) is distinguishable from the intended result on either
// axis — unlike a stale mode of 0o600, which would equal the umask-masked
// answer by coincidence and pass even with the removal deleted.
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

// Test_Create_a_second_close_neither_errors_nor_undoes_the_commit lets the
// idiomatic "defer Close" safety net sit alongside an explicit Close whose
// error is checked, without the deferred call reporting a spurious failure
// or disturbing the committed file.
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

// Test_Create_a_second_close_replays_the_first_failure covers the other
// half of the second-Close contract: when the first Close failed, a second
// one must report the same failure rather than reporting nil just because
// nothing is retried.
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

// Test_Create_refuses_a_write_after_close pins that every Close path closes
// the underlying descriptor before it returns, not merely that Write refuses
// after Close: mutate Close to skip p.file.Close() before the rename and
// this test reddens, because the descriptor stays open and the write it
// probes no longer fails at all.
func Test_Create_refuses_a_write_after_close(t *testing.T) {
	root, _ := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, writeErr := w.Write([]byte("too late"))

	require.Error(t, writeErr)
	assert.ErrorIs(t, writeErr, os.ErrClosed)
}

// Test_Create_does_not_report_a_missing_sibling_it_never_left_behind covers
// removeTmp's ErrNotExist passthrough: the temp sibling is removed out from
// under Close by something other than this package before Close runs, so
// the rename fails (the target is a directory) and the cleanup that follows
// finds nothing to remove. The reported error must still name the rename
// failure without also claiming a removal failed that never happened.
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

// Test_Create_fails_when_the_temp_sibling_cannot_be_opened reaches the only
// error Create itself can report, using the same planted-directory seam the
// Finish crash-convergence tests use.
func Test_Create_fails_when_the_temp_sibling_cannot_be_opened(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".target.txt.brief-tmp"), 0o755))

	w, err := atomicfile.Create(root, "target.txt", 0o644)

	require.Error(t, err)
	assert.Nil(t, w)
}

// Test_Create_reapplies_an_existing_targets_mode_onto_a_reused_stale_sibling
// covers the one case Chmod is the sole guard for, and the only one where
// it is load-bearing regardless of the umask.
//
// When the target exists, replaceMode takes its bits, but the temp sibling
// also already exists — left by a crashed write — so OpenFile reuses that
// inode and ignores its mode argument outright. Neither the umask nor perm
// enters into it: without Chmod the committed file simply keeps the stale
// sibling's own 0o666, and the target's 0o640 is silently widened.
//
// Deleting the Chmod call leaves the rest of this package green at umask
// 0o000 — the existing coverage discriminates it only through the umask, on
// the fresh-sibling path — so this is the fixture that makes the guard
// individually falsifiable.
//
// The three modes are deliberately distinct: 0o640 for the target, 0o666
// for the stale sibling, 0o600 for perm. A committed 0o666 means the stale
// mode survived, a 0o600 means perm won over replaceMode, and only 0o640 is
// the intended result. os.WriteFile's own mode argument is umask-masked
// too, so both fixture files are Chmod'd after writing — otherwise this
// test reproduces exactly the defect it was written to catch.
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

	// Without this, the test's name outruns what it pins: lifting the
	// !existed gate on removeStaleRegularSibling makes the sibling be
	// deleted and recreated rather than reused, and every assertion above
	// still passes — Chmod would then be discriminated only through the
	// umask again, which is exactly the coverage this test exists to
	// replace. The inode is what proves OpenFile reopened the stale file.
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
