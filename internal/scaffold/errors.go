package scaffold

import (
	"errors"
	"fmt"
)

// ErrInvalidFeatureName is returned when NewFeature is asked to create a
// name that would break the whitespace-separated four-field status
// contract: an empty name, or one containing a rune unicode.IsSpace
// reports true for — the same predicate strings.Fields splits that
// contract's fields on. It is a bare sentinel, not a *RefusalError: an
// invalid name is an invocation defect, reported by cli as a usage error
// rather than as a write refusal.
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
// Finish opens a fenced code block it never closes: assemble.Start finds
// every configured heading by scanning forward for a terminator, so an
// open fence there would leave every heading after it unreadable.
var ErrUnterminatedFence = errors.New("input has an unterminated fence")

// ErrAlreadyFinished is returned when Finish is asked to close a step whose
// frontmatter already says done, but the supplied handoff or state differs
// from what is recorded on disk. It travels inside a *RefusalError naming
// the specific divergent file — the recorded handoff file or the state
// file — so the caller can read the recorded bytes and compare rather than
// having their new input silently discarded or the record silently
// replaced. A done step whose handoff file is missing or unreadable is
// exempt from this refusal: with no recorded handoff there is nothing to
// diverge from, and Finish is the only path to a done step, so refusing
// would leave a crash-then-hand-edit tree, or a tree migrated before
// handoff files existed, with no way forward.
var ErrAlreadyFinished = errors.New("step already finished with different inputs")

// ErrOverCap is returned when a body Finish is asked to write measures more
// lines, by markdown.CountLines, than its configured cap
// (cfg.HandoffCapLines for the handoff argument, cfg.StateCapLines for the
// state argument — SCENARIO-17/18 share this one sentinel). RefusalError.Path
// names which body: HandoffSource or StateSource.
var ErrOverCap = errors.New("input is over the configured line cap")

// StateSource is the RefusalError.Path placeholder a refusal carries when
// it concerns the bytes of Finish's state argument rather than a file
// Finish opened itself. Finish never learns where those bytes came from —
// a file, or standard input — so a caller that does know, such as cli's
// --state flag, is expected to replace the placeholder with the real
// source before rendering the refusal.
const StateSource = "<state>"

// HandoffSource is StateSource's counterpart for Finish's handoff argument.
const HandoffSource = "<handoff>"

// RefusalError reports a refusal that changed nothing on disk: the path
// it concerns, what was wrong with it, and how to fix it. cli renders
// these fields into R14a's one-line refusal template, appending ":Line"
// to the path when Line is set. Line is 0 when the refusal names a whole
// file or path rather than one line inside it.
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
