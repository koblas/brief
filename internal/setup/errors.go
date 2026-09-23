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

// ErrAgentsNeedHost is returned when InitRequest.WithAgents is set but the
// resolved Host is not HostClaudeCode: the three role agents only ever
// install under a claude-code plugin. Like ErrUnknownHost it is a bare
// sentinel, not a *RefusalError — an invocation defect cli reports as a
// usage error rather than as a write refusal.
var ErrAgentsNeedHost = errors.New("--with-agents requires --host claude-code")

// ErrEditAgentsNeedHost is returned when InitRequest.EditAgents is set but
// the resolved Host is not HostClaudeCode: --edit-agents only ever edits a
// claude-code-bound agent file. Like ErrAgentsNeedHost it is a bare
// sentinel, not a *RefusalError — an invocation defect cli reports as a
// usage error rather than as a write refusal.
var ErrEditAgentsNeedHost = errors.New("--edit-agents requires --host claude-code")

// ErrNotADirectory is returned when the configured feature root already
// exists as something other than a directory. It travels inside a
// *RefusalError naming that path.
var ErrNotADirectory = errors.New("not a directory")

// ErrUnwritable is returned when R10's pre-write check (checkWritable)
// finds a target whose nearest existing ancestor is not a directory, or is
// a directory that cannot be written to. It travels inside a *RefusalError
// naming that ancestor; unlike every other refusal, Init still returns a
// populated Result (Artifacts and Print) alongside it, so a caller can
// render the --print output the refusal's own Fix points at.
var ErrUnwritable = errors.New("unwritable target")

// ErrPartialWrite marks a write-path error returned after at least one of
// Init's own writes already landed on disk — distinct from one returned
// before any of them did. cli's files_changed (R3) reads this through
// errors.Is rather than assuming every write command's own failure always
// changed nothing.
var ErrPartialWrite = errors.New("partial write")

// ErrConcurrentEdit is returned when CLAUDE.md's own bytes, re-read
// immediately before Init or Uninstall writes to it, no longer match the
// bytes planning read — another process (a concurrent brief invocation, or
// a person editing the file by hand) changed it in between. It travels
// inside a *RefusalError naming the CLAUDE.md path; no write to it is ever
// attempted once this fires.
var ErrConcurrentEdit = errors.New("changed since it was planned")

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
