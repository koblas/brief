package rwfs

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"
	"sync"
	"syscall"
	"testing/fstest"
	"time"
)

var _ FS = (*Mem)(nil)

// memCore is the state a Mem and every view OpenRoot returns from it share,
// so a write through any of them is visible from all the others.
type memCore struct {
	mu    sync.Mutex
	fsys  fstest.MapFS
	clock int64
}

// tick returns a fresh, strictly increasing time for a new or rewritten
// entry's ModTime. Callers hold c.mu already.
func (c *memCore) tick() time.Time {
	c.clock++

	return time.Unix(0, c.clock)
}

// Mem is an in-memory FS adapter backed by testing/fstest.MapFS, guarded by
// its core's mutex so it is safe for concurrent use. A Mem returned by
// NewMem is the root view; OpenRoot returns a second Mem over the same core,
// confined to a subtree, so writes through either are visible from both.
type Mem struct {
	core   *memCore
	prefix string
}

// NewMem returns a Mem seeded with fsys's entries, adding an explicit
// directory entry for every ancestor fsys implies but does not itself list,
// so that directory persists after its last explicit child is removed.
func NewMem(fsys fstest.MapFS) *Mem {
	seeded := make(fstest.MapFS, len(fsys))
	maps.Copy(seeded, fsys)
	materializeAncestors(seeded)

	return &Mem{core: &memCore{fsys: seeded}}
}

// materializeAncestors adds an explicit fs.ModeDir entry for every ancestor
// directory implied by fsys's keys that lacks one already.
func materializeAncestors(fsys fstest.MapFS) {
	names := make([]string, 0, len(fsys))
	for name := range fsys {
		names = append(names, name)
	}

	for _, name := range names {
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			if _, ok := fsys[dir]; ok {
				break
			}

			fsys[dir] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
		}
	}
}

// Snapshot returns a fresh copy of the whole tree m's core holds — every
// entry shared across every view OpenRoot has returned, not just the
// subtree m is confined to — safe for a caller to inspect, compare, or
// mutate without racing further calls on m or its relatives. Each entry's
// ModTime comes from m's fake clock, which advances on every write, so
// diffing two Snapshots distinguishes a rewrite from no write at all.
func (m *Mem) Snapshot() fstest.MapFS {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	out := make(fstest.MapFS, len(m.core.fsys))
	for name, file := range m.core.fsys {
		out[name] = &fstest.MapFile{
			Data:    bytes.Clone(file.Data),
			Mode:    file.Mode,
			ModTime: file.ModTime,
			Sys:     file.Sys,
		}
	}

	return out
}

// full translates name from m's view into the map-rooted path the shared
// core stores it under.
func (m *Mem) full(name string) string {
	if m.prefix == "" {
		return name
	}
	if name == "." {
		return m.prefix
	}

	return m.prefix + "/" + name
}

func (m *Mem) Open(name string) (fs.File, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("open", name); err != nil {
		return nil, err
	}

	f, err := m.core.fsys.Open(m.full(name))
	if err != nil {
		return nil, fmt.Errorf("rwfs: open %s: %w", name, err)
	}

	return f, nil
}

func (m *Mem) ReadFile(name string) ([]byte, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("readfile", name); err != nil {
		return nil, err
	}

	data, err := m.core.fsys.ReadFile(m.full(name))
	if err != nil {
		return nil, fmt.Errorf("rwfs: read %s: %w", name, err)
	}

	return data, nil
}

func (m *Mem) ReadDir(name string) ([]fs.DirEntry, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("readdir", name); err != nil {
		return nil, err
	}

	entries, err := m.core.fsys.ReadDir(m.full(name))
	if err != nil {
		return nil, fmt.Errorf("rwfs: read dir %s: %w", name, err)
	}

	return entries, nil
}

func (m *Mem) Stat(name string) (fs.FileInfo, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("stat", name); err != nil {
		return nil, err
	}

	info, err := m.core.fsys.Stat(m.full(name))
	if err != nil {
		return nil, fmt.Errorf("rwfs: stat %s: %w", name, err)
	}

	return info, nil
}

func (m *Mem) Lstat(name string) (fs.FileInfo, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("lstat", name); err != nil {
		return nil, err
	}

	info, err := m.core.fsys.Lstat(m.full(name))
	if err != nil {
		return nil, fmt.Errorf("rwfs: lstat %s: %w", name, err)
	}

	return info, nil
}

func (m *Mem) ReadLink(name string) (string, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("readlink", name); err != nil {
		return "", err
	}

	target, err := m.core.fsys.ReadLink(m.full(name))
	if err != nil {
		return "", fmt.Errorf("rwfs: readlink %s: %w", name, err)
	}

	return target, nil
}

// checkName validates name and reports syscall.ENOTDIR when some proper
// ancestor of name already exists as a non-directory entry.
func (m *Mem) checkName(op, name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	return m.notDirAncestor(op, name)
}

// notDirAncestor reports whether some proper ancestor of name already
// exists as a non-directory entry, walking from the view's root down so the
// first conflicting segment is the one reported.
func (m *Mem) notDirAncestor(op, name string) error {
	dir := path.Dir(name)
	if dir == "." {
		return nil
	}

	segments := strings.Split(dir, "/")
	prefix := ""
	for _, seg := range segments {
		if prefix == "" {
			prefix = seg
		} else {
			prefix = prefix + "/" + seg
		}

		info, err := m.core.fsys.Lstat(m.full(prefix))
		if err != nil {
			// prefix (and therefore name) is simply missing, not blocked by
			// a non-directory ancestor; the caller's own not-exist handling
			// reports this.
			return nil //nolint:nilerr // see comment above
		}
		if !info.IsDir() {
			return &fs.PathError{Op: op, Path: name, Err: syscall.ENOTDIR}
		}
	}

	return nil
}

// Mkdir creates name as a new, empty directory. See rwfs.FS for the
// contract.
func (m *Mem) Mkdir(name string, perm fs.FileMode) error {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("mkdir", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.core.fsys.Lstat(m.full(parent)); err != nil {
			return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
		}
	}

	full := m.full(name)
	if _, err := m.core.fsys.Lstat(full); err == nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	m.core.fsys[full] = &fstest.MapFile{Mode: fs.ModeDir | perm, ModTime: m.core.tick()}

	return nil
}

// MkdirAll creates name and every missing parent directory. See rwfs.FS for
// the contract.
func (m *Mem) MkdirAll(name string, perm fs.FileMode) error {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdirall", Path: name, Err: fs.ErrInvalid}
	}

	segments := strings.Split(name, "/")
	prefix := ""
	for i, seg := range segments {
		if prefix == "" {
			prefix = seg
		} else {
			prefix = prefix + "/" + seg
		}

		full := m.full(prefix)
		info, err := m.core.fsys.Lstat(full)
		switch {
		case err != nil:
			m.core.fsys[full] = &fstest.MapFile{Mode: fs.ModeDir | perm, ModTime: m.core.tick()}
		case info.IsDir():
			// already a directory; MkdirAll is idempotent here.
		case i == len(segments)-1:
			return &fs.PathError{Op: "mkdirall", Path: name, Err: fs.ErrExist}
		default:
			return &fs.PathError{Op: "mkdirall", Path: name, Err: syscall.ENOTDIR}
		}
	}

	return nil
}

// WriteFile replaces name's content atomically. See rwfs.FS for the
// contract, including how perm is treated differently on create than on
// replace.
func (m *Mem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("writefile", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.core.fsys.Lstat(m.full(parent)); err != nil {
			return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrNotExist}
		}
	}

	full := m.full(name)

	// Confirmed empirically against os.Root: renaming a regular file over an
	// existing directory fails with EEXIST, not EISDIR, so that is the
	// sentinel a caller can rely on from either adapter.
	mode := perm
	if existing, err := m.core.fsys.Lstat(full); err == nil {
		if existing.IsDir() {
			return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrExist}
		}

		// A replace keeps the existing mode only when it is a regular file.
		// Gating on IsRegular() matches atomicfile.replaceMode: without it,
		// replacing a symlink would hand its (often 0o777) mode to the new
		// regular file that takes its place, instead of perm.
		if existing.Mode().IsRegular() {
			mode = existing.Mode().Perm()
		}
	}

	m.core.fsys[full] = &fstest.MapFile{Data: bytes.Clone(data), Mode: mode, ModTime: m.core.tick()}

	return nil
}

// CreateExclusive creates name with data and perm. See rwfs.FS for the
// contract.
func (m *Mem) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("createexclusive", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.core.fsys.Lstat(m.full(parent)); err != nil {
			return &fs.PathError{Op: "createexclusive", Path: name, Err: fs.ErrNotExist}
		}
	}

	full := m.full(name)
	if _, err := m.core.fsys.Lstat(full); err == nil {
		return &fs.PathError{Op: "createexclusive", Path: name, Err: fs.ErrExist}
	}

	m.core.fsys[full] = &fstest.MapFile{Data: bytes.Clone(data), Mode: perm, ModTime: m.core.tick()}

	return nil
}

// Remove deletes name. See rwfs.FS for the contract.
func (m *Mem) Remove(name string) error {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("remove", name); err != nil {
		return err
	}

	full := m.full(name)
	info, err := m.core.fsys.Lstat(full)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if info.IsDir() {
		entries, err := m.core.fsys.ReadDir(full)
		if err != nil {
			return &fs.PathError{Op: "remove", Path: name, Err: err}
		}
		if len(entries) > 0 {
			return &fs.PathError{Op: "remove", Path: name, Err: syscall.ENOTEMPTY}
		}
	}

	delete(m.core.fsys, full)

	return nil
}

// maxSymlinkHops bounds resolveDir's symlink-following loop so a symlink
// cycle fails instead of spinning forever.
const maxSymlinkHops = 40

// OpenRoot returns name as a fresh Mem view over m's own core, confined to
// that subtree. See rwfs.FS for the contract.
func (m *Mem) OpenRoot(name string) (FS, error) {
	m.core.mu.Lock()
	defer m.core.mu.Unlock()

	if err := m.checkName("openroot", name); err != nil {
		return nil, err
	}

	full, err := m.resolveDir("openroot", name)
	if err != nil {
		return nil, err
	}

	return &Mem{core: m.core, prefix: full}, nil
}

// resolveDir resolves name to the map-rooted path of the directory it
// names, following a chain of symlink entries up to maxSymlinkHops deep and
// refusing an absolute or invalid target. It normalizes a fully-resolved
// "." to "".
func (m *Mem) resolveDir(op, name string) (string, error) {
	full := m.full(name)

	for range maxSymlinkHops {
		info, err := m.core.fsys.Lstat(full)
		if err != nil {
			return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			if !info.IsDir() {
				return "", &fs.PathError{Op: op, Path: name, Err: syscall.ENOTDIR}
			}
			if full == "." {
				full = ""
			}

			return full, nil
		}

		target, err := m.core.fsys.ReadLink(full)
		if err != nil {
			return "", &fs.PathError{Op: op, Path: name, Err: err}
		}
		if path.IsAbs(target) {
			return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
		}

		next := path.Join(path.Dir(full), target)
		if !fs.ValidPath(next) {
			return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
		}
		full = next
	}

	return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
}

// Close is a no-op: Mem holds no OS handle. It exists so Mem satisfies the
// same FS.Close a caller holding OS's nested root needs.
func (m *Mem) Close() error {
	return nil
}
