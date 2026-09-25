package assemble

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ErrNoSuchFeature is returned when the named feature has no directory.
var ErrNoSuchFeature = errors.New("no such feature")

// ErrMalformedFeature is returned when a feature directory is missing a
// file Start requires, or that file cannot be read or parsed.
var ErrMalformedFeature = errors.New("malformed feature")

// Problem describes why Status could not read a feature directory or one
// of its step files, or why Start refused to assemble a Brief. Path is the
// offending directory or file, Detail is the failure's message, Fix is the
// one-line remedy, and Line is the 1-based line within Path the fault
// points at (0 for the whole file).
type Problem struct {
	Path   string
	Detail string
	Fix    string
	Line   int
}

// RefusalError is the read-side refusal Start returns when it declines to
// assemble a Brief rather than return one that silently omits context. Err
// is the sentinel this refusal wraps for errors.Is. It is the read-side
// counterpart of scaffold.RefusalError, duplicated rather than shared
// because assemble must not import scaffold.
type RefusalError struct {
	Problem

	Err error
}

// Error renders "<path>: <detail>; <fix>", or "<path>:<line>: <detail>;
// <fix>" when Line is set.
func (e *RefusalError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s; %s", e.Path, e.Line, e.Detail, e.Fix)
	}

	return fmt.Sprintf("%s: %s; %s", e.Path, e.Detail, e.Fix)
}

// Unwrap exposes Err so errors.Is reaches the sentinel this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}

// readClassFix is the remedy for a directory or file that could not be opened or read at all.
const readClassFix = "make it readable and re-run"

// stepFrontmatterError wraps a stepfile.ParseFrontmatter failure with the
// step file's name, so errors.Is still reaches the wrapped sentinel.
type stepFrontmatterError struct {
	name string
	err  error
}

// Error renders "assemble: <name>: <err>".
func (e *stepFrontmatterError) Error() string {
	return fmt.Sprintf("assemble: %s: %s", e.name, e.err)
}

// Unwrap exposes err so errors.Is/errors.As reach the sentinel this error
// wraps.
func (e *stepFrontmatterError) Unwrap() error {
	return e.err
}

// newProblem converts err, from opening/reading a feature's directory or
// files or from parsing a step file's frontmatter, into the Problem a
// caller reports.
func newProblem(base string, err error, nameable bool) *Problem {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		// nameable joins the PathError's own relative Path onto base;
		// unset, base already names the offending file directly.
		path := base
		if nameable {
			path = filepath.Join(base, pathErr.Path)
		}

		return &Problem{Path: path, Detail: pathErr.Err.Error(), Fix: readClassFix}
	}

	if fmErr, ok := errors.AsType[*stepFrontmatterError](err); ok {
		return &Problem{
			Path:   filepath.Join(base, fmErr.name),
			Detail: "frontmatter does not parse: " + fmErr.err.Error(),
			Fix:    fmt.Sprintf("run 'brief check %s' to list every fault", filepath.Base(base)),
		}
	}

	// Every other error falls here: no caller currently passes anything
	// but a *fs.PathError or a *stepFrontmatterError.
	detail := strings.TrimPrefix(err.Error(), "assemble: ")

	return &Problem{
		Path:   base,
		Detail: detail,
		Fix:    fmt.Sprintf("fix its frontmatter, or run 'brief new step %s' to scaffold a conforming step file", filepath.Base(base)),
	}
}
