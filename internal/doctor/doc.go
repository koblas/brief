// Package doctor reports whether a repository's setup for brief is
// healthy: its ".brief.yaml" (present, parses, every value valid, and
// whether it shadows an ancestor config), its feature root (exists, is a
// directory, is readable and writable), its host environment (a ".git"
// found by walking up from the working directory, and whether the "brief"
// binary on PATH matches the one currently running), and its Claude Code
// integration (the skills-directory plugin, its hook, the CLAUDE.md
// instruction block, the three role agents, and whether every bound role
// resolves to an agent file). It never reads a feature's own contents —
// that is internal/assemble's Check, the backstop for "brief check" —
// doctor answers only "is brief set up to run here at all", the question
// "brief doctor" exists to answer before "brief check" ever runs.
//
// Diagnose is the one entry point: it returns a Report, one Check per
// question, in a fixed order, and never refuses — every fault, including
// an invalid or unparseable ".brief.yaml" or a missing feature root, is
// reported as a Check, so a broken setup is diagnosable rather than
// merely rejected. Only a working directory that does not exist at all
// keeps doctor from running; a caller resolving one from the real
// filesystem (os.Getwd) never sees that case. The five host-integration
// rows and roles check the install root config.Locate resolves — its
// directory whenever a ".brief.yaml" was found there, parseable or not,
// else wd — against a Claude Code host with no detection of its own.
//
// doctor imports internal/platform/{config,host,artifact} and the
// standard library only, never internal/setup. Its own environment
// seams — WithLookPath, WithExecutable, WithBinaryVersion, WithVersion,
// WithHomeDir — let a caller substitute the PATH lookup, the running
// binary's own path, its version, and the home directory a bare role
// binding resolves against, for a test; the filesystem itself is read
// directly, with no Store port, since every check here is a local read
// (and, for root-dir's own writability probe, a create-and-remove) rather
// than a persisted resource.
package doctor
