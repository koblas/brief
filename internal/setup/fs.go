package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// fsName maps path onto the name diskFS (or a test's own
// fstest.MapFS-backed rwfs.Mem standing in for it) expects: path
// absolutized first when it is not already (filepath.Abs, the process's
// own current directory — the same resolution every raw os.* call this
// seam replaced always applied to a relative root or wd), then the leading
// path separator stripped, forward-slash separated, "." for the root
// itself. Absolutizing is local to this mapping: it never changes what a
// caller sees in Result.Root or an Artifact's own Path, both of which stay
// exactly the root or wd string planning was given. A Mem-backed test
// always passes an already-absolute fsAbs(...) path, so filepath.Abs is a
// no-op there. Duplicated from the identical helper in
// internal/platform/config, internal/platform/repo and internal/doctor
// rather than shared, following those packages' own precedent — a shared
// package would invert the dependency those own for an eight-line mapping.
func fsName(path string) string {
	abs := path
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
	}

	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// diskFS is setup's own production rwfs.FS: a stateless, unconfined view of
// the whole "/"-rooted namespace, the same namespace config.rootFS,
// repo.rootFS and doctor's own (*Server).rootFS already read through. Every
// method performs exactly the os.* call setup's planning and apply cores
// used before this seam existed, joined onto "/" via fsName's own inverse —
// never routed through an os.Root confined below "/", so an ancestor
// symlink under a repository root (a ".claude" pointed elsewhere) is
// followed exactly as it always was: rwfs.OS's own confinement guarantee is
// deliberately not adopted here, since Init and Uninstall's own planning
// already has a passing test (bound_agent_disk_test.go's "a .claude symlinked
// outside the repo gets no row" case) that depends on today's
// symlink-following read behavior, and R10's own writability pre-check
// (checkWritable, unconfined, stays real disk) does not gate every write
// path a broader confinement would newly refuse. It exists purely so a test
// can substitute an rwfs.Mem for it (WithFSRoot); production behavior is
// unchanged by construction, since every method below is the same os.*
// call the code it replaces already made.
type diskFS struct{}

var _ rwfs.FS = diskFS{}

// abs turns name, already fs.ValidPath, into the absolute OS path diskFS's
// underlying os.* call takes.
func (diskFS) abs(name string) string {
	if name == "." {
		return string(filepath.Separator)
	}

	return string(filepath.Separator) + filepath.FromSlash(name)
}

// Every method below returns os.*'s own error unwrapped, deliberately: the
// setup.go and uninstall.go call sites that invoke them reconstruct
// today's exact "setup: <op> <path>: %w" text (fsName's inverse) around
// the very same *fs.PathError os.* has always returned — wrapping it again
// here would double that prefix and change the message this seam exists
// not to change.

//nolint:wrapcheck // see the block comment above
func (d diskFS) Open(name string) (fs.File, error) { return os.Open(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) ReadLink(name string) (string, error) { return os.Readlink(d.abs(name)) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) Mkdir(name string, perm fs.FileMode) error { return os.Mkdir(d.abs(name), perm) }

//nolint:wrapcheck // see the block comment above
func (d diskFS) MkdirAll(name string, perm fs.FileMode) error {
	return os.MkdirAll(d.abs(name), perm)
}

//nolint:wrapcheck // see the block comment above
func (d diskFS) Remove(name string) error { return os.Remove(d.abs(name)) }

// CreateExclusive is not exercised by Init or Uninstall — both always
// replace-or-create through WriteFile, never a create-only-if-absent write
// — but is implemented, directly against the target rather than a temp
// sibling, so diskFS still satisfies rwfs.FS in full.
func (d diskFS) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	f, err := os.OpenFile(d.abs(name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err //nolint:wrapcheck // see the block comment above Open
	}

	_, writeErr := f.Write(data)
	closeErr := f.Close()

	if writeErr != nil {
		return writeErr //nolint:wrapcheck // see the block comment above Open
	}

	return closeErr //nolint:wrapcheck // see the block comment above Open
}

// WriteFile replaces name's content atomically, exactly reproducing
// writePluginFile's, writeConfigFile's and writeSnippetFile's own former
// bodies: os.OpenRoot(dir), confined only to name's own immediate parent —
// never the repository root — then internal/platform/atomicfile.Create.
// Unlike rwfs.OS's own WriteFile, the error returned is not itself the
// caller-facing message: it is a *fs.PathError whose Op is "open" for an
// os.OpenRoot(dir) failure or "write" for anything atomicfile.Create,
// Write or Close reported, and whose Path is the absolute dir or name
// respectively, so writeThrough (setup.go) can rebuild the exact
// "setup: open %s: %w" / "setup: write %s: %w" text planning and apply
// produced before this seam existed, from pe.Path and pe.Err alone.
func (d diskFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	abs := d.abs(name)
	dir := filepath.Dir(abs)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return &fs.PathError{Op: "open", Path: dir, Err: err}
	}
	defer func() { _ = root.Close() }()

	w, err := atomicfile.Create(root, filepath.Base(abs), perm)
	if err != nil {
		return &fs.PathError{Op: "write", Path: abs, Err: err}
	}

	if _, err := w.Write(data); err != nil {
		_ = w.Close()

		return &fs.PathError{Op: "write", Path: abs, Err: err}
	}

	if err := w.Close(); err != nil {
		return &fs.PathError{Op: "write", Path: abs, Err: err}
	}

	return nil
}

// OpenRoot is unimplemented: rwfs.FS's own contract confines the returned
// view to name's own subtree, refusing a name that escapes it or does not
// resolve to a directory — a real guarantee diskFS's own stateless,
// unconfined "/"-rooted abs mapping cannot honor without becoming a second,
// narrower adapter in its own right. It is never called by Init or
// Uninstall (bound_agent.go's own confinedAgentFile opens its own os.Root
// directly, never through this seam); implemented to satisfy rwfs.FS's
// interface rather than silently returning a view that ignores name
// entirely.
func (diskFS) OpenRoot(name string) (rwfs.FS, error) {
	return nil, fmt.Errorf("setup: diskFS.OpenRoot(%s): %w", name, errors.ErrUnsupported)
}

// Close is a no-op: diskFS holds no resource of its own beyond the os.*
// calls each method already opens and closes around itself.
func (d diskFS) Close() error { return nil }
