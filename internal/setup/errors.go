package setup

import (
	"errors"
	"fmt"
)

// ErrUnknownHost is returned when InitRequest.Host names a value Hosts does
// not list. It is a bare sentinel, not a *RefusalError: an unknown host is
// an invocation defect, reported by cli as a usage error rather than as a
// write refusal.
var ErrUnknownHost = errors.New("unknown host")

// ErrNotADirectory is returned when the configured feature root already
// exists as something other than a directory. It travels inside a
// *RefusalError naming that path.
var ErrNotADirectory = errors.New("not a directory")

// ErrPartialWrite marks a write-path error returned after at least one of
// Init's own writes already landed on disk — distinct from one returned
// before any of them did. cli's files_changed (R3) reads this through
// errors.Is rather than assuming every write command's own failure always
// changed nothing.
var ErrPartialWrite = errors.New("partial write")

// RefusalError reports a refusal that changed nothing on disk: the path it
// concerns, what was wrong with it, and how to fix it. cli renders these
// fields into R14a's one-line refusal template.
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

// Unwrap exposes Err so errors.Is/errors.As reach the sentinel or typed
// error this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}

// partialWriteError marks err as ErrPartialWrite without changing what
// Error() reports: Go's multi-error Unwrap lets errors.Is reach both err's
// own chain and ErrPartialWrite, while Error() renders exactly what err
// alone would have.
type partialWriteError struct {
	err error
}

func (e *partialWriteError) Error() string   { return e.err.Error() }
func (e *partialWriteError) Unwrap() []error { return []error{e.err, ErrPartialWrite} }

// markPartial wraps err with ErrPartialWrite, reporting that at least one
// artifact already landed before err was produced. It returns nil
// unchanged.
func markPartial(err error) error {
	if err == nil {
		return nil
	}

	return &partialWriteError{err: err}
}
