package rwfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/koblas/brief/internal/platform/atomicfile"
)

var _ FS = (*OS)(nil)

// OS is the production FS adapter: every operation is confined to a single
// directory tree by an *os.Root, so a name that would traverse outside the
// root — including via a symbolic link — is refused rather than followed.
type OS struct {
	root *os.Root
	fsys fs.FS // root.FS(): already ReadFileFS, ReadDirFS, StatFS and ReadLinkFS.
}

// OpenOS opens dir as the root of a new OS, confining every subsequent
// operation on the returned OS to dir's tree.
func OpenOS(dir string) (*OS, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("rwfs: open %s: %w", dir, err)
	}

	return &OS{root: root, fsys: root.FS()}, nil
}

// Close releases the file descriptor or handle the underlying *os.Root
// holds. Operations on o after Close fail.
func (o *OS) Close() error {
	if err := o.root.Close(); err != nil {
		return fmt.Errorf("rwfs: close: %w", err)
	}

	return nil
}

func (o *OS) Open(name string) (fs.File, error) {
	f, err := o.fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: open %s: %w", name, err)
	}

	return f, nil
}

func (o *OS) ReadFile(name string) ([]byte, error) {
	data, err := o.fsys.(fs.ReadFileFS).ReadFile(name) //nolint:forcetypeassert // root.FS() always implements ReadFileFS
	if err != nil {
		return nil, fmt.Errorf("rwfs: read %s: %w", name, err)
	}

	return data, nil
}

func (o *OS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := o.fsys.(fs.ReadDirFS).ReadDir(name) //nolint:forcetypeassert // root.FS() always implements ReadDirFS
	if err != nil {
		return nil, fmt.Errorf("rwfs: read dir %s: %w", name, err)
	}

	return entries, nil
}

func (o *OS) Stat(name string) (fs.FileInfo, error) {
	info, err := o.fsys.(fs.StatFS).Stat(name) //nolint:forcetypeassert // root.FS() always implements StatFS
	if err != nil {
		return nil, fmt.Errorf("rwfs: stat %s: %w", name, err)
	}

	return info, nil
}

func (o *OS) Lstat(name string) (fs.FileInfo, error) {
	info, err := o.fsys.(fs.ReadLinkFS).Lstat(name) //nolint:forcetypeassert // root.FS() always implements ReadLinkFS
	if err != nil {
		return nil, fmt.Errorf("rwfs: lstat %s: %w", name, err)
	}

	return info, nil
}

func (o *OS) ReadLink(name string) (string, error) {
	target, err := o.fsys.(fs.ReadLinkFS).ReadLink(name) //nolint:forcetypeassert // root.FS() always implements ReadLinkFS
	if err != nil {
		return "", fmt.Errorf("rwfs: readlink %s: %w", name, err)
	}

	return target, nil
}

// Mkdir creates name as a new, empty directory. See rwfs.FS for the
// contract.
func (o *OS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrInvalid}
	}

	if err := o.root.Mkdir(name, perm); err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	return nil
}

// MkdirAll creates name and every missing parent directory. See rwfs.FS for
// the contract.
func (o *OS) MkdirAll(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdirall", Path: name, Err: fs.ErrInvalid}
	}

	if err := o.root.MkdirAll(name, perm); err != nil {
		return &fs.PathError{Op: "mkdirall", Path: name, Err: err}
	}

	return nil
}

// WriteFile replaces name's content atomically via
// internal/platform/atomicfile. See rwfs.FS for the contract, including how
// perm is treated differently on create than on replace.
//
// atomicfile's own error wraps the temp sibling's name, not name itself
// (Create's failure) or nothing at all identifiable (Close's rename
// failure). Every path below is re-wrapped in a *fs.PathError naming name,
// so a caller sees the name it passed regardless of which internal step
// failed.
func (o *OS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrInvalid}
	}

	w, err := atomicfile.Create(o.root, name, perm)
	if err != nil {
		return &fs.PathError{Op: "writefile", Path: name, Err: err}
	}

	// Close must run even when Write failed: per atomicfile.PendingFile,
	// Close — not Write — is what closes the descriptor and, since Write
	// failed, abandons the replacement and removes the temp sibling.
	_, writeErr := w.Write(data)
	closeErr := w.Close()

	if joined := errors.Join(writeErr, closeErr); joined != nil {
		return &fs.PathError{Op: "writefile", Path: name, Err: joined}
	}

	return nil
}

// CreateExclusive creates name with data and perm, refusing an existing
// entry rather than replacing it. See rwfs.FS for the contract, including
// the partial-write guarantee this method does not give that WriteFile
// does.
func (o *OS) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "createexclusive", Path: name, Err: fs.ErrInvalid}
	}

	f, err := o.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return &fs.PathError{Op: "createexclusive", Path: name, Err: err}
	}

	_, writeErr := f.Write(data)
	closeErr := f.Close()

	if joined := errors.Join(writeErr, closeErr); joined != nil {
		return &fs.PathError{Op: "createexclusive", Path: name, Err: joined}
	}

	return nil
}

// Remove deletes name. See rwfs.FS for the contract.
func (o *OS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	if err := o.root.Remove(name); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	return nil
}
