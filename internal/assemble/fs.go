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
// Status open the configured feature directory through: fsys is a
// "/"-rooted namespace, the same shape internal/scaffold's, internal/
// setup's and internal/doctor's own WithFS/WithFSRoot/WithRootFS already
// take, so one rwfs.Mem fixture can back every seamed package a
// command-level test constructs. Production leaves this unset (nil, the
// zero value): every entry point keeps opening the configured feature
// directory through a real, nested os.Root exactly as before this option
// existed — including Check's and Status's own openRoot test-injection
// seam (export_test.go), which WithFS never touches.
func WithFS(fsys rwfs.FS) Option {
	return func(s *Server) { s.rootFS = fsys }
}

// dirFS is the minimal directory capability Start, Check, Features and
// Status all share once the configured feature directory is open: FS is
// its own fs.FS view (fed straight into FeatureFS, or to fs.ReadDir for
// the top-level entry listing), Lstat inspects one entry by name without
// following a symlink (Check's own symlink guard, checked only at the top
// level), OpenRoot descends into a named entry as its own nested dirFS,
// and Close releases whatever the opener acquired.
type dirFS interface {
	FS() fs.FS
	Lstat(name string) (fs.FileInfo, error)
	OpenRoot(name string) (dirFS, error)
	Close() error
}

// osRoot adapts *os.Root to dirFS, the production path: FS, Lstat and
// Close call straight through to os.Root's own methods, so every error
// byte production ever returned stays unchanged. OpenRoot goes through its
// own openRoot field instead of calling (*os.Root).OpenRoot directly, so a
// test-injected open failure (export_test.go's SetOpenRootForTest) still
// applies at every depth, exactly as it did before dirFS existed.
type osRoot struct {
	r        *os.Root
	openRoot func(parent *os.Root, name string) (*os.Root, error)
}

// Every method below returns its underlying call's own error unwrapped,
// deliberately: every call site above (Start, Check, Features, Status)
// already reconstructs its own "assemble: ..." context around the exact
// same error os.Root or rwfs.FS has always returned, the same way it did
// before dirFS existed — wrapping it again here would stack a second,
// redundant prefix onto a message a caller already names.

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

// memDirFS adapts an rwfs.FS to dirFS, backing a WithFS fixture: fsys is
// already an fs.FS (rwfs.FS embeds it), so FS returns it directly; Lstat
// and OpenRoot call straight through to fsys's own methods (fs.ReadLinkFS
// and rwfs.FS.OpenRoot respectively), and Close matches rwfs.FS's own
// contract — a no-op on rwfs.Mem.
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
// expects: the leading path separator stripped, forward-slash separated,
// "." for the root itself. Duplicated from internal/scaffold's,
// internal/setup's, internal/platform/config's and internal/doctor's own
// identical helper, following this codebase's own precedent of copying an
// eight-line mapping rather than sharing it across packages with no other
// reason to depend on one another.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// openFeatureDir opens dir — the configured feature directory — as a
// dirFS: s.rootFS's own OpenRoot, mapped through fsName and wrapped in
// memDirFS, when WithFS is set; a real, nested os.Root wrapped in osRoot
// otherwise, carrying s.openRoot through so SetOpenRootForTest still
// applies at the per-feature hop below. The returned error is s.rootFS's
// or os.OpenRoot's own, unwrapped: every caller applies its own context.
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
