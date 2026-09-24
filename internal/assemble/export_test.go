package assemble

import "os"

// SetOpenRootForTest overrides s's directory-open function, letting a test
// in the external assemble_test package inject a failure opening a
// subdirectory (permission denied, a race, or any other os.Root.OpenRoot
// error) without depending on OS permission bits or effective uid — root
// bypasses ordinary permission checks, so a chmod-based test cannot
// reliably exercise this branch under every CI identity. osRoot.OpenRoot
// (fs.go) reads this field at every depth on the production path, so the
// override applies to Start's own per-feature open as well as Check's and
// Status's. It is exported only through this test-only file and is never
// part of assemble's public API.
func SetOpenRootForTest(s *Server, fn func(parent *os.Root, name string) (*os.Root, error)) {
	s.openRoot = fn
}
