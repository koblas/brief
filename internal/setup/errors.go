package setup

import (
	"errors"
	"fmt"
)

// ErrUnknownHost is returned when InitRequest.Host names a value Hosts does not list.
var ErrUnknownHost = errors.New("unknown host")

// ErrAgentsNeedHost is returned when InitRequest.WithAgents is set but the resolved Host is not HostClaudeCode.
var ErrAgentsNeedHost = errors.New("--with-agents requires --host claude-code")

// ErrEditAgentsNeedHost is returned when InitRequest.EditAgents is set but the resolved Host is not HostClaudeCode.
var ErrEditAgentsNeedHost = errors.New("--edit-agents requires --host claude-code")

// ErrNotADirectory is returned when the configured feature root exists as something other than a directory.
var ErrNotADirectory = errors.New("not a directory")

// ErrUnwritable is returned when checkWritable finds a target whose nearest existing ancestor cannot be written to.
var ErrUnwritable = errors.New("unwritable target")

// ErrPartialWrite marks a write-path error returned after at least one write already landed on disk.
var ErrPartialWrite = errors.New("partial write")

// ErrConcurrentEdit is returned when a file's bytes, re-read immediately before a write, no longer match what planning read.
var ErrConcurrentEdit = errors.New("changed since it was planned")

// RefusalError reports a refusal that changed nothing on disk: the path it
// concerns, what was wrong with it, and how to fix it.
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

// partialWriteError marks err as ErrPartialWrite without changing what Error() reports.
type partialWriteError struct {
	err error
}

func (e *partialWriteError) Error() string   { return e.err.Error() }
func (e *partialWriteError) Unwrap() []error { return []error{e.err, ErrPartialWrite} }

// markPartial wraps err with ErrPartialWrite. It returns nil unchanged.
func markPartial(err error) error {
	if err == nil {
		return nil
	}

	return &partialWriteError{err: err}
}
