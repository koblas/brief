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

func Test_NewMem_seeds_a_literal_fixture(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"a.txt": {Data: []byte("hello"), Mode: 0o600},
	})

	got, err := m.ReadFile("a.txt")

	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func Test_Mem_Snapshot_reflects_writes_made_through_the_FS_interface(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))

	snap := m.Snapshot()

	require.Contains(t, snap, "a.txt")
	assert.Equal(t, "hello", string(snap["a.txt"].Data))
}

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

// testing/fstest.MapFS synthesizes "d" purely from the presence of "d/f",
// so this seed has no explicit entry for "d" itself.
func Test_Mem_implicit_ancestor_directories_survive_removing_their_last_child(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"d/f.txt": {Data: []byte("x"), Mode: 0o600},
	})

	require.NoError(t, m.Remove("d/f.txt"))

	entries, err := m.ReadDir("d")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func Test_Mem_WriteFile_clones_the_caller_s_buffer(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	data := []byte("hello")

	require.NoError(t, m.WriteFile("a.txt", data, 0o600))
	data[0] = 'H'

	got, err := m.ReadFile("a.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got), "mutating the caller's buffer after WriteFile must not reach stored content")
}

func Test_Mem_WriteFile_of_identical_bytes_still_advances_ModTime(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))
	before := m.Snapshot()["a.txt"].ModTime

	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))

	after := m.Snapshot()["a.txt"].ModTime
	assert.NotEqual(t, before, after)
}

func Test_Mem_ModTime_is_unchanged_by_a_read(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{})
	require.NoError(t, m.WriteFile("a.txt", []byte("hello"), 0o600))
	before := m.Snapshot()["a.txt"].ModTime

	_, err := m.ReadFile("a.txt")
	require.NoError(t, err)

	after := m.Snapshot()["a.txt"].ModTime
	assert.Equal(t, before, after)
}

// These two refusal branches have no OS counterpart: os.Root reports an
// absolute or escaping symlink target with its own error shapes.
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

func Test_Mem_OpenRoot_fails_on_a_symlink_cycle(t *testing.T) {
	m := rwfs.NewMem(fstest.MapFS{
		"a": {Data: []byte("b"), Mode: fs.ModeSymlink | 0o777},
		"b": {Data: []byte("a"), Mode: fs.ModeSymlink | 0o777},
	})

	_, err := m.OpenRoot("a")

	require.ErrorIs(t, err, fs.ErrInvalid)
}

// Each goroutine writes a distinct name; this proves concurrent map access
// is safe, not same-name write ordering. Run with -race.
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
