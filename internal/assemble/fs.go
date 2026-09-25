package assemble

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/rwfs"
)

// Option configures a Server built by NewServer.
type Option func(*Server)

// WithFS overrides the production filesystem Start, Check, Features and
// Status open the configured feature directory through, for tests. fsys is
// a "/"-rooted namespace, the same shape internal/scaffold's and internal/
// setup's own WithFS already take. Left unset, every entry point opens the
// configured feature directory through a real, nested os.Root.
func WithFS(fsys rwfs.FS) Option {
	return func(s *Server) { s.rootFS = fsys }
}

// dirFS is the directory capability Start, Check, Features and Status all
// share once the configured feature directory is open. Lstat inspects one
// entry without following a symlink; OpenRoot descends into a named entry
// as its own nested dirFS.
type dirFS interface {
	FS() fs.FS
	Lstat(name string) (fs.FileInfo, error)
	OpenRoot(name string) (dirFS, error)
	Close() error
}

// osRoot adapts *os.Root to dirFS, the production path. OpenRoot goes
// through its own openRoot field rather than calling (*os.Root).OpenRoot
// directly, so a test-injected open failure (export_test.go's
// SetOpenRootForTest) still applies at every depth.
//
// osRoot's OpenRoot does not classify a raw *os.Root.OpenRoot failure the
// way rwfs.OS's own OpenRoot does: opening a file as if it were a
// directory reports a bare "not a directory" string here rather than
// syscall.ENOTDIR. Start reaches this uncaught (no prior Lstat); Check and
// Status both Lstat and check IsDir() first, so neither does. Classifying
// it would change Start's wrapped error text, so it stays documented
// rather than fixed.
type osRoot struct {
	r        *os.Root
	openRoot func(parent *os.Root, name string) (*os.Root, error)
}

// Every method below returns its underlying call's error unwrapped: each
// call site (Start, Check, Features, Status) already wraps it with its own
// "assemble: ..." context.

func (o osRoot) FS() fs.FS                              { return o.r.FS() }
func (o osRoot) Lstat(name string) (fs.FileInfo, error) { return o.r.Lstat(name) } //nolint:wrapcheck // see the block comment above
func (o osRoot) Close() error                           { return o.r.Close() }     //nolint:wrapcheck // see the block comment above

func (o osRoot) OpenRoot(name string) (dirFS, error) {
	r, err := o.openRoot(o.r, name)
	if err != nil {
		return nil, err
	}

	return osRoot{r: r, openRoot: o.openRoot}, nil
}

// memDirFS adapts an rwfs.FS to dirFS, backing a WithFS fixture.
type memDirFS struct {
	fsys rwfs.FS
}

func (m memDirFS) FS() fs.FS                              { return m.fsys }
func (m memDirFS) Lstat(name string) (fs.FileInfo, error) { return m.fsys.Lstat(name) } //nolint:wrapcheck // see osRoot's own block comment above
func (m memDirFS) Close() error                           { return m.fsys.Close() }     //nolint:wrapcheck // see osRoot's own block comment above

func (m memDirFS) OpenRoot(name string) (dirFS, error) {
	sub, err := m.fsys.OpenRoot(name)
	if err != nil {
		return nil, err //nolint:wrapcheck // see osRoot's own block comment above
	}

	return memDirFS{fsys: sub}, nil
}

// fsName maps abs, an absolute OS path, onto the name a WithFS fixture
// expects: leading separator stripped, forward-slash separated, "." for
// the root itself. Duplicated from sibling packages rather than shared,
// since they have no other reason to depend on one another.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// openFeatureDir opens dir, the configured feature directory, as a dirFS:
// through s.rootFS when WithFS is set, otherwise a real, nested os.Root.
func (s *Server) openFeatureDir(dir string) (dirFS, error) {
	if s.rootFS != nil {
		sub, err := s.rootFS.OpenRoot(fsName(dir))
		if err != nil {
			return nil, err //nolint:wrapcheck // see osRoot's own block comment above
		}

		return memDirFS{fsys: sub}, nil
	}

	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err //nolint:wrapcheck // see osRoot's own block comment above
	}

	return osRoot{r: r, openRoot: s.openRoot}, nil
}
