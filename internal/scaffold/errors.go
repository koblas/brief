package scaffold

import (
	"errors"
	"fmt"
)

// ErrNoSuchFeature is returned when the named feature has no directory
// under the configured feature directory.
var ErrNoSuchFeature = errors.New("no such feature")

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

// ErrUnterminatedFence is returned when a handoff given to Finish opens a
// fenced code block it never closes: spliced into the step file as-is, an
// open fence would swallow every section after the handoff anchor on the
// very next Finish that re-parses it, since SectionRange would never find
// the fence's close and would scan to end of file.
var ErrUnterminatedFence = errors.New("handoff has an unterminated fence")

// RefusalError reports a refusal that changed nothing on disk: the path
// it concerns, what was wrong with it, and how to fix it. cli renders
// these three fields into R14a's one-line refusal template.
type RefusalError struct {
	Path    string
	Problem string
	Fix     string
	Err     error
}

// Error renders "<path>: <problem>; <fix>".
func (e *RefusalError) Error() string {
	return fmt.Sprintf("%s: %s; %s", e.Path, e.Problem, e.Fix)
}

// Unwrap exposes Err so errors.Is reaches the sentinel this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}
