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

// ErrUnterminatedFence is returned when a handoff or a replacement state
// body given to Finish opens a fenced code block it never closes. For the
// state body this would leave every configured heading unreadable on the
// next read that scans it for a terminator (assemble.Start among them);
// for the handoff it would leave the step file itself carrying a fence
// CommonMark never closes, malformed for any later reader even though
// spliceHandoff's own fixed point no longer depends on the fence closing.
var ErrUnterminatedFence = errors.New("input has an unterminated fence")

// ErrHandoffNotLast is returned when a step file being finished for the
// first time already has a heading after its handoff anchor. spliceHandoff
// takes everything from the anchor to end of file as the handoff section,
// by contract with the default profile's "## Handoff is the last section
// of every step file" — a heading placed after it by a hand edit, never by
// Finish itself, would otherwise be silently overwritten the moment this
// Finish call lands. The check runs only when the step is not yet done:
// once done, everything after the anchor is Finish's own prior output,
// which may legitimately contain a heading as ordinary handoff prose, and
// R11 requires that content survive an identical re-finish untouched.
var ErrHandoffNotLast = errors.New("handoff anchor is not the last heading in the step file")

// HandoffSource and StateSource are the RefusalError.Path placeholder a
// refusal carries when it concerns the bytes of Finish's handoff or state
// argument rather than a file Finish opened itself. Finish never learns
// where those bytes came from — a file, or standard input — so a caller
// that does know, such as cli's --handoff/--state flags, is expected to
// replace the placeholder with the real source before rendering the
// refusal.
const (
	HandoffSource = "<handoff>"
	StateSource   = "<state>"
)

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
