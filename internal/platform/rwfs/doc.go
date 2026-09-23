// Package rwfs is a read-write filesystem port: FS composes the standard
// library's own read-side interfaces (fs.FS, fs.ReadFileFS, fs.ReadDirFS,
// fs.StatFS, fs.ReadLinkFS) with the minimal write operations brief's
// production code performs — Mkdir, MkdirAll, WriteFile, CreateExclusive and
// Remove.
//
// OS is the production adapter, confined to a directory tree by an
// *os.Root; WriteFile goes through internal/platform/atomicfile so a
// concurrent reader never observes a partial write. Mem is an in-memory
// adapter backed by testing/fstest.MapFS for tests that want the same
// contract without touching disk. Both adapters pass the same contract
// test, defined once in rwfs_test.go and run against each.
//
// A handful of guarantees are inherent to a real filesystem and are not,
// and cannot economically be, reproduced by Mem:
//
//   - Confinement: os.Root refuses a name, or a symbolic link, that would
//     resolve outside its root. Mem has no notion of "outside" — every name
//     is just a map key, so nothing stops a symlink entry seeded with an
//     escaping target from reporting one.
//   - Permission enforcement: the OS adapter's operations fail with a
//     permission error when the underlying file or directory forbids them.
//     Mem records the permission bits given to Mkdir/MkdirAll/WriteFile
//     verbatim but never consults them — every operation is allowed
//     regardless of the mode on record.
//   - The process umask: a fresh create on the OS adapter is masked by the
//     umask, the same as any os.OpenFile call; a replace is not (see
//     internal/platform/atomicfile). Mem never applies a umask — a create's
//     perm argument lands exactly as given.
//   - Atomicity: WriteFile's replace-without-partial-write guarantee is
//     specific to that method. CreateExclusive writes directly to name on
//     the OS adapter rather than through a temp sibling, so a crash between
//     open and the write completing can leave a concurrent reader observing
//     a truncated file. Mem's CreateExclusive has no partial-write case at
//     all: its write is one in-memory assignment under m's own mutex.
package rwfs
