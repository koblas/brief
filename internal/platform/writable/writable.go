package writable

import "os"

// Probe reports whether dir can be written to. It returns false if dir does
// not exist, is not a directory, or denies write access.
func Probe(dir string) bool {
	f, err := os.CreateTemp(dir, ".brief-writable-probe-*")
	if err != nil {
		return false
	}

	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)

	return true
}
