// Shared rwfs.Mem test plumbing for every command-level test that seams
// setup through newMemSetupSeam — init_internal_test.go and
// uninstall_internal_test.go both use it — or that builds a bare feature
// tree through memTree for new/finish/start/check/status's own
// withRootFS seam. White-box package: newMemSetupSeam builds a runSeam via
// the unexported withSetupOpts.

package cli

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
)

// memRoot is the virtual working directory every new/finish/start/check/
// status Mem test resolves against — fabricated, never a real disk path,
// the same convention doctor_internal_test.go's own "/repo" fixtures use.
const memRoot = "/repo"

// fsAbs joins slash-separated segments under "/", the way every
// newMemSetupSeam-backed test names an absolute path its rwfs.Mem fixture
// is keyed against. Identical to internal/setup's, config's, repo's and
// doctor's own fsAbs test helper, duplicated for the same reason those
// packages duplicate fsName from each other.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// memKey turns abs — an already-absolute, fsAbs-fabricated path — into the
// name a Mem's own fstest.MapFS is keyed against: fsName's own inverse
// (internal/setup's fs.go, package-private, so this package cannot call it
// directly), duplicated for the same reason internal/setup's own mem_internal_test.go
// duplicates it from config/repo/doctor.
func memKey(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// newVirtualMem returns an rwfs.Mem seeded with root as an explicit
// directory entry — every fsys read Init or Uninstall performs starts by
// confirming wd itself exists (config.LocateWithinFS's own first check),
// so any fixture needs at least this much regardless of what else it
// seeds. root is virtual, fabricated as an absolute path — never a real
// disk path — since newMemSetupSeam's own writableCheck never reaches
// real disk to ask.
func newVirtualMem(root string) *rwfs.Mem {
	return rwfs.NewMem(fstest.MapFS{memKey(root): &fstest.MapFile{Mode: fs.ModeDir | 0o755}})
}

// newMemSetupSeam returns the runSeam a command-level init/uninstall test
// runs mem's own fixture through instead of real disk: setup.WithFSRoot and
// setup.WithResolveRoot(identity) — mem names no real directory for
// filepath.EvalSymlinks to resolve — plus setup.WithWritableCheck's own
// no-op, so R10's pre-write check never asks real disk about a target that
// only exists on mem, mirroring internal/setup's own newMemServer recipe
// (mem_internal_test.go). setup.WithHomeDir defaults to a fixed empty string, the
// same as emptyHomeSeam, so a bare-name role binding never resolves against
// the developer's own real "~/.claude/agents"; extra opts are appended
// last, so a test can still override any of these without repeating the
// others.
func newMemSetupSeam(mem *rwfs.Mem, extra ...setup.Option) runSeam {
	opts := append([]setup.Option{
		setup.WithFSRoot(mem),
		setup.WithResolveRoot(func(root string) (string, error) { return root, nil }),
		setup.WithHomeDir(func() (string, error) { return "", nil }),
		setup.WithWritableCheck(func([]string) error { return nil }),
	}, extra...)

	return withSetupOpts(opts...)
}

// memTree accumulates directory and file entries for an rwfs.Mem fixture,
// keyed by absolute, fsAbs-fabricated path — a small builder so a
// new/finish/start/check/status Mem test can lay out a feature tree the
// same declarative way its disk-based sibling built one with
// os.MkdirAll/os.WriteFile, without repeating fstest.MapFS's own map
// literal shape at every call site. dir and file both return t so calls
// chain; mem builds the rwfs.Mem once every entry is added.
type memTree struct {
	entries fstest.MapFS
}

// newMemTree returns a memTree seeded with every path in dirs as an
// explicit directory entry — root itself always belongs in that list, the
// same "confirm wd exists" requirement newVirtualMem documents, since a
// zero-file fixture would otherwise have no entry for config.LocateWithinFS's
// own first check to find.
func newMemTree(dirs ...string) *memTree {
	t := &memTree{entries: fstest.MapFS{}}

	for _, d := range dirs {
		t.dir(d)
	}

	return t
}

// dir adds path as an explicit directory entry.
func (t *memTree) dir(path string) *memTree {
	t.entries[memKey(path)] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}

	return t
}

// file adds path as a regular file entry holding body.
func (t *memTree) file(path, body string) *memTree {
	t.entries[memKey(path)] = &fstest.MapFile{Data: []byte(body), Mode: 0o600}

	return t
}

// mem builds the rwfs.Mem every entry added so far backs.
func (t *memTree) mem() *rwfs.Mem {
	return rwfs.NewMem(t.entries)
}
