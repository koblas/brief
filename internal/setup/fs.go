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

// fsName maps path onto the name diskFS (or a test's rwfs.Mem standing in
// for it) expects: absolutized, forward-slash separated, leading separator
// stripped, "." for the root itself.
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

// diskFS is setup's production rwfs.FS: a stateless, unconfined view of
// the whole "/"-rooted namespace, so an ancestor symlink is followed the
// same way every raw os.* call this seam replaced always did.
// fs_contract_internal_test.go runs rwfs' shared contract against it and
// declares its divergences rather than narrowing the contract.
type diskFS struct{}

var _ rwfs.FS = diskFS{}

// abs turns name into the absolute OS path diskFS's os.* call takes.
func (diskFS) abs(name string) string {
	if name == "." {
		return string(filepath.Separator)
	}

	return string(filepath.Separator) + filepath.FromSlash(name)
}

// Every method below returns os.*'s error unwrapped: the call sites that
// invoke them reconstruct the "setup: <op> <path>: %w" text themselves, so
// wrapping it here would double that prefix.

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

// CreateExclusive is not exercised by Init or Uninstall, but is
// implemented so diskFS still satisfies rwfs.FS in full.
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

// WriteFile replaces name's content atomically via
// internal/platform/atomicfile, confined only to name's immediate parent.
// The returned *fs.PathError's Op and Path let writeThrough rebuild its
// own "setup: open/write %s" text.
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

// OpenRoot is unimplemented: diskFS's unconfined mapping cannot honor
// rwfs.FS's subtree-confinement contract. Never called by Init or
// Uninstall.
func (diskFS) OpenRoot(name string) (rwfs.FS, error) {
	return nil, fmt.Errorf("setup: diskFS.OpenRoot(%s): %w", name, errors.ErrUnsupported)
}

// Close is a no-op: diskFS holds no resource of its own.
func (d diskFS) Close() error { return nil }
