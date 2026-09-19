package atomicfile_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// Test_Create_concatenates_incremental_writes covers the reason an
// io.WriteCloser exists rather than only WriteFile: the caller streams
// without holding the whole payload.
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
}

func Test_Create_reports_the_failure_and_keeps_the_target_when_the_rename_cannot_land(t *testing.T) {
	root, dir := openRoot(t)
	targetDir := filepath.Join(dir, "target.txt")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	sentinel := filepath.Join(targetDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep me"), 0o600))

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	_, writeErr := io.WriteString(w, "new")
	require.NoError(t, writeErr)

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

func Test_Create_overwrites_a_stale_temp_file_left_by_a_crashed_write(t *testing.T) {
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

// Test_Create_refuses_a_write_after_close pins the observable, not a guard
// in this package: every Close path closes the descriptor first, so the
// refusal comes from the descriptor itself. An explicit p.closed check in
// Write was removed after a mutation showed no test could tell it apart from
// this behaviour.
func Test_Create_refuses_a_write_after_close(t *testing.T) {
	root, _ := openRoot(t)

	w, err := atomicfile.Create(root, "target.txt", 0o644)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, writeErr := w.Write([]byte("too late"))

	require.Error(t, writeErr)
	assert.ErrorIs(t, writeErr, os.ErrClosed)
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
