// Package atomicfile replaces a file's contents through a temporary sibling
// and a rename, so a reader never observes a truncated or partially-written
// file. WriteFile takes contents the caller already holds; Create returns an
// io.WriteCloser for contents that have to be streamed, committing the
// replacement when Close renames.
//
// It makes no durability claim: there is no fsync, so a write is atomic with
// respect to concurrent readers but not guaranteed to survive a power loss.
package atomicfile
