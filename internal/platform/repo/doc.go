// Package repo locates the git repository enclosing a directory.
//
// Root is the one walk-up every caller that needs a repository boundary
// shares: doctor's own env-git check, and config.LocateInRepo, which bounds
// init/uninstall/doctor's own install-root discovery to it so an ancestor
// ".brief.yaml" outside the repository the working directory is inside is
// never adopted. RootFS is Root's own core: a root FS (production:
// os.DirFS("/"), a test: a fstest.MapFS holding just the ancestors in
// play) rather than os.Stat per directory, with Root as the thin OS
// adapter that owns filepath.Abs's own cwd-dependent resolution — the core
// never sees a relative path. The fs-name mapping (fsName) is duplicated
// from an identical helper in internal/platform/config rather than
// shared: config already imports repo for LocateInRepo's own boundary, and
// a shared package for an eight-line path mapping would invert that
// dependency.
//
// The production root FS (os.DirFS("/")) is not evaluated on a Windows
// volume path ("C:\..."), and rejects, via fs.ValidPath, any path element
// that is not valid UTF-8 before the underlying stat ever runs: on Linux,
// where the kernel accepts an arbitrary byte string as a file name, a
// ".git" sitting at or below an ancestor directory whose own name holds
// invalid UTF-8 bytes is never seen, so Root falls through to a valid
// ancestor repository, or reports none.
package repo
