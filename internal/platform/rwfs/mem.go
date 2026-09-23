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

// Mem is an in-memory FS adapter backed by testing/fstest.MapFS, guarded by
// a mutex so it is safe for concurrent use. It exists so internal/scaffold
// and internal/setup can exercise filesystem behavior without touching
// disk, sharing rwfs' contract test with the OS adapter.
//
// Every write replaces a map entry's *fstest.MapFile wholesale rather than
// mutating one in place. A reader that has already opened a file keeps
// referencing the old, now-orphaned *fstest.MapFile, so a concurrent write
// to the same name cannot corrupt bytes a reader is midway through — the
// same freedom from partial writes rwfs.FS.WriteFile documents.
type Mem struct {
	mu   sync.Mutex
	fsys fstest.MapFS
}

// NewMem returns a Mem seeded with fsys's entries, after adding an explicit
// directory entry for every ancestor fsys implies but does not itself list.
// Without that, a directory that exists in fsys only because some deeper
// entry's path implies it — the way testing/fstest.MapFS synthesizes such
// ancestors for reads — would disappear the moment its last explicit child
// is Remove'd; a real directory persists independent of its children.
func NewMem(fsys fstest.MapFS) *Mem {
	seeded := make(fstest.MapFS, len(fsys))
	maps.Copy(seeded, fsys)
	materializeAncestors(seeded)

	return &Mem{fsys: seeded}
}

// materializeAncestors adds an explicit fs.ModeDir entry for every ancestor
// directory implied by fsys's own keys, skipping any ancestor that already
// has an entry of its own.
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

// Snapshot returns a copy of m's current contents — a fresh map holding
// freshly cloned *fstest.MapFile values — safe for a caller to inspect,
// compare, or mutate without racing further calls on m or affecting m's own
// state. The copy includes an explicit fs.ModeDir entry for every directory
// materialized only because some deeper entry's path implied it — the same
// entries NewMem and MkdirAll add — not just the entries a caller wrote
// explicitly.
func (m *Mem) Snapshot() fstest.MapFS {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make(fstest.MapFS, len(m.fsys))
	for name, file := range m.fsys {
		out[name] = &fstest.MapFile{
			Data:    bytes.Clone(file.Data),
			Mode:    file.Mode,
			ModTime: file.ModTime,
			Sys:     file.Sys,
		}
	}

	return out
}

func (m *Mem) Open(name string) (fs.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("open", name); err != nil {
		return nil, err
	}

	f, err := m.fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: open %s: %w", name, err)
	}

	return f, nil
}

func (m *Mem) ReadFile(name string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("readfile", name); err != nil {
		return nil, err
	}

	data, err := m.fsys.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: read %s: %w", name, err)
	}

	return data, nil
}

func (m *Mem) ReadDir(name string) ([]fs.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("readdir", name); err != nil {
		return nil, err
	}

	entries, err := m.fsys.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: read dir %s: %w", name, err)
	}

	return entries, nil
}

func (m *Mem) Stat(name string) (fs.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("stat", name); err != nil {
		return nil, err
	}

	info, err := m.fsys.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: stat %s: %w", name, err)
	}

	return info, nil
}

func (m *Mem) Lstat(name string) (fs.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("lstat", name); err != nil {
		return nil, err
	}

	info, err := m.fsys.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("rwfs: lstat %s: %w", name, err)
	}

	return info, nil
}

func (m *Mem) ReadLink(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("readlink", name); err != nil {
		return "", err
	}

	target, err := m.fsys.ReadLink(name)
	if err != nil {
		return "", fmt.Errorf("rwfs: readlink %s: %w", name, err)
	}

	return target, nil
}

// checkName validates name and reports syscall.ENOTDIR when some proper
// ancestor of name already exists as a non-directory entry — a case a bare
// map lookup reports as fs.ErrNotExist instead, the way a real openat(2)
// walking that same path would not.
func (m *Mem) checkName(op, name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	return m.notDirAncestor(op, name)
}

// notDirAncestor reports whether some proper ancestor of name already
// exists as a non-directory entry, walking from the root down so the first
// segment that conflicts is the one reported — matching the order a real
// filesystem resolves path components in. It returns nil, deferring to the
// caller's own not-exist handling, the moment it reaches an ancestor that
// simply does not exist yet.
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

		info, err := m.fsys.Lstat(prefix)
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
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("mkdir", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.fsys.Lstat(parent); err != nil {
			return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
		}
	}

	if _, err := m.fsys.Lstat(name); err == nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	m.fsys[name] = &fstest.MapFile{Mode: fs.ModeDir | perm, ModTime: time.Time{}}

	return nil
}

// MkdirAll creates name and every missing parent directory. See rwfs.FS for
// the contract.
func (m *Mem) MkdirAll(name string, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

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

		info, err := m.fsys.Lstat(prefix)
		switch {
		case err != nil:
			m.fsys[prefix] = &fstest.MapFile{Mode: fs.ModeDir | perm, ModTime: time.Time{}}
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
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("writefile", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.fsys.Lstat(parent); err != nil {
			return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrNotExist}
		}
	}

	// Confirmed empirically against os.Root: renaming a regular file over an
	// existing directory fails with EEXIST, not EISDIR, so that is the
	// sentinel a caller can rely on from either adapter.
	mode := perm
	if existing, err := m.fsys.Lstat(name); err == nil {
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

	m.fsys[name] = &fstest.MapFile{Data: bytes.Clone(data), Mode: mode, ModTime: time.Time{}}

	return nil
}

// CreateExclusive creates name with data and perm. See rwfs.FS for the
// contract.
func (m *Mem) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("createexclusive", name); err != nil {
		return err
	}

	// notDirAncestor, run by checkName above, already rejects a parent that
	// exists as something other than a directory; only "parent missing"
	// remains to check here.
	parent := path.Dir(name)
	if parent != "." {
		if _, err := m.fsys.Lstat(parent); err != nil {
			return &fs.PathError{Op: "createexclusive", Path: name, Err: fs.ErrNotExist}
		}
	}

	if _, err := m.fsys.Lstat(name); err == nil {
		return &fs.PathError{Op: "createexclusive", Path: name, Err: fs.ErrExist}
	}

	m.fsys[name] = &fstest.MapFile{Data: bytes.Clone(data), Mode: perm, ModTime: time.Time{}}

	return nil
}

// Remove deletes name. See rwfs.FS for the contract.
func (m *Mem) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkName("remove", name); err != nil {
		return err
	}

	info, err := m.fsys.Lstat(name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if info.IsDir() {
		entries, err := m.fsys.ReadDir(name)
		if err != nil {
			return &fs.PathError{Op: "remove", Path: name, Err: err}
		}
		if len(entries) > 0 {
			return &fs.PathError{Op: "remove", Path: name, Err: syscall.ENOTEMPTY}
		}
	}

	delete(m.fsys, name)

	return nil
}
