// Package scaffold owns the write path for a feature directory: creating
// one (NewFeature), adding a step to it (NewStep), and closing a step in
// it (Finish). internal/assemble is the read path over the same layout;
// the two packages never import each other.
//
// A step's frontmatter status is the sole authority on whether it is
// done; the progress-list checkbox NewStep appends and Finish ticks is a
// projection, not a second source of truth. Finish validates everything
// and computes every write body before the first byte reaches disk, then
// lands its four writes — handoff file, state file, step file,
// specification — in a fixed order chosen so a crash between them always
// converges on a retry.
//
// A caller-facing refusal is a *RefusalError: a path, an optional line, a
// problem and a fix, wrapping a sentinel a caller can branch on with
// errors.Is. An invalid feature name is the one exception — a bare
// ErrInvalidFeatureName, since it is an invocation defect rather than a
// write that changed nothing on disk.
//
// scaffold writes through internal/platform/rwfs.FS rather than the OS
// package directly; there is no Store port. NewFeature, NewStep and Finish
// each open one rwfs.FS and delegate to an exported FS-taking core
// (NewFeatureFS, NewStepFS, FinishFS); WithFS substitutes an rwfs.Mem for
// tests, so most of this package's suite touches no real disk. A few
// contracts stay properties of a real filesystem rwfs.Mem does not
// reproduce — see rwfs/doc.go — and those tests run against the OS adapter
// instead.
package scaffold
