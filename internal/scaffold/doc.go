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
// the reverse order does not. A write failure after validation is
// returned as-is, never as a *RefusalError, since nothing changed on disk
// is not true past that point. Finishing an already-finished step with
// the same handoff and state is a true no-op: nothing is written and
// every file's modification time is preserved. Finishing it again with a
// handoff or state that differs from what is recorded is refused instead,
// naming the specific divergent file, unless the recorded handoff file is
// missing or unreadable, which exempts the step from the refusal
// entirely.
//
// A caller-facing refusal is a *RefusalError: a path, an optional line
// within it, what was wrong, and how to fix it, wrapping one of
// ErrNoSuchFeature, ErrFeatureExists, ErrMalformedFeature,
// ErrNoProgressHeading, ErrNoSuchStep, ErrNoProgressEntry,
// ErrUnterminatedFence, ErrOverCap, ErrMissingStateHeading,
// ErrOpenChecklistItem or ErrAlreadyFinished (or, from internal/platform/stepfile,
// ErrInvalidPattern, ErrInvalidHandoffSuffix or ErrNoStatusField) so
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
// scaffold writes through the real filesystem; there is no Store port. The
// contracts this package ships — no temp file left behind, byte-identity
// after a refusal — are filesystem properties that an in-memory adapter
// cannot model, so every test that matters runs against a real directory
// tree regardless. The seam kept instead is a pure renderer, exercised
// only through Server. Every write that replaces an existing file's full
// contents goes through internal/platform/atomicfile, so a reader never
// observes a truncated specification.
package scaffold
