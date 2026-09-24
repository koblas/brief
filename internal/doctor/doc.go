// Package doctor reports whether a repository's setup for brief is
// healthy: its ".brief.yaml" (present, parses, every value valid, and
// whether it shadows an ancestor config), its feature root (exists, is a
// directory, is readable and writable), its host environment (a ".git"
// found by walking up from the working directory, and whether the "brief"
// binary on PATH matches the one currently running), and its Claude Code
// integration (the skills-directory plugin, its hook, the brief-workflow
// skill, the CLAUDE.md instruction block, the three role agents, whether
// every bound role resolves to an agent file, and whether the bound
// planner and implementer preload the brief-workflow skill). It never
// reads a feature's own contents — that is internal/assemble's Check, the
// backstop for "brief check" — doctor answers only "is brief set up to
// run here at all", the question "brief doctor" exists to answer before
// "brief check" ever runs.
//
// Diagnose is the one entry point: it returns a Report, one Check per
// question, in a fixed order, and never refuses — every fault, including
// an invalid or unparseable ".brief.yaml" or a missing feature root, is
// reported as a Check, so a broken setup is diagnosable rather than
// merely rejected. Only a working directory that does not exist at all
// keeps doctor from running; a caller resolving one from the real
// filesystem (os.Getwd) never sees that case. The six host-integration
// rows, roles and roles-skill check the install root Diagnose's own config
// lookup resolves — its directory whenever a ".brief.yaml" was found
// there, parseable or not, else wd — against a Claude Code host with no
// detection of its own.
//
// doctor imports internal/platform/{agentfile,config,host,artifact,repo}
// and the standard library only, never internal/setup. Its own environment
// seams — WithLookPath, WithExecutable, WithBinaryVersion, WithVersion,
// WithHomeDir — let a caller substitute the PATH lookup, the running
// binary's own path, its version, and the home directory a bare role
// binding resolves against, for a test.
//
// A narrow set of reads is strictly OS-subject and stays that way: the
// feature root's own existence, directory-ness, readability and
// writability (root-dir, including its create-and-remove writable.Probe),
// env-path's binary identity check (os.SameFile, filepath.EvalSymlinks)
// and version read (debug/buildinfo.ReadFile), and the home directory
// lookup (os.UserHomeDir). One further slice is OS-subject only in its
// error shape, not its classification logic: an ancestor path component
// that is itself a regular file reports ENOTDIR on a real filesystem, the
// one shape classifyProbeError's own ENOTDIR arm exists to recognize, but
// an equivalent fstest.MapFS fixture reports plain fs.ErrNotExist for the
// same shape instead — so that arm, and every chmod-driven
// permission-denied case alongside it, is tested against real disk even
// though the production code path it exercises is fs.FS-generic. None of
// these can be expressed against an arbitrary fs.FS without either losing
// a real permission error's own shape or fabricating one a real
// filesystem would never produce.
//
// Every other read in this package — the config family (config-file,
// config-parse, config-values, config-shadow), env-git, and the six
// host-integration rows' own probes (probeIntegrationFile,
// scanSnippetCandidateStates, blockingDir, and (*Server).projectTree for
// roles' own project-scope tree) — runs through one fs.FS root,
// (*Server).rootFS, defaulting to os.DirFS("/"). config.LocateWithinFS,
// config.InspectFS and repo.RootFS each map an absolute path onto that
// root internally (their own fsName); host.go keeps an identical copy of
// that mapping for the reads it makes directly (fs.Lstat, fs.ReadFile,
// fs.Sub), the same duplication internal/platform/config and
// internal/platform/repo already carry between each other rather than
// inverting the dependency for an eight-line helper. Rule 5's own
// user-scope role resolution (roles, roles-skill) runs through
// agentfile.ResolveBindingIn over an agentfile.Tree — projectTree(root)
// (fs.Sub of rootFS(), Dir set to root) for the project side,
// (*Server).userTree() (DirTree(homeDir()) by default) for the user side —
// so a test can substitute an in-memory Tree, or the whole root fs.FS,
// without touching disk. Both seams are exposed to this package's own
// tests only, via export_test.go (WithRootFS, WithHomeTree); production
// always builds the OS adapter.
package doctor
