package atomicfile

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Create opens name for atomic replacement under root and returns an
// io.WriteCloser whose Write goes straight to a temporary sibling of name
// and whose Close renames that sibling over name. A concurrent reader
// therefore observes either name's previous content or its complete new
// content, never a partial write, no matter how many Writes the caller
// makes or how far apart they are.
//
// Close is the commit, and it returns an error because the rename it
// performs can fail — a failure that no earlier call can report, since
// until Close runs nothing has touched name. A caller that checks only
// Write's error has checked nothing that matters.
//
// Abandoning the writer without calling Close leaves name untouched and
// the temp sibling on disk, which is the safe direction: the new content
// is lost rather than half-committed, and the next Create for the same
// name overwrites the stale sibling. Because Close commits rather than
// aborts, a bare "defer w.Close()" is not a rollback — on an early return
// it publishes whatever was written so far. Callers that hold the whole
// payload already should prefer WriteFile, which has no such window.
//
// Once a Write has failed, Close does not commit: it removes the temp
// sibling and returns that first write error, so a caller that ignores
// Write's return still cannot publish a truncated file. A second Close
// after the first returns nil and changes nothing, so the deferred-Close
// safety net can sit alongside an explicit Close whose error is checked.
//
// The temp sibling is opened O_CREATE|O_TRUNC, not O_EXCL: one left behind
// by a crashed write must be overwritten by the next attempt rather than
// wedging every future write, and R20's single-writer guarantee makes a
// genuine collision a non-concern.
//
// Create performs no fsync. It claims atomicity, not durability.
func Create(root *os.Root, name string, perm fs.FileMode) (io.WriteCloser, error) {
	p, err := newPendingFile(root, name, perm)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// newPendingFile opens the temp sibling and returns the concrete pending
// file. Create hands it out as an io.WriteCloser; WriteFile keeps the
// concrete type so that the errors it propagates are visibly the ones
// pendingFile already wrapped rather than opaque interface-method returns.
func newPendingFile(root *os.Root, name string, perm fs.FileMode) (*pendingFile, error) {
	tmp := tempName(name)

	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, replaceMode(root, name, perm))
	if err != nil {
		return nil, fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	return &pendingFile{root: root, file: f, tmp: tmp, name: name}, nil
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

// pendingFile is an in-progress atomic replacement of name: writes land in
// the temp sibling tmp, and Close renames tmp over name.
type pendingFile struct {
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
func (p *pendingFile) Write(b []byte) (int, error) {
	n, err := p.file.Write(b)
	if err != nil {
		wrapped := fmt.Errorf("atomicfile: write %s: %w", p.name, err)

		if p.writeErr == nil {
			p.writeErr = wrapped
		}

		return n, wrapped
	}

	return n, nil
}

// Close commits the replacement by renaming the temp sibling over name, or
// removes the sibling and reports why it could not. A second Close returns
// nil without touching anything.
func (p *pendingFile) Close() error {
	if p.closed {
		return nil
	}

	p.closed = true

	if p.writeErr != nil {
		_ = p.file.Close()
		_ = p.root.Remove(p.tmp)

		return p.writeErr
	}

	if err := p.file.Close(); err != nil {
		_ = p.root.Remove(p.tmp)

		return fmt.Errorf("atomicfile: write %s: %w", p.name, err)
	}

	if err := p.root.Rename(p.tmp, p.name); err != nil {
		_ = p.root.Remove(p.tmp)

		return fmt.Errorf("atomicfile: write %s: %w", p.name, err)
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
