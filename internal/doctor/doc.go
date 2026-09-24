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
// Two families of reads are strictly OS-subject and stay that way: the
// feature root's own existence, directory-ness, readability and
// writability (root-dir, including its create-and-remove writable.Probe),
// and the six host-integration files' own permission-error classification
// (classifyProbeError), env-path's binary identity check (os.SameFile,
// filepath.EvalSymlinks) and version read (debug/buildinfo.ReadFile), and
// the home directory lookup (os.UserHomeDir) — none of these can be
// expressed against an fs.FS without either losing a real permission
// error's own shape or fabricating one a real filesystem would never
// produce.
//
// The config family (config-file, config-parse, config-values,
// config-shadow) and env-git instead run through an fs.FS root
// (*Server).rootFS, defaulting to os.DirFS("/") and passed straight to
// config.LocateWithinFS, config.InspectFS and repo.RootFS — each already
// maps an absolute path onto its own root FS internally (fsName), so
// doctor calls only those exported *FS entry points and keeps no mapping
// of its own. Rule 5's own user-scope role resolution (roles,
// roles-skill) runs through agentfile.ResolveBindingIn over an
// agentfile.Tree — DirTree(root) for the project side, (*Server).userTree()
// (DirTree(homeDir()) by default) for the user side — so a test can
// substitute an in-memory Tree without touching disk. Both seams are
// exposed to this package's own tests only, via export_test.go
// (WithRootFS, WithHomeTree); production always builds the OS adapter.
package doctor
