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
// whatever was written so far. A caller holding the whole payload should
// write it and then check Close, which is the shape every caller in this
// repository uses.
//
// Once a write has failed, Close does not commit: it removes the temp
// sibling and reports that name was not replaced, wrapping that first write
// error as the cause, so a caller that ignores a write's return still cannot
// publish a truncated file and still learns the target is untouched. A second
// Close returns nil and changes nothing, so the deferred-Close safety net can
// sit alongside an explicit Close whose error is checked.
//
// The temp sibling is opened O_CREATE|O_TRUNC, not O_EXCL: one left behind
// by a crashed write must be overwritten by the next attempt rather than
// wedging every future write, and R20's single-writer guarantee makes a
// genuine collision a non-concern.
//
// Create performs no fsync. It claims atomicity, not durability.
func Create(root *os.Root, name string, perm fs.FileMode) (*PendingFile, error) {
	tmp := tempName(name)

	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, replaceMode(root, name, perm))
	if err != nil {
		return nil, fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	return &PendingFile{root: root, file: f, tmp: tmp, name: name}, nil
}

// replaceMode returns the permission bits the temp sibling for name should
// be created with. When name already exists and is a regular file it takes
// name's existing bits instead of perm, so a replace does not silently
// narrow or widen the file's mode; perm applies when name does not exist or
// is not a regular file.
//
// Gating on Mode().IsRegular() is load-bearing: a target that is a
// directory must not have that directory's mode handed to the temp file's
// creation, or the write would fail at temp creation instead of at the
// rename that is supposed to report the failure.
func replaceMode(root *os.Root, name string, perm fs.FileMode) fs.FileMode {
	if info, err := root.Lstat(name); err == nil && info.Mode().IsRegular() {
		return info.Mode().Perm()
	}

	return perm
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
}

// Write writes b to the temp sibling. It records the first error it hits so
// that Close can refuse to commit.
//
// A Write after Close fails with os.ErrClosed because every Close path
// closes the underlying descriptor first; there is no separate guard here,
// since one would be unfalsifiable — no test can distinguish it from the
// descriptor's own refusal.
func (p *PendingFile) Write(b []byte) (int, error) {
	return p.record(p.file.Write(b))
}

// WriteString writes s to the temp sibling, satisfying io.StringWriter so a
// caller that already holds a string does not have to copy it into a []byte
// to write it. It is otherwise identical to Write, including how a failure
// stops Close from committing.
func (p *PendingFile) WriteString(s string) (int, error) {
	return p.record(p.file.WriteString(s))
}

// record annotates a failed underlying write and remembers the first one, so
// that Close can refuse to commit no matter which write method produced it.
func (p *PendingFile) record(n int, err error) (int, error) {
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
// individual cause rather than comparing it directly — unlike gzip.Writer
// and tar.Writer, which replay a sticky write error verbatim, Close returns
// a joined error and not the identical value.
//
// A Close that will not commit because an earlier write failed reports that
// as "<name> not replaced", wrapping the write error as the cause. Replaying
// the write error verbatim, as the stdlib writers do, would say nothing about
// whether the target was touched — and since callers check Close and not the
// write, that is the only error most of them see.
//
// A second Close returns nil without touching anything.
func (p *PendingFile) Close() error {
	if p.closed {
		return nil
	}

	p.closed = true

	closeErr := p.wrap(p.file.Close())

	if p.writeErr != nil {
		notReplaced := fmt.Errorf("atomicfile: %s not replaced: %w", p.name, p.writeErr)

		return errors.Join(notReplaced, closeErr, p.removeTmp())
	}

	if closeErr != nil {
		return errors.Join(closeErr, p.removeTmp())
	}

	if err := p.root.Rename(p.tmp, p.name); err != nil {
		return errors.Join(p.wrap(err), p.removeTmp())
	}

	return nil
}

// wrap annotates err with the target's name, passing nil through so callers
// can join unconditionally.
func (p *PendingFile) wrap(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("atomicfile: write %s: %w", p.name, err)
}

// removeTmp deletes the temp sibling of an abandoned replacement, reporting
// a failure to do so. A leftover sibling is not fatal — the next Create for
// the same name truncates it — but it is a broken promise, so it is reported
// rather than swallowed.
func (p *PendingFile) removeTmp() error {
	if err := p.root.Remove(p.tmp); err != nil {
		return fmt.Errorf("atomicfile: remove temp file for %s: %w", p.name, err)
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
