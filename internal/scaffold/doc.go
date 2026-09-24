// Package scaffold owns the write path for a feature directory: creating
// one (NewFeature), adding a step to it (NewStep), and closing a step in
// it (Finish). internal/assemble is the read path over the same layout;
// the two packages never import each other, so a change to how a feature
// is read can never accidentally also change how it is written.
//
// NewStep names each step file through internal/platform/stepfile,
// scanning the feature directory for the highest existing step number and
// writing the next one; the number is never derived from the progress
// list, so a deleted step file cannot cause a number to be reused.
// status: in a step's frontmatter is the sole authority on whether that
// step is done — the progress-list checkbox NewStep appends, and Finish
// ticks, is a projection, not a second source of truth.
//
// Finish writes a step's handoff to its own file (named
// internal/platform/stepfile.CompileHandoff beside the step file) and
// replaces a feature's state file, then marks the step's frontmatter
// done, validating everything and computing every write body before the
// first byte reaches disk. Its four writes land in a fixed order — handoff
// file, state file, step file, specification — chosen so a crash between
// them always converges on a retry; see the doc comment on Finish for why
// the reverse order does not. On success it returns a FinishResult naming
// the absolute handoff and state paths it wrote, whether anything changed,
// and the id of the next open step (ignoring depends-on) — the same step
// brief start would brief next, computed before any write so it is
// present even on a no-op. A write failure after validation is returned
// as-is, never as a *RefusalError, since nothing changed on disk is not
// true past that point. Finishing an already-finished step with the same
// handoff and state is a true no-op: nothing is written and every file's
// modification time is preserved, and FinishResult.Changed reports false.
// Finishing it again with a handoff or state that differs from what is
// recorded is refused instead, naming the specific divergent file, unless
// the recorded handoff file is missing or unreadable, which exempts the
// step from the refusal entirely.
//
// A caller-facing refusal is a *RefusalError: a path, an optional line
// within it, what was wrong, and how to fix it, wrapping one of
// ErrNoSuchFeature, ErrFeatureExists, ErrMalformedFeature,
// ErrNoProgressHeading, ErrNoSuchStep, ErrNoProgressEntry or
// ErrAlreadyFinished; or, from internal/platform/conform, ErrUnterminatedFence,
// ErrOverCap, ErrMissingStateHeading or ErrOpenChecklistItem — the four
// body-shaped predicates internal/assemble's Check reports as findings
// against the same rule; or, from internal/platform/stepfile,
// ErrInvalidPattern, ErrInvalidHandoffSuffix or ErrNoStatusField — so
// callers can branch on the specific cause with
// errors.Is while still rendering the same "nothing changed on disk"
// line. A refusal about the state argument's own bytes, rather than about
// a file Finish opened, carries Path == StateSource — a placeholder cli
// replaces with the real --state source before rendering, since Finish
// itself never learns it. A "## Handoff" section left behind in a step
// file by an unmigrated tree is ordinary prose Finish never reads.
//
// One refusal is not a *RefusalError: an empty or whitespace-carrying name
// given to NewFeature is an invocation defect, not a write that changed
// nothing on disk, so it is reported as the bare sentinel
// ErrInvalidFeatureName and cli classifies it as a usage error rather than
// rendering it with the "(no files changed)" write-refusal template.
//
// scaffold writes through internal/platform/rwfs.FS rather than the OS
// package directly; there is no Store port. NewFeature, NewStep and Finish
// each build one rwfs.FS through Server's own mkdirAll/openDir methods — an
// OS adapter, via rwfs.OpenOS, confined to the configured feature directory
// or, for NewStep and Finish, nested one level deeper at the feature's own
// subdirectory, in production — and delegate to an exported FS-taking core
// (NewFeatureFS, NewStepFS, FinishFS) that carries every check and write.
// That core is what most of this package's tests exercise, against
// rwfs.Mem instead of a real directory tree; a command-level test one
// layer up (internal/cli) reaches the same fixture through NewServer's own
// WithFS Option, which substitutes an rwfs.Mem for mkdirAll/openDir's
// production bodies wholesale — os.MkdirAll and rwfs.OpenOS — so no real
// disk is touched from NewFeature down. A handful of contracts remain
// properties of a real filesystem that rwfs.Mem does not reproduce — a
// symlinked feature entry refused rather than followed, a write blocked by
// a directory at atomicfile's own temp-sibling name, a file's permission
// bits under a pinned umask — and those tests run against the OS adapter
// instead, WithFS unset; see rwfs/doc.go for the full list of what Mem does
// not model. Every write that replaces an existing file's full contents
// goes through rwfs.FS.WriteFile — atomicfile on the OS adapter — so a
// reader never observes a truncated specification.
package scaffold
