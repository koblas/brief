// Package atomicfile replaces a file's contents through a temporary sibling
// and a rename, so a reader never observes a truncated or partially-written
// file.
//
// Create is the only entry point. It returns a PendingFile — an
// io.WriteCloser and io.StringWriter that writes to the temporary sibling and
// commits the replacement when Close renames. Close is therefore the call
// that reports whether the replacement landed, and the one whose error a
// caller must check.
//
// It makes no durability claim: there is no fsync, so a write is atomic with
// respect to concurrent readers but not guaranteed to survive a power loss.
package atomicfile
