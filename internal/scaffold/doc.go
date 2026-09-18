// Package scaffold creates the on-disk layout a feature needs before any
// scenario work starts: a feature directory holding a specification
// skeleton and a state file, both empty of prose. It owns "new feature"
// and "new step" — both compute the same feature-directory layout and both
// touch the progress list, so splitting them across packages would force a
// shared types package for nothing.
//
// NewStep names each step file through internal/platform/stepfile,
// scanning the feature directory for the highest existing step number and
// writing the next one; the number is never derived from the progress
// list, so a deleted step file cannot cause a number to be reused.
// status: in a step's frontmatter is the sole authority on whether that
// step is done — the progress-list checkbox NewStep appends is a
// projection, not a second source of truth.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong with
// it, and how to fix it, wrapping one of ErrNoSuchFeature,
// ErrMalformedFeature or ErrNoProgressHeading (or, from
// internal/platform/stepfile, ErrInvalidPattern) so callers can branch on
// the specific cause with errors.Is while still rendering the same
// "nothing changed on disk" line.
//
// scaffold writes through the real filesystem; there is no Store port. The
// contracts this package ships — mtime identity on re-finish, no temp file
// left behind, byte-identity after a refusal — are filesystem properties
// that an in-memory adapter cannot model, so every test that matters runs
// against a real directory tree regardless. The seam kept instead is a pure
// renderer, exercised only through Server. Every write that replaces an
// existing file's full contents goes through
// internal/platform/atomicfile, so a reader never observes a truncated
// specification.
package scaffold
