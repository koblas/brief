package assemble

import "os"

// SetOpenRootForTest overrides s's directory-open function, letting a test
// inject an OpenRoot failure without depending on OS permission bits.
func SetOpenRootForTest(s *Server, fn func(parent *os.Root, name string) (*os.Root, error)) {
	s.openRoot = fn
}
