// Package atomicfile writes a file's full contents through a temporary
// sibling and a rename, so a reader never observes a truncated or
// partially-written file. It makes no durability claim: there is no fsync,
// so a write is atomic with respect to concurrent readers but not
// guaranteed to survive a power loss.
package atomicfile
