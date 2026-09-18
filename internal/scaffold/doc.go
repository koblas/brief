// Package scaffold creates the on-disk layout a feature needs before any
// scenario work starts: a feature directory holding a specification
// skeleton and a state file, both empty of prose. It owns "new feature"
// and "new step" — both compute the same feature-directory layout and both
// touch the progress list, so splitting them across packages would force a
// shared types package for nothing.
//
// scaffold writes through the real filesystem; there is no Store port. The
// contracts this package ships — mtime identity on re-finish, no temp file
// left behind, byte-identity after a refusal — are filesystem properties
// that an in-memory adapter cannot model, so every test that matters runs
// against a real directory tree regardless. The seam kept instead is a pure
// renderer, exercised only through Server.
package scaffold
