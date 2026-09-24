package setup_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
)

// findArtifact returns res's own first Artifact of kind, failing the test
// if there is none — the by-kind lookup a Mem-backed test uses in place of
// an index into Result.Artifacts, so it never depends on that list's own
// row order except in the one test that deliberately pins it.
func findArtifact(t *testing.T, res setup.Result, kind setup.Kind) setup.Artifact {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Kind == kind {
			return a
		}
	}

	t.Fatalf("no %s row in %v", kind, res.Artifacts)

	return setup.Artifact{}
}

// findArtifactByPath returns res's own Artifact at path, failing the test
// if there is none — the by-path lookup a Mem-backed test uses when
// findArtifact's by-Kind lookup is ambiguous (several rows share one Kind,
// the plugin's own manifest, start and finish skills all KindPlugin).
func findArtifactByPath(t *testing.T, res setup.Result, path string) setup.Artifact {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Path == path {
			return a
		}
	}

	t.Fatalf("no row for %s in %v", path, res.Artifacts)

	return setup.Artifact{}
}

// fsAbs joins slash-separated segments under "/", the way every
// WithFSRoot-backed test names an absolute path its fstest.MapFS fixture
// is keyed against — setup's own fsName (fs.go) strips the leading "/"
// internally to get the fs.FS-relative name back. Identical to config's,
// repo's and doctor's own fsAbs test helper, duplicated for the same
// reason those packages duplicate fsName from each other.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// identityResolveRoot is a setup.Option's own resolveRoot stand-in for
// filepath.EvalSymlinks: mem (below) names no real directory on disk, so
// resolving root's own symlinks for real would always fail. It returns
// root unchanged, matching a root with no symlinks in it at all — every
// Mem-backed fixture in this package builds root as a plain string, never
// a symlink.
func identityResolveRoot(root string) (string, error) { return root, nil }

// memKey turns abs — an already-absolute path, real or fsAbs-fabricated —
// into the name a Mem's own fstest.MapFS is keyed against: fsName's own
// inverse (fs.go, package-private, so this test package cannot call it
// directly), duplicated for the same reason fsAbs above is.
func memKey(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// newVirtualMem returns a Mem seeded with root as an explicit directory
// entry — every fsys read Init or Uninstall performs starts by confirming
// wd itself exists (config.LocateWithinFS's own first check), so any
// fixture needs at least this much regardless of what else it seeds. root
// is virtual, fabricated with fsAbs — never a real disk path — since
// newMemServer's own default writableCheck (WithWritableCheck) never
// reaches real disk to ask.
func newVirtualMem(root string) *rwfs.Mem {
	return rwfs.NewMem(fstest.MapFS{memKey(root): &fstest.MapFile{Mode: fs.ModeDir | 0o755}})
}

// emptyHomeDir is a setup.Option pinning WithHomeDir to a function that
// always reports "", nil — newMemServer's own default, mirroring doctor's
// own emptyHomeDir: a Mem-backed fixture holds no real "~/.claude/agents"
// for agentfile.ResolveBinding to search, so without this every
// HostClaudeCode Mem test would silently depend on whatever role agents
// happen to exist under the developer's own real home directory.
func emptyHomeDir() setup.Option {
	return setup.WithHomeDir(func() (string, error) { return "", nil })
}

// writableRecorder stands in for R10's own real-disk writableCheck
// (checkWritable, writable.go, pinned directly by writable_disk_test.go):
// every call reports nil rather than walking real disk, so a Mem-backed
// apply always proceeds regardless of what real disk at wd would have
// reported, and calls records exactly which target lists a real run would
// have checked, in call order — writableTargets' own planning logic,
// assertable without disk. It never re-implements, approximates or skips
// around checkWritable's own decision; it simply is not asked to make one.
type writableRecorder struct {
	calls [][]string
}

func (r *writableRecorder) check(targets []string) error {
	r.calls = append(r.calls, append([]string{}, targets...))

	return nil
}

// newMemServer builds a Server whose every read and write under a
// repository root, and whose boundAgentTargets/agentsMissingSkill own
// root-resolution, run against mem rather than real disk (WithFSRoot,
// WithResolveRoot), homeDir defaulted to emptyHomeDir, and writableCheck
// defaulted to a no-op recorder (WithWritableCheck) — opts are appended
// last, so a test can still override any of these without repeating the
// others. No Mem fixture in this package may bind a bare-name planner or
// implementer role: agentfile.ResolveBinding and planBoundAgent always
// search and read real disk regardless of fsRoot, so such a binding would
// silently resolve against nothing rather than the fixture's own content —
// a test that needs one stays on disk (bound_agent_disk_test.go,
// bound_agent_internal_test.go, uninstall_bound_agent_disk_test.go,
// missing_skill_disk_test.go, missing_skill_internal_test.go).
func newMemServer(mem *rwfs.Mem, opts ...setup.Option) *setup.Server {
	srv, _ := newMemServerRecording(mem, opts...)

	return srv
}

// newMemServerRecording is newMemServer's own twin for a test that needs
// the writableRecorder itself — asserting writableTargets' own contents or
// order, or that a DryRun/Print call never invokes it at all.
func newMemServerRecording(mem *rwfs.Mem, opts ...setup.Option) (*setup.Server, *writableRecorder) {
	rec := &writableRecorder{}

	all := make([]setup.Option, 0, len(opts)+4)
	all = append(all,
		setup.WithFSRoot(mem),
		setup.WithResolveRoot(identityResolveRoot),
		emptyHomeDir(),
		setup.WithWritableCheck(rec.check),
	)
	all = append(all, opts...)

	return setup.NewServer(all...), rec
}
