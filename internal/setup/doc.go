// Package setup owns brief's own install write path: Init plans and then
// applies the ".brief.yaml" config file, the feature root a repository
// needs before brief's other commands work in it, and, for
// HostClaudeCode, that host's own skills-directory plugin files
// (internal/platform/host.Host.Plugin); Uninstall plans and applies
// removing everything Init installed. Both plan every artifact and decide
// every refusal before the first byte is written or removed, so a DryRun
// request computes exactly what a real run would and a refusal never
// leaves a partial tree behind on its own account.
//
// Init never resolves configuration the way every other command does
// (internal/platform/config.Resolve, via internal/cli's resolveRoot): a
// repository's own ".brief.yaml" may be invalid, and --force must still be
// able to rewrite it from defaults, so Init uses Locate and Inspect
// directly and classifies what it finds itself. Every plugin file, and
// Uninstall's own removal of the config, is recognized digest-only
// (internal/platform/artifact.Recognize) and never decoded, so none of
// them has a refusal class of its own — an unparseable, R1-invalid, or
// locally edited file is simply "edited locally", the same as any other
// byte mismatch, kept unless --force. --force rewrites only the config
// from defaults; it never rewrites an edited plugin file, only removes one
// under Uninstall.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong, and
// how to fix it, wrapping ErrUnknownHost or an
// internal/platform/config.InvalidConfigError. A write failure after at
// least one artifact already landed is wrapped in ErrPartialWrite instead,
// distinguishing it from a refusal that changed nothing on disk. Init's
// own artifact list carries the config first, the feature root, then the
// plugin's own files in host.Host.Plugin's write order; Uninstall's own
// list carries the plugin's files in the reverse of that order, then the
// config file last, so a partial uninstall never removes the repository's
// opt-in marker while something else still stands. The feature root and
// everything under it is never an Uninstall artifact at all, and is never
// removed; nor is ".claude/" or ".claude/skills/" above the plugin's own
// directory — brief owns only host.PluginDir and below (R6).
//
// setup writes through the real filesystem — internal/platform/atomicfile
// for the config file's and every plugin file's own byte-identical
// replace, os.MkdirAll for the feature root and a plugin file's parent
// directories, os.Remove for Uninstall's own file and now-empty-directory
// removals (never RemoveAll) — imports only internal/platform/config,
// internal/platform/artifact, internal/platform/atomicfile and
// internal/platform/host alongside the standard library, and never
// internal/scaffold or internal/doctor: those own the write and read paths
// over a feature's own content, a question setup never asks.
package setup
