package scaffold

import (
	"errors"
	"io/fs"
)

// peelWriteErr returns err's own Err field when err is an *fs.PathError —
// the shape every rwfs.FS write method (Mkdir, WriteFile, CreateExclusive)
// wraps every failure in, naming the operation and the path the caller
// already has. Peeling it here means a caller that adds its own context —
// writeExclusive's "write <name>: ", replaceBytes' and replaceString's
// bare pass-through — reproduces the exact text scaffold always produced
// against *os.Root, rather than stacking rwfs's own "<op> <name>: " prefix
// on top of it.
//
// It is deliberately not applied to every write-side call site: the two
// cases rwfs classifies without preserving the underlying OS error —
// fs.ValidPath's rejection of a traversing name, and OpenRoot's
// classification of "exists but is not a directory" as syscall.ENOTDIR —
// would peel down to a bare "invalid argument" or "not a directory" with
// no name in it at all, which is worse than the *fs.PathError's own
// Op/Path framing. Those two call sites keep rwfs's wrapping instead.
func peelWriteErr(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}

	return err
}

// peelReadErr returns err's immediate cause when err carries rwfs's own
// read-side wrap (fmt.Errorf("rwfs: <op> <name>: %w", cause), used by
// every rwfs.FS read method and rwfs.OpenOS), so a caller that adds its
// own "scaffold: ..." context does not stack a second "rwfs:" segment onto
// a message it already names. It returns err unchanged when err carries no
// wrapped cause.
func peelReadErr(err error) error {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapped
	}

	return err
}
