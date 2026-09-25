package scaffold

import (
	"errors"
	"fmt"

	"github.com/koblas/brief/internal/platform/conform"
)

// ErrInvalidFeatureName is returned when NewFeature is asked to create an
// empty name, or one containing whitespace. It is a bare sentinel, not a
// *RefusalError: an invalid name is an invocation defect, reported by cli
// as a usage error rather than as a write refusal.
var ErrInvalidFeatureName = errors.New("invalid feature name")

// ErrNoSuchFeature is returned when the named feature has no directory
// under the configured feature directory.
var ErrNoSuchFeature = errors.New("no such feature")

// ErrFeatureExists is returned when NewFeature is asked to create a
// feature whose directory already exists. It travels inside a
// *RefusalError naming that directory.
var ErrFeatureExists = errors.New("feature already exists")

// ErrMalformedFeature is returned when a feature directory exists but is
// missing a file this package requires, such as its specification.
var ErrMalformedFeature = errors.New("malformed feature")

// ErrNoProgressHeading is returned when a feature's specification has no
// line matching the configured progress heading.
var ErrNoProgressHeading = errors.New("no progress heading found")

// ErrNoSuchStep is returned when a feature directory exists but has no
// step file whose id equals the one Finish was asked to close.
var ErrNoSuchStep = errors.New("no such step")

// ErrNoProgressEntry is returned when a feature's progress list has no
// entry for a step whose file does exist.
var ErrNoProgressEntry = errors.New("no progress entry found")

// ErrUnterminatedFence is returned when a replacement state body given to
// Finish opens a fenced code block it never closes. It is
// conform.ErrUnterminatedFence.
var ErrUnterminatedFence = conform.ErrUnterminatedFence

// ErrAlreadyFinished is returned when Finish is asked to close a step whose
// frontmatter already says done, but the supplied handoff or state differs
// from what is recorded on disk. It travels inside a *RefusalError naming
// the specific divergent file, so the caller can read the recorded bytes
// and compare rather than having their new input silently discarded.
var ErrAlreadyFinished = errors.New("step already finished with different inputs")

// ErrOverCap is returned when a body Finish is asked to write measures more
// lines than its configured cap (cfg.HandoffCapLines or cfg.StateCapLines).
// RefusalError.Path names which body: HandoffSource or StateSource. It is
// conform.ErrOverCap.
var ErrOverCap = conform.ErrOverCap

// ErrOpenChecklistItem is returned when Finish is asked to close a step
// whose checklist section (cfg.ChecklistHeading) carries an item not ticked
// with "[x]"/"[X]". It travels inside a *RefusalError naming the step file
// and the item's line; a checklist with no items, or no checklist heading
// at all, is never refused this way. It is conform.ErrOpenChecklistItem.
var ErrOpenChecklistItem = conform.ErrOpenChecklistItem

// ErrUnmetDependency is returned when Finish is asked to close a step whose
// frontmatter declares a depends-on id that is not a done step. It travels
// inside a *RefusalError naming the step file being finished (Line 0), with
// copy distinguishing a dependency that exists but is not done from one
// that names no step file at all. A done step is never refused this way.
var ErrUnmetDependency = errors.New("step depends on a step that is not finished")

// ErrMissingStateHeading is returned when a replacement state body given to
// Finish carries no section for one of cfg.StateHeadings.Ordered()'s four
// required headings. The check is presence-only, in any order; an empty
// section is valid. It is conform.ErrMissingStateHeading.
var ErrMissingStateHeading = conform.ErrMissingStateHeading

// ErrPartialWrite marks a write-path error returned after at least one of
// Finish's, NewFeature's, or NewStep's own writes already landed on disk.
var ErrPartialWrite = errors.New("partial write")

// StateSource is the RefusalError.Path placeholder a refusal carries when
// it concerns the bytes of Finish's state argument rather than a file
// Finish opened itself, since Finish never learns their real source; a
// caller that does, such as cli's --state flag, replaces the placeholder
// before rendering the refusal.
const StateSource = "<state>"

// HandoffSource is StateSource's counterpart for Finish's handoff argument.
const HandoffSource = "<handoff>"

// RefusalError reports a refusal that changed nothing on disk: the path it
// concerns, what was wrong with it, and how to fix it. Line is 0 when the
// refusal names a whole file or path rather than one line inside it.
type RefusalError struct {
	Path    string
	Line    int
	Problem string
	Fix     string
	Err     error
}

// Error renders "<path>: <problem>; <fix>", or "<path>:<line>: <problem>;
// <fix>" when Line is set.
func (e *RefusalError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s; %s", e.Path, e.Line, e.Problem, e.Fix)
	}

	return fmt.Sprintf("%s: %s; %s", e.Path, e.Problem, e.Fix)
}

// Unwrap exposes Err so errors.Is reaches the sentinel this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}

// partialWriteError marks err as ErrPartialWrite without changing what
// Error() reports, so a caller's rendered message is unaffected by whether
// a write landed before this error was returned.
type partialWriteError struct {
	err error
}

func (e *partialWriteError) Error() string   { return e.err.Error() }
func (e *partialWriteError) Unwrap() []error { return []error{e.err, ErrPartialWrite} }

// markPartial wraps err with ErrPartialWrite, reporting that at least one
// write already landed before err was produced. It returns nil unchanged.
func markPartial(err error) error {
	if err == nil {
		return nil
	}

	return &partialWriteError{err: err}
}
