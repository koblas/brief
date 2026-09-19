package atomicfile

import (
	"io/fs"
	"os"
)

// WriteFile writes data to name under root atomically, mirroring
// os.Root.WriteFile's signature and semantics but replacing name through a
// temporary sibling and a rename so a concurrent reader never observes a
// partially-written file.
//
// It is Create plus a single Write plus Close, and it is the right choice
// whenever the caller already holds the complete contents: there is no
// window in which a forgotten or early Close could publish a truncated
// file, and no error path that leaves the temp sibling behind. Reach for
// Create only to stream contents the caller does not have in one piece.
//
// Create documents the shared details: the existing file's permission bits
// win over perm on a replace, a stale temp sibling is overwritten rather
// than treated as a collision, and there is no fsync — this claims
// atomicity, not durability.
func WriteFile(root *os.Root, name string, data []byte, perm fs.FileMode) error {
	w, err := newPendingFile(root, name, perm)
	if err != nil {
		return err
	}

	if _, writeErr := w.Write(data); writeErr != nil {
		// Close removes the temp sibling and returns this same error;
		// returning it directly keeps the failure the caller sees identical
		// to the one Write reported.
		_ = w.Close()

		return writeErr
	}

	return w.Close()
}
