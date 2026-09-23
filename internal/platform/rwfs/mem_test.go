package rwfs_test

import (
	"fmt"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_NewMem_seeds_a_literal_fixture proves the constructor's whole point:
// a test can hand it a literal fstest.MapFS and read it straight back
// through the FS interface, with no Mkdir/WriteFile calls of its own.
func Test_NewMem_seeds_a_literal_fixture(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"a.txt": {Data: []byte("hello"), Mode: 0o600},
	})

	got, err := m.ReadFile("a.txt")

	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

// Test_Mem_Snapshot_reflects_writes_made_through_the_FS_interface proves
// Snapshot is reading m's live state, not the fixture NewMem started from.
func Test_Mem_Snapshot_reflects_writes_made_through_the_FS_interface(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))

	snap := m.Snapshot()

	require.Contains(t, snap, "a.txt")
	assert.Equal(t, "hello", string(snap["a.txt"].Data))
}

// Test_Mem_Snapshot_returns_a_copy_safe_to_mutate is the control for the
// "copy" half of Snapshot's contract: mutating the returned map, and the
// MapFile it points to, must not be visible on a second Snapshot call.
func Test_Mem_Snapshot_returns_a_copy_safe_to_mutate(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))

	first := m.Snapshot()
	first["a.txt"].Data[0] = 'H'
	first["b.txt"] = &fstest.MapFile{Data: []byte("injected")}

	second := m.Snapshot()
	assert.Equal(t, "hello", string(second["a.txt"].Data), "mutating a snapshot's bytes must not reach m")
	assert.NotContains(t, second, "b.txt", "adding a key to a snapshot must not reach m")
}

// Test_Mem_implicit_ancestor_directories_survive_removing_their_last_child
// pins the divergence rwfs' contract cannot exercise: testing/fstest.MapFS
// synthesizes a directory like "d" purely from the presence of "d/f", so a
// seed built that way has no explicit entry for "d" itself. A real
// filesystem's directory persists once created regardless of its children;
// NewMem must materialize that explicit entry at construction so Remove
// of the last child does not make the directory disappear too.
func Test_Mem_implicit_ancestor_directories_survive_removing_their_last_child(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"d/f.txt": {Data: []byte("x"), Mode: 0o600},
	})

	require.NoError(t, m.Remove("d/f.txt"))

	entries, err := m.ReadDir("d")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Test_Mem_WriteFile_clones_the_caller_s_buffer proves WriteFile stores its
// own copy of data: mutating the caller's slice after WriteFile returns must
// not change what a later ReadFile sees.
func Test_Mem_WriteFile_clones_the_caller_s_buffer(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	data := []byte("hello")

	require.NoError(t, m.WriteFile("a.txt", data, 0o600))
	data[0] = 'H'

	got, err := m.ReadFile("a.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got), "mutating the caller's buffer after WriteFile must not reach stored content")
}

// Test_Mem_WriteFile_of_identical_bytes_still_advances_ModTime pins Mem's
// fake clock: WriteFile always advances the entry's ModTime, even when the
// new bytes equal the old ones, so a caller diffing two Snapshots can tell
// "rewrote the same content" from "never wrote at all" — a distinction
// content-equality alone can't make.
func Test_Mem_WriteFile_of_identical_bytes_still_advances_ModTime(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))
	before := m.Snapshot()["a.txt"].ModTime

	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))

	after := m.Snapshot()["a.txt"].ModTime
	assert.NotEqual(t, before, after)
}

// Test_Mem_ModTime_is_unchanged_by_a_read pins the other half of the fake
// clock's contract: a call that writes nothing must not advance any entry's
// ModTime, else Snapshot could not distinguish "no write happened" either.
func Test_Mem_ModTime_is_unchanged_by_a_read(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))
	before := m.Snapshot()["a.txt"].ModTime

	_, err := m.ReadFile("a.txt")
	require.NoError(t, err)

	after := m.Snapshot()["a.txt"].ModTime
	assert.Equal(t, before, after)
}

// Test_Mem_OpenRoot_refuses_a_symlink_target_it_cannot_resolve pins
// resolveDir's two refusal branches with no OS counterpart: os.Root reports
// an absolute symlink target, or one that resolves above the root, with its
// own error shapes (see os_test.go's escape test), so these are Mem-only.
func Test_Mem_OpenRoot_refuses_a_symlink_target_it_cannot_resolve(t *testing.T) {
	cases := []struct {
		name   string
		symlnk string
		target string
	}{
		{name: "target is absolute", symlnk: "abs", target: "/nowhere"},
		{name: "target resolves above the root", symlnk: "up", target: "../x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := rwfs.NewMem(fstest.MapFS{
				c.symlnk: {Data: []byte(c.target), Mode: fs.ModeSymlink | 0o777},
			})

			_, err := m.OpenRoot(c.symlnk)

			require.ErrorIs(t, err, fs.ErrInvalid)
		})
	}
}

// Test_Mem_OpenRoot_fails_on_a_symlink_cycle pins resolveDir's hop cap: a
// two-entry symlink cycle never reaches a real directory, so OpenRoot must
// fail rather than loop forever.
func Test_Mem_OpenRoot_fails_on_a_symlink_cycle(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"a": {Data: []byte("b"), Mode: fs.ModeSymlink | 0o777},
		"b": {Data: []byte("a"), Mode: fs.ModeSymlink | 0o777},
	})

	_, err := m.OpenRoot("a")

	require.ErrorIs(t, err, fs.ErrInvalid)
}

// Test_Mem_is_safe_for_concurrent_use runs WriteFile from many goroutines
// against distinct names, guarding the shared map with m's own mutex. Each
// goroutine writes a name no other goroutine touches, so this proves the
// mutex protects concurrent map access rather than proving anything about
// two writers racing for the same name — atomicfile's own temp-sibling name
// is deterministic, so same-name concurrent WriteFile is not a contract
// either adapter offers. Run with -race; the mutation this guards against
// (dropping Mem's mutex) is only detectable under -race, not by this test's
// own assertions.
func Test_Mem_is_safe_for_concurrent_use(t *testing.T) {
	const n = 50
	m := rwfs.NewMem(fstest.MapFS{})

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("file-%02d.txt", i)
			assert.NoError(t, m.WriteFile(name, []byte(name), 0o600))
		}(i)
	}
	wg.Wait()

	snap := m.Snapshot()
	for i := range n {
		name := fmt.Sprintf("file-%02d.txt", i)
		require.Contains(t, snap, name)
		assert.Equal(t, name, string(snap[name].Data))
	}
}
