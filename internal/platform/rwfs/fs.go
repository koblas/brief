package rwfs

import "io/fs"

// FS is a filesystem that supports both reading and writing, rooted so that
// every name is a slash-separated path satisfying fs.ValidPath — the same
// contract os.Root and fs.FS already share. Names never carry a leading
// slash and "." refers to the root itself.
//
// The read side is exactly the standard library's composition of
// interfaces: fs.FS, fs.ReadFileFS, fs.ReadDirFS, fs.StatFS and
// fs.ReadLinkFS (Lstat and ReadLink, for inspecting a symbolic link without
// following it). The write side is the minimal set brief's production code
// needs: creating a directory (Mkdir, MkdirAll), replacing a file's
// contents atomically (WriteFile), creating one only if absent
// (CreateExclusive), deleting an entry (Remove), and confining a caller to
// one subtree (OpenRoot).
//
// Every error a write method returns is a *fs.PathError with Op and Path
// set, Path being the name the caller passed — not an internal temp name a
// wrapped call happened to use. Sentinel checks use errors.Is against
// fs.ErrNotExist / fs.ErrExist, or against syscall.ENOTDIR / syscall.ENOTEMPTY
// for the two conditions io/fs has no sentinel for: an operation reaching
// through a path segment that exists but is not a directory, and removing a
// directory that still has entries.
//
//nolint:interfacebloat // contract-tested (rwfstest.Contract) against every adapter this repository builds; the read side alone is the standard library's own five-interface composition
type FS interface {
	fs.FS
	fs.ReadFileFS
	fs.ReadDirFS
	fs.StatFS
	fs.ReadLinkFS

	// Mkdir creates name as a new, empty directory with the given
	// permission bits. It returns a *fs.PathError wrapping fs.ErrExist
	// when name already exists, of any type, and one wrapping
	// fs.ErrNotExist when name's parent does not exist. Unlike MkdirAll,
	// Mkdir never creates more than the one, final path segment.
	Mkdir(name string, perm fs.FileMode) error

	// MkdirAll creates name and every missing parent directory, applying
	// perm to each one. Idempotent: a name that already exists as a
	// directory returns nil. It returns a *fs.PathError wrapping
	// fs.ErrExist when name exists as something else, or wrapping
	// syscall.ENOTDIR when an intermediate segment does.
	MkdirAll(name string, perm fs.FileMode) error

	// WriteFile replaces name's content atomically: a concurrent reader
	// observes either the previous content in full or the complete new
	// content, never a partial write. On create it applies perm; on
	// replace it keeps the target's existing permission bits. It does not
	// create missing parent directories: it returns a *fs.PathError
	// wrapping fs.ErrNotExist when name's parent does not exist, or
	// wrapping syscall.ENOTDIR when some other segment is not a directory.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// Remove deletes name. It returns a *fs.PathError wrapping
	// fs.ErrNotExist when name does not exist, and one wrapping
	// syscall.ENOTEMPTY when name is a directory that still has entries.
	Remove(name string) error

	// CreateExclusive creates name with data and perm, refusing to touch an
	// existing entry rather than replacing it. It returns a *fs.PathError
	// wrapping fs.ErrExist when name already exists, of any type, and one
	// wrapping fs.ErrNotExist when name's parent does not exist. Unlike
	// WriteFile, it gives no atomicity guarantee against a partial write on
	// the OS adapter.
	CreateExclusive(name string, data []byte, perm fs.FileMode) error

	// OpenRoot returns name as a fresh FS confined to that subtree: every
	// name a method on the result takes is relative to name. It follows a
	// symbolic link at name's final segment to a directory, refusing only
	// a resolution that would leave the receiver's root. It returns a
	// *fs.PathError wrapping fs.ErrNotExist when name does not exist, and
	// one wrapping syscall.ENOTDIR when name is not a directory.
	OpenRoot(name string) (FS, error)

	// Close releases any resource the receiver holds that a view returned
	// by OpenRoot does not share with its parent. On the OS adapter this
	// closes the underlying OS handle; operations on the receiver fail
	// afterward. On Mem it is a no-op.
	Close() error
}
