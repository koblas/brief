package scaffold

import (
	"os"

	"github.com/koblas/brief/internal/platform/atomicfile"
)

// replaceBytes atomically replaces name under root with data.
//
// It returns atomicfile's error unwrapped on purpose: Finish's writeFailure
// wraps it once for that boundary and adds the retry hint, and wrapping
// here as well would put "scaffold:" in the message twice.
func replaceBytes(root *os.Root, name string, data []byte) error {
	w, err := atomicfile.Create(root, name, 0o644)
	if err != nil {
		return err //nolint:wrapcheck // the caller owns this boundary
	}

	_, _ = w.Write(data)

	return w.Close() //nolint:wrapcheck // the caller owns this boundary
}

// replaceString atomically replaces name under root with data, writing it
// through io.StringWriter so the caller's string is not copied into a
// []byte first.
//
// It returns atomicfile's error unwrapped on purpose, for the same reason
// as replaceBytes: the caller owns the one place this boundary is wrapped.
func replaceString(root *os.Root, name, data string) error {
	w, err := atomicfile.Create(root, name, 0o644)
	if err != nil {
		return err //nolint:wrapcheck // the caller owns this boundary
	}

	_, _ = w.WriteString(data)

	return w.Close() //nolint:wrapcheck // the caller owns this boundary
}
