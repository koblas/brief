// Package repo locates the git repository enclosing a directory.
//
// Root walks up from a directory to find the nearest enclosing ".git"
// entry (a directory, or a file naming a linked worktree's real gitdir)
// and returns the directory that holds it. RootFS is its filesystem-
// agnostic core, taking an fs.FS and an absolute path so tests can run
// against a fstest.MapFS instead of the real disk.
package repo
