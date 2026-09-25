// Package doctor reports whether a repository's setup for brief is
// healthy: its ".brief.yaml", its feature root, its host environment (a
// ".git" found above the working directory, and whether "brief" on PATH
// matches the running binary), and its Claude Code integration (the
// skills-directory plugin, its hook, the brief-workflow skill, the
// CLAUDE.md instruction block, the three role agents, and whether every
// bound role resolves to an agent that preloads brief-workflow).
//
// Diagnose is the one entry point: it returns a Report, one Check per
// question, in a fixed order, and never refuses — every fault is reported
// as a Check rather than rejected. It never reads a feature's own
// contents; that is internal/assemble's job.
//
// doctor imports internal/platform/{agentfile,config,host,artifact,repo}
// and the standard library only, never internal/setup. Its environment
// seams (WithLookPath, WithExecutable, WithBinaryVersion, WithVersion,
// WithHomeDir, WithRootFS, WithHomeTree) let a test substitute the PATH
// lookup, the running binary's path and version, the home directory, and
// the filesystem every probe reads through.
//
// A narrow set of reads stays OS-subject even under a substituted
// filesystem: the feature root's existence, directory-ness, readability
// and writability, env-path's binary identity check and version read, and
// the home directory lookup. These cannot be expressed against an
// arbitrary fs.FS without either losing a real permission error's shape or
// fabricating one a real filesystem would never produce.
package doctor
