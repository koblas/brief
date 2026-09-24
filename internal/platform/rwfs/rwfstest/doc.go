// Package rwfstest exercises rwfs.FS's own contract against any adapter,
// not only the two rwfs defines. Contract runs the full read/write
// contract rwfs_test.go used to define once and duplicate per adapter;
// ReadContract runs its read-only half against a narrower, read-only view
// shape such as internal/assemble's own dirFS.
//
// Every documented divergence from the contract — a case an adapter
// deliberately does not, or cannot, satisfy — is declared through an
// Option rather than silently skipped: each Option carries the reason a
// caller passes, and Contract logs every one it was given before running,
// so a reader of `go test -v` output sees exactly which rows were relaxed
// or skipped and why. An adapter passed with no Option must pass every
// row unmodified — that is what proves rwfs.OS and rwfs.Mem share one
// contract with no adapter-specific carve-out.
package rwfstest
