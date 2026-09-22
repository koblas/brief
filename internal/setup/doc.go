// Package setup owns brief's own install write path: Init plans and then
// applies the ".brief.yaml" config file and the feature root a repository
// needs before brief's other commands work in it. It plans every artifact
// and decides every refusal before the first byte is written, so a
// DryRun request computes exactly what a real run would and a refusal
// never leaves a partial tree behind on its own account.
//
// Init never resolves configuration the way every other command does
// (internal/platform/config.Resolve, via internal/cli's resolveRoot): a
// repository's own ".brief.yaml" may be invalid, and --force must still be
// able to rewrite it from defaults, so Init uses Locate and Inspect
// directly and classifies what it finds itself.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong, and
// how to fix it, wrapping ErrUnknownHost or an
// internal/platform/config.InvalidConfigError. A write failure after at
// least one artifact already landed is wrapped in ErrPartialWrite instead,
// distinguishing it from a refusal that changed nothing on disk.
//
// setup writes through the real filesystem — internal/platform/atomicfile
// for the config file's own byte-identical replace, os.MkdirAll for the
// feature root — imports only internal/platform/config,
// internal/platform/artifact and internal/platform/atomicfile alongside
// the standard library, and never internal/scaffold or internal/doctor:
// those own the write and read paths over a feature's own content, a
// question setup never asks.
package setup
