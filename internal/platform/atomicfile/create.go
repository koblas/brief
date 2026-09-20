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
// PendingFile — an io.WriteCloser and io.StringWriter whose writes go
// straight to a temporary sibling of name and whose Close renames that
// sibling over name. A concurrent reader therefore observes either name's
// previous content or its complete new content, never a partial write, no
// matter how many writes the caller makes or how far apart they are.
//
// Close is the commit, and it returns an error because the rename it
// performs can fail — a failure that no earlier call can report, since until
// Close runs nothing has touched name. A caller that checks only Write's
// error has checked nothing that matters.
//
// Abandoning the writer without calling Close leaves name untouched and the
// temp sibling on disk, which is the safe direction: the new content is lost
// rather than half-committed, and the next Create for the same name
// overwrites the stale sibling. Because Close commits rather than aborts, a
// bare "defer w.Close()" is not a rollback — on an early return it publishes
// whatever was written so far.
//
// The temp sibling's mode is applied with Chmod once it is open, but only
// when name already existed as a regular file: OpenFile's mode argument is
// masked by the process umask, which is correct for perm on a fresh create,
// and is ignored outright when the sibling already exists from a crashed
// write, which Chmod must still correct. When name existed, replaceMode
// takes its bits instead of perm, and those must survive exactly —
// umask notwithstanding — so a replace does not silently narrow or widen a
// mode the caller never chose; Chmod is what makes that hold regardless of
// a reused stale sibling's own mode. When name did not exist, perm is
// exactly what OpenFile's masked mode argument already produced, and
// Chmod must not run — running it would defeat the umask on every fresh
// create.
//
// The temp sibling is opened O_CREATE|O_TRUNC, not O_EXCL: one left behind
// by a crashed write is overwritten by the next attempt rather than wedging
// every future write — except when that sibling's own mode carries no
// owner-write bit, in which case OpenFile fails with permission denied on
// every retry until the sibling is removed by hand.
//
// The condition is the sibling's mode, not the target's. A sibling acquires
// a read-only mode by being created for a read-only target — replaceMode
// takes the target's bits for the sibling too — but it is the sibling that
// OpenFile reopens, so chmodding the target afterwards does not lift the
// wedge, and a target that is read-only now does not cause one if the
// sibling left behind is writable. Deleting .<name>.brief-tmp is the
// recovery. brief runs at most one step per
// feature at a time with no concurrency machinery, so a genuine collision
// between two writers for the same name is not a concern this package has
// to handle.
//
// When name itself does not exist, a regular-file sibling left behind this
// way is removed before OpenFile runs, rather than reused: reusing it would
// make OpenFile skip its own mode argument (the kernel only applies that
// argument to an inode it actually creates), so a fresh create would inherit
// whatever mode an earlier, unrelated write happened to leave rather than
// perm masked by the current umask. That removal is why the owner-write
// exception above only applies when name exists — the delete goes through
// regardless of the sibling's own mode. A sibling that is a directory (or
// anything else OpenFile cannot reuse) is left alone; OpenFile fails on its
// own and that failure is a seam some callers use to inject one.
//
// Create performs no fsync. It claims atomicity, not durability.
func Create(root *os.Root, name string, perm fs.FileMode) (*PendingFile, error) {
	tmp := tempName(name)
	mode, existed := replaceMode(root, name, perm)

	if !existed {
		if err := removeStaleRegularSibling(root, tmp); err != nil {
			return nil, fmt.Errorf("atomicfile: write %s: %w", name, err)
		}
	}

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
// be created with, and whether name already exists as a regular file. When
// it does, replaceMode returns name's existing bits instead of perm, so a
// replace does not silently narrow or widen the file's mode; perm applies,
// with existed false, when name does not exist or is not a regular file —
// callers use existed to decide whether that mode still needs Chmod to
// survive OpenFile's umask-masked mode argument, or is already correct as
// OpenFile produced it.
//
// Gating on Mode().IsRegular() is load-bearing: without it, a target that is
// a directory would hand that directory's mode — typically carrying the
// execute bit on every class — to a plain file, instead of perm.
func replaceMode(root *os.Root, name string, perm fs.FileMode) (fs.FileMode, bool) {
	if info, err := root.Lstat(name); err == nil && info.Mode().IsRegular() {
		return info.Mode().Perm(), true
	}

	return perm, false
}

// removeStaleRegularSibling deletes tmp when it already exists as a regular
// file, so the OpenFile that follows creates a brand-new inode rather than
// reusing one whose mode reflects an earlier, unrelated call. It leaves tmp
// alone when it does not exist, or exists as something other than a regular
// file — a directory at tmp is a seam some callers plant on purpose to make
// the following OpenFile fail, and this must not clear that seam.
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

	// writeErr holds the first error any Write returned. Close reports it
	// instead of committing, so an ignored Write error cannot publish a
	// truncated file.
	writeErr error

	closed bool
	// closeErr holds the first Close's result, so a second Close replays it
	// instead of returning nil regardless of how the first one went.
	closeErr error
}

// Write writes b to the temp sibling. It records the first error it hits so
// that Close can refuse to commit.
//
// A Write after Close fails with os.ErrClosed because every Close path
// closes the underlying descriptor first; there is no separate guard here,
// since one would be unfalsifiable — no test can distinguish it from the
// descriptor's own refusal.
func (p *PendingFile) Write(b []byte) (int, error) {
	return p.noteWriteErr(p.file.Write(b))
}

// WriteString writes s to the temp sibling, satisfying io.StringWriter so a
// caller that already holds a string does not have to copy it into a []byte
// to write it. It is otherwise identical to Write, including how a failure
// stops Close from committing.
func (p *PendingFile) WriteString(s string) (int, error) {
	return p.noteWriteErr(p.file.WriteString(s))
}

// noteWriteErr annotates a failed underlying write and remembers the first
// one, so that Close can refuse to commit no matter which write method
// produced it.
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
// target, or abandons it and reports every reason it could not commit.
//
// Close discards no error. When it abandons the replacement it still has to
// close the descriptor and remove the temp sibling, and either of those can
// fail too; the returned error joins all of them, because a temp sibling
// that could not be removed contradicts this package's claim to leave none
// behind and must not be hidden. Test the result with errors.Is against an
// individual cause rather than comparing it directly.
//
// A Close that will not commit because an earlier write failed reports that
// as "<name> not replaced", wrapping the write error as the cause. Replaying
// the write error verbatim would say nothing about whether the target was
// touched — and since callers check Close and not the write, that is the
// only error most of them see.
//
// A second Close replays the first Close's result without touching anything
// again, so a deferred "defer w.Close()" safety net can sit alongside an
// explicit Close whose error is checked, and a caller that checks only the
// second one still sees whatever the first one found.
func (p *PendingFile) Close() error {
	if p.closed {
		return p.closeErr
	}

	p.closed = true

	closeErr := p.wrap("close", p.file.Close())

	switch {
	case p.writeErr != nil:
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

// removeTmp deletes the temp sibling of an abandoned replacement, reporting
// a failure to do so. A leftover sibling is not fatal — the next Create for
// the same name truncates it — but it is a broken promise, so it is reported
// rather than swallowed.
func (p *PendingFile) removeTmp() error {
	return removeTmpFile(p.root, p.tmp, p.name)
}

// removeTmpFile deletes tmp, the temp sibling of name, under root. It
// reports fs.ErrNotExist as nil: a sibling that was never left behind (or
// was already removed) is not a broken promise, so a caller must not report
// "remove temp file for <name>" when there was never one to remove.
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
// dot and a fixed suffix on the base name, in the same directory. The name
// is deterministic rather than random so that a sibling left by a crashed
// write is found and reused instead of accumulating, and so that planting a
// directory at this path is a usable seam for injecting a real write
// failure in tests.
func tempName(name string) string {
	dir, base := filepath.Split(name)

	return filepath.Join(dir, "."+base+".brief-tmp")
}
