// Package repo locates the git repository enclosing a directory.
//
// Root is the one walk-up every caller that needs a repository boundary
// shares: doctor's own env-git check, and config.LocateInRepo, which bounds
// init/uninstall/doctor's own install-root discovery to it so an ancestor
// ".brief.yaml" outside the repository the working directory is inside is
// never adopted.
package repo
