package atomicfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes data to name under root by writing to a deterministic
// temporary sibling and renaming it over name, so a concurrent reader never
// observes a partially-written file.
//
// The temp sibling is opened O_CREATE|O_TRUNC, not O_EXCL: a temp left
// behind by a crashed write must be overwritten by the next attempt rather
// than wedging every future write, and R20's single-writer guarantee makes
// a genuine collision a non-concern.
//
// When name already exists and is a regular file, the temp sibling takes
// name's existing permission bits instead of perm, so a replace does not
// silently narrow or widen the file's mode; perm applies when name does
// not exist or is not a regular file. Looking the mode up through
// root.Lstat and gating on Mode().IsRegular() is load-bearing: a target
// that is a directory must not have that directory's mode handed to the
// temp file's creation, or the write would fail at temp creation instead
// of at the rename that is supposed to report the failure.
//
// WriteFile performs no fsync. It claims atomicity, not durability.
func WriteFile(root *os.Root, name string, data []byte, perm fs.FileMode) error {
	tmp := tempName(name)

	mode := perm
	if info, err := root.Lstat(name); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = root.Remove(tmp)

		return fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	if err := f.Close(); err != nil {
		_ = root.Remove(tmp)

		return fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)

		return fmt.Errorf("atomicfile: write %s: %w", name, err)
	}

	return nil
}

// tempName returns the deterministic temp sibling name for name: a leading
// dot and a fixed suffix on the base name, in the same directory.
func tempName(name string) string {
	dir, base := filepath.Split(name)

	return filepath.Join(dir, "."+base+".brief-tmp")
}
