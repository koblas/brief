// Package doctor reports whether a repository's setup for brief is
// healthy: its ".brief.yaml" (present, parses, every value valid, and
// whether it shadows an ancestor config), its feature root (exists, is a
// directory, is readable and writable), and its host environment (a
// ".git" found by walking up from the working directory, and whether the
// "brief" binary on PATH matches the one currently running). It never
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
// filesystem (os.Getwd) never sees that case.
//
// doctor imports internal/platform/config and the standard library only.
// Its own environment seams — WithLookPath, WithExecutable,
// WithBinaryVersion, WithVersion — let a caller substitute the PATH
// lookup, the running binary's own path, and its version for a test; the
// filesystem itself is read directly, with no Store port, since every
// check here is a local read (and, for root-dir's own writability probe,
// a create-and-remove) rather than a persisted resource.
package doctor
