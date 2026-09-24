package assemble

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/rwfs/rwfstest"
	"github.com/stretchr/testify/require"
)

// dirOpenerAdapter adapts dirFS (fs.go) to rwfstest.DirOpener: the two
// interfaces share an identical method set, but Go does not consider two
// distinct named interface types interchangeable in a method's return
// position, so osRoot and memDirFS — both dirFS, never rwfstest.DirOpener —
// need this translation at every OpenRoot hop.
type dirOpenerAdapter struct{ d dirFS }

func (a dirOpenerAdapter) FS() fs.FS { return a.d.FS() }

func (a dirOpenerAdapter) Lstat(name string) (fs.FileInfo, error) { return a.d.Lstat(name) }

func (a dirOpenerAdapter) OpenRoot(name string) (rwfstest.DirOpener, error) {
	sub, err := a.d.OpenRoot(name)
	if err != nil {
		return nil, err
	}

	return dirOpenerAdapter{d: sub}, nil
}

// seedFixtureTree lists the entries every read_contract_internal_test.go
// fixture seeds, relative to the directory openFeatureDir opens: a
// top-level file (rwfstest.ReadContract's own "a.txt"), a "list" directory
// holding two files in non-sorted creation order (its own sorted-listing
// row), and a "sub" directory holding one file (ReadOpenRootContract's own
// descend row).
var seedFixtureTree = map[string]string{
	"a.txt":         "hello",
	"list/b.txt":    "x",
	"list/a.txt":    "x",
	"sub/a.txt":     "nested",
	"sub/a.txt.bak": "unused", // ensures "sub" is not pruned to a single-entry edge case
}

// newOSRootFixture builds osRoot the same way Start, Check and Status
// build it in production: through (*Server).openFeatureDir with no WithFS
// option, over a real temp directory tree written with plain os.*.
func newOSRootFixture(t *testing.T) rwfstest.DirOpener {
	t.Helper()

	dir := t.TempDir()
	for name, body := range seedFixtureTree {
		full := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}

	s := NewServer(config.Default(), dir)
	d, err := s.openFeatureDir(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	return dirOpenerAdapter{d: d}
}

// memFixtureDir is the multi-segment name openFeatureDir's own fsName
// mapping produces from an absolute OS directory — the exact shape
// rwfs/doc.go's own ancestor-symlink divergence note is about, and the one
// every WithFS-backed cli command test already exercises via
// resolveRoot/openFeatureDir (assemble/fs.go, scaffold/fs.go).
const memFixtureDir = "repo/docs/specifications"

// newMemDirFixture builds memDirFS the same way Start, Check and Status
// build it under WithFS: through (*Server).openFeatureDir over an
// rwfs.Mem, opened at memFixtureDir — proving the exact call site rwfs'
// own doc.go describes, not a hand-built memDirFS bypassing it.
func newMemDirFixture(t *testing.T) rwfstest.DirOpener {
	t.Helper()

	seed := fstest.MapFS{}
	for name, body := range seedFixtureTree {
		seed[memFixtureDir+"/"+name] = &fstest.MapFile{Data: []byte(body), Mode: 0o600}
	}
	mem := rwfs.NewMem(seed)

	s := NewServer(config.Default(), "/"+memFixtureDir, WithFS(mem))
	d, err := s.openFeatureDir("/" + memFixtureDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	return dirOpenerAdapter{d: d}
}

// Test_osRoot_and_memDirFS_share_the_read_contract proves assemble's own
// hand-rolled production adapter (osRoot) and its Mem-backed test twin
// (memDirFS) satisfy one shared read contract, the same way rwfs.OS and
// rwfs.Mem share rwfstest.Contract — both built through the real
// (*Server).openFeatureDir entry point Start, Check and Status all use, so
// this also exercises the exact multi-segment name openFeatureDir's own
// fsName mapping produces from an absolute OS directory.
func Test_osRoot_and_memDirFS_share_the_read_contract(t *testing.T) {
	t.Run("osRoot", func(t *testing.T) {
		rwfstest.ReadContract(t, func(t *testing.T) fs.FS {
			t.Helper()

			return newOSRootFixture(t).FS()
		})
		rwfstest.ReadOpenRootContract(t, newOSRootFixture,
			rwfstest.OpenRootNotDirUnclassified(
				"osRoot.OpenRoot (fs.go) calls (*os.Root).OpenRoot directly, unlike rwfs.OS's own OpenRoot, which "+
					"runs the raw failure through classifyOpenRootErr first; confirmed on go1.27.1/darwin that the "+
					"raw call reports a bare, unwrapped \"not a directory\" string rather than syscall.ENOTDIR when "+
					"name exists as a file. Reachable in production through Start (topRoot.OpenRoot(feature) with "+
					"no prior Lstat, assemble.go); Check and Status both Lstat and check IsDir() first, so neither "+
					"ever reaches this branch (check.go, status.go). Classifying it would change Start's own "+
					"wrapped error text, so this stays a documented exception rather than a fix",
			),
		)
	})

	t.Run("memDirFS", func(t *testing.T) {
		rwfstest.ReadContract(t, func(t *testing.T) fs.FS {
			t.Helper()

			return newMemDirFixture(t).FS()
		})
		rwfstest.ReadOpenRootContract(t, newMemDirFixture)
	})
}
