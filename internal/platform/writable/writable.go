package writable

import "os"

// Probe reports whether dir can be written to: it creates a temporary file
// inside dir, closes it, and removes it immediately. It is the only write
// either caller performs to answer this question — never syscall.Access
// (unix-only, and go.mod carries no golang.org/x/sys) or inspecting
// permission bits by hand, which root-owned directories and ACLs can make
// misleading.
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
