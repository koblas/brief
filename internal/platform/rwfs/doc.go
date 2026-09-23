// Package rwfs is a read-write filesystem port: FS composes the standard
// library's own read-side interfaces (fs.FS, fs.ReadFileFS, fs.ReadDirFS,
// fs.StatFS, fs.ReadLinkFS) with the minimal write operations brief's
// production code performs — Mkdir, MkdirAll, WriteFile, CreateExclusive,
// Remove and OpenRoot, the last for confining a caller to one subtree of a
// larger FS — plus Close, for releasing what OpenRoot acquired.
//
// OS is the production adapter, confined to a directory tree by an
// *os.Root; WriteFile goes through internal/platform/atomicfile so a
// concurrent reader never observes a partial write, and OpenRoot wraps
// os.Root.OpenRoot, so a nested root inherits the same confinement and the
// same symlink-following on every path segment. Mem is an in-memory adapter
// backed by testing/fstest.MapFS for tests that want the same contract
// without touching disk; its OpenRoot follows a symlink at name itself the
// way OS's OpenRoot does — a symlink in one of name's ancestor segments
// instead reports syscall.ENOTDIR, unlike OS — and returns a second Mem
// sharing the parent's underlying map, mutex and fake clock rather than a
// copy, so a write through either is visible from the other and Close is a
// no-op. Both adapters pass the same contract test, defined once in
// rwfs_test.go and run against each.
//
// A handful of guarantees are inherent to a real filesystem and are not,
// and cannot economically be, reproduced by Mem:
//
//   - Confinement: os.Root refuses a name, or a symbolic link, that would
//     resolve outside its root — including one reached through a nested
//     OpenRoot. Mem has no notion of "outside": its own OpenRoot follows a
//     symlink at name to wherever its target names, inside or outside the
//     subtree the view addresses, and nothing stops one seeded with an
//     escaping target from reporting a location that would be refused on a
//     real filesystem.
//   - Symlink-following through an ancestor segment: os.Root resolves a
//     symlink anywhere in a path, not only its final segment. Mem's own
//     ancestor check (notDirAncestor) only Lstats each segment, so a
//     symlink standing in for a directory partway through name reports
//     syscall.ENOTDIR on Mem where OS would follow it — this divergence has
//     no test of its own; brief's only call site resolves a single segment.
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
//   - ModTime granularity: the OS adapter's entries carry whatever mtime the
//     real filesystem assigns, at whatever resolution it offers. Mem instead
//     advances a monotonic counter on every write — including a WriteFile
//     that rewrites identical bytes — so two Mem.Snapshot calls can always
//     tell "wrote again" from "never wrote"; the shared contract makes no
//     assertion on either adapter's actual ModTime value.
package rwfs
