package rwfs

import "io/fs"

// FS is a filesystem that supports both reading and writing, rooted so that
// every name is a slash-separated path satisfying fs.ValidPath — the same
// contract os.Root and fs.FS already share. Names never carry a leading
// slash and "." refers to the root itself.
//
// The read side is exactly the standard library's own composition of
// interfaces: fs.FS, fs.ReadFileFS, fs.ReadDirFS, fs.StatFS and
// fs.ReadLinkFS (Lstat and ReadLink, for inspecting a symbolic link without
// following it). The write side is the minimal set brief's production code
// needs: creating a directory (Mkdir, MkdirAll), replacing a file's
// contents atomically (WriteFile), and deleting an entry (Remove).
//
// Every error a write method returns is a *fs.PathError with Op and Path
// set, Path being the name the caller passed — not an internal temp name a
// wrapped call happened to use. Sentinel checks use errors.Is against
// fs.ErrNotExist / fs.ErrExist, or against syscall.ENOTDIR / syscall.ENOTEMPTY
// for the two conditions io/fs has no sentinel for: an operation reaching
// through a path segment that exists but is not a directory, and removing a
// directory that still has entries.
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
	// perm to each directory it creates. It is idempotent: calling it on
	// a name that already exists as a directory returns nil. It returns a
	// *fs.PathError wrapping fs.ErrExist when name itself exists as
	// something other than a directory, or one wrapping syscall.ENOTDIR
	// when an intermediate segment does.
	MkdirAll(name string, perm fs.FileMode) error

	// WriteFile replaces name's content atomically: a concurrent reader
	// observes either the previous content in full or the complete new
	// content, never a partial write. On create it applies perm; on
	// replace it keeps the target's existing permission bits and perm is
	// ignored — the same rule internal/platform/atomicfile implements.
	// WriteFile does not create missing parent directories: it returns a
	// *fs.PathError wrapping fs.ErrNotExist when name's parent does not
	// exist, or one wrapping syscall.ENOTDIR when some other segment of
	// name exists but is not a directory.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// Remove deletes name. It returns a *fs.PathError wrapping
	// fs.ErrNotExist when name does not exist, and one wrapping
	// syscall.ENOTEMPTY when name is a directory that still has entries.
	Remove(name string) error
}
