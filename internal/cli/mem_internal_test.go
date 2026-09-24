// Shared rwfs.Mem test plumbing for every command-level test that seams
// setup through newMemSetupSeam — init_internal_test.go and
// uninstall_internal_test.go both use it. White-box package: newMemSetupSeam
// builds a runSeam via the unexported withSetupOpts.

package cli

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
)

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
