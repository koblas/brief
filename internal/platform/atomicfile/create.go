package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// compile-time proof that the documented interface contracts hold.
var (
	_ io.WriteCloser  = (*PendingFile)(nil)
	_ io.StringWriter = (*PendingFile)(nil)
)

// Create opens name for atomic replacement under root and returns a
// PendingFile: writes land in a temporary sibling of name, and Close
// renames that sibling over name, so a concurrent reader sees either
// name's previous content or its complete new content, never a partial
// write. Create performs no fsync — it claims atomicity, not durability.
func Create(root *os.Root, name string, perm fs.FileMode) (*PendingFile, error) {
	tmp := tempName(name)
	mode, existed := replaceMode(root, name, perm)

	if !existed {
		if err := removeStaleRegularSibling(root, tmp); err != nil {
			return nil, fmt.Errorf("atomicfile: write %s: %w", name, err)
		}
	}

	// O_TRUNC, not O_EXCL: a sibling left by a crashed write is reused
	// rather than wedging every future write, unless that stale sibling's
	// own mode carries no owner-write bit — then OpenFile fails until it
	// is removed by hand.
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return nil, fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	// Chmod runs only when mode came from an existing regular file: perm on
	// a fresh create is already umask-masked by OpenFile itself, and
	// re-applying it exactly with Chmod would defeat that umask.
	if existed {
		if err := f.Chmod(mode); err != nil {
			_ = f.Close()

			return nil, errors.Join(
				fmt.Errorf("atomicfile: chmod %s: %w", name, err),
				removeTmpFile(root, tmp, name),
			)
		}
	}

	return &PendingFile{root: root, file: f, tmp: tmp, name: name}, nil
}

// replaceMode returns the permission bits the temp sibling for name should
// be created with, and whether name already exists as a regular file —
// callers use that to decide whether Chmod must still apply the mode
// (OpenFile's mode argument is umask-masked and ignored on a reused sibling).
func replaceMode(root *os.Root, name string, perm fs.FileMode) (fs.FileMode, bool) {
	// Gated on IsRegular: a directory's mode (execute bit on every class)
	// must never be handed to the plain file being written.
	if info, err := root.Lstat(name); err == nil && info.Mode().IsRegular() {
		return info.Mode().Perm(), true
	}

	return perm, false
}

// removeStaleRegularSibling deletes tmp when it exists as a regular file,
// so OpenFile creates a fresh inode rather than reusing one whose mode
// reflects an earlier call. A directory at tmp is left alone: it is a seam
// some callers plant to make the following OpenFile fail.
func removeStaleRegularSibling(root *os.Root, tmp string) error {
	info, err := root.Lstat(tmp)
	if err != nil {
		// Nothing at tmp (including a fresh ErrNotExist) leaves nothing to
		// remove, and any other Lstat failure is not this function's to
		// report: the OpenFile that follows reaches the same path and
		// reports for itself.
		return nil //nolint:nilerr // see comment above
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	if err := root.Remove(tmp); err != nil {
		return fmt.Errorf("remove stale temp sibling: %w", err)
	}

	return nil
}

// PendingFile is an in-progress atomic replacement of a file: writes land in
// a temporary sibling, and Close renames that sibling over the target. It
// implements io.WriteCloser.
type PendingFile struct {
	root *os.Root
	file *os.File
	tmp  string
	name string

	// writeErr is the first error any Write returned; Close refuses to
	// commit when set.
	writeErr error

	closed bool
	// closeErr is the first Close's result; a second Close replays it.
	closeErr error
}

// Write writes b to the temp sibling. It records the first error it hits so
// that Close can refuse to commit. A Write after Close fails with
// os.ErrClosed, since every Close path closes the descriptor first.
func (p *PendingFile) Write(b []byte) (int, error) {
	return p.noteWriteErr(p.file.Write(b))
}

// WriteString writes s to the temp sibling, satisfying io.StringWriter. It
// is otherwise identical to Write.
func (p *PendingFile) WriteString(s string) (int, error) {
	return p.noteWriteErr(p.file.WriteString(s))
}

// noteWriteErr annotates a failed write and remembers the first one, so
// Close can refuse to commit regardless of which write method produced it.
func (p *PendingFile) noteWriteErr(n int, err error) (int, error) {
	if err == nil {
		return n, nil
	}

	wrapped := fmt.Errorf("atomicfile: write %s: %w", p.name, err)

	if p.writeErr == nil {
		p.writeErr = wrapped
	}

	return n, wrapped
}

// Close commits the replacement by renaming the temp sibling over the
// target, or abandons it and reports every reason it could not commit,
// joining the descriptor-close and temp-removal errors rather than hiding
// them. Test the result with errors.Is against an individual cause. A
// second Close replays the first Close's result without touching anything again.
func (p *PendingFile) Close() error {
	if p.closed {
		return p.closeErr
	}

	p.closed = true

	closeErr := p.wrap("close", p.file.Close())

	switch {
	case p.writeErr != nil:
		// Wrapped as "not replaced", not replayed verbatim: a caller checks
		// only Close, so this is the only signal most of them see that the
		// target was never touched.
		notReplaced := fmt.Errorf("atomicfile: %s not replaced: %w", p.name, p.writeErr)
		p.closeErr = errors.Join(notReplaced, closeErr, p.removeTmp())
	case closeErr != nil:
		p.closeErr = errors.Join(closeErr, p.removeTmp())
	default:
		if err := p.root.Rename(p.tmp, p.name); err != nil {
			p.closeErr = errors.Join(p.wrap("rename", err), p.removeTmp())
		}
	}

	return p.closeErr
}

// wrap annotates err with verb and the target's name, passing nil through
// so callers can join unconditionally.
func (p *PendingFile) wrap(verb string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("atomicfile: %s %s: %w", verb, p.name, err)
}

// removeTmp deletes the temp sibling of an abandoned replacement and
// reports a failure to do so.
func (p *PendingFile) removeTmp() error {
	return removeTmpFile(p.root, p.tmp, p.name)
}

// removeTmpFile deletes tmp, the temp sibling of name, under root,
// reporting fs.ErrNotExist as nil: no sibling to remove is not a failure.
func removeTmpFile(root *os.Root, tmp, name string) error {
	if err := root.Remove(tmp); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("atomicfile: remove temp file for %s: %w", name, err)
	}

	return nil
}

// tempName returns the deterministic temp sibling name for name: a leading
// dot and a fixed suffix on the base name, in the same directory. Fixed
// rather than random, so a sibling left by a crashed write is found and
// reused instead of accumulating.
func tempName(name string) string {
	dir, base := filepath.Split(name)

	return filepath.Join(dir, "."+base+".brief-tmp")
}
