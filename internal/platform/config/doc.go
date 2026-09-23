// Package config resolves brief's configuration: the shipped default profile,
// and the repository's own ".brief.yaml" when one is present.
//
// Every feature package (new, start, finish, status, check) consumes a
// Config; this package holds no feature knowledge of its own, only the
// schema and the resolution rule. Locate walks upward from a start
// directory to the filesystem root and finds the nearest ".brief.yaml",
// naming every farther ancestor's own config file as shadowed — a
// repository with no config file at all is not an error, it is the shipped
// profile in effect. The walk has an fs.FS-backed core, LocateWithinFS: a
// root FS (production: os.DirFS("/"), a test: a fstest.MapFS holding just
// the ancestors in play) rather than os.Stat per directory, with
// LocateWithin/Locate/LocateInRepo as the thin OS adapters that own
// filepath.Abs's own cwd-dependent resolution — the core never sees a
// relative path. Inspect decodes a found file onto Default(), so an
// omitted key keeps its shipped value, an unknown key is refused rather
// than silently ignored, and reports every decoded value that fails its own
// rule (a cap is at least 1, a heading is non-empty and distinct from the
// others, a file name carries no path separator, and so on) alongside the
// decoded Config; Inspect reads its own single, already-located path
// through os.Open directly rather than the root FS — it is not part of the
// walk, and has no other caller that would benefit from an fs.FS-backed
// twin. Resolve is Locate plus Inspect's first violation: the one rule set
// and the one walk-up every caller shares, so a command refusing an invalid
// value and a tool reporting every value that fails (doctor) can never
// drift onto two different checks.
package config
