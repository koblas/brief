// Package setup owns brief's own install write path: Init plans and then
// applies the ".brief.yaml" config file and the feature root a repository
// needs before brief's other commands work in it, and Uninstall plans and
// applies removing what Init installed. Both plan every artifact and
// decide every refusal before the first byte is written or removed, so a
// DryRun request computes exactly what a real run would and a refusal
// never leaves a partial tree behind on its own account.
//
// Init never resolves configuration the way every other command does
// (internal/platform/config.Resolve, via internal/cli's resolveRoot): a
// repository's own ".brief.yaml" may be invalid, and --force must still be
// able to rewrite it from defaults, so Init uses Locate and Inspect
// directly and classifies what it finds itself. Uninstall goes further
// still: recognition is digest-only (internal/platform/artifact.Recognize),
// so it never decodes the config at all and has no config-refusal class of
// its own — an unparseable or R1-invalid file is simply "edited locally",
// the same as any other byte mismatch, kept unless --force.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong, and
// how to fix it, wrapping ErrUnknownHost or an
// internal/platform/config.InvalidConfigError. A write failure after at
// least one artifact already landed is wrapped in ErrPartialWrite instead,
// distinguishing it from a refusal that changed nothing on disk.
// Uninstall's own artifact list carries the config file last — a later
// scenario's host artifacts land ahead of it — so a partial uninstall
// never removes the repository's opt-in marker while something else still
// stands; the feature root and everything under it is never an Uninstall
// artifact at all, and is never removed.
//
// setup writes through the real filesystem — internal/platform/atomicfile
// for the config file's own byte-identical replace, os.MkdirAll for the
// feature root, os.Remove for Uninstall's own removals (never RemoveAll) —
// imports only internal/platform/config, internal/platform/artifact and
// internal/platform/atomicfile alongside the standard library, and never
// internal/scaffold or internal/doctor: those own the write and read paths
// over a feature's own content, a question setup never asks.
package setup
