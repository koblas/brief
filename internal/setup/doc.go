// Package setup owns brief's own install write path: Init plans and then
// applies the ".brief.yaml" config file, the feature root a repository
// needs before brief's other commands work in it, and, for
// HostClaudeCode, that host's own skills-directory plugin files
// (internal/platform/host.Host.Plugin) and the CLAUDE.md instruction block
// (R5); Uninstall plans and applies removing everything Init installed.
// Both plan every artifact and decide every refusal before the first byte
// is written or removed, so a DryRun request computes exactly what a real
// run would and a refusal never leaves a partial tree behind on its own
// account.
//
// The CLAUDE.md block lives at whichever of internal/platform/host's own
// InstructionFiles candidates already holds a recognized block, else the
// first that exists, else root CLAUDE.md is created — scanSnippetCandidates
// and chooseSnippetLocation own that choice, shared by Init's planSnippet
// and Uninstall's planSnippetRemoval. A marker defect (a lone or
// out-of-order begin/end, two blocks in one file, a block in both
// candidates) or a CRLF chosen candidate refuses, naming the file and, for
// a marker defect, its 1-based line; CRLF is checked only against the one
// candidate a caller resolved to act on, never blanket across both, so an
// untouched CLAUDE.md elsewhere never blocks an install or removal aimed at
// the other one. mergeSnippet and removeSnippet are exact inverses: the
// separator mergeSnippet encodes around an appended block (one newline
// before it when the file already ended in one, a blank line before it
// when it did not, no newline after it in that second case) is exactly
// what removeSnippet strips back out, so a round trip through Init then
// Uninstall reproduces byte-identical bytes with no sidecar recording which
// shape was used. An emptied CLAUDE.md is deleted on Uninstall — the only
// "brief created it" signal this package keeps, so a pre-existing,
// already-empty CLAUDE.md is deleted too, not restored.
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
// plugin's own files in host.Host.Plugin's write order, then the CLAUDE.md
// block last; applying instead writes the feature root, the plugin files,
// the CLAUDE.md block, then the config file last, so the config — the
// repository's opt-in marker — never lands before everything else has.
// Uninstall's own list carries the CLAUDE.md block first, then the
// plugin's files in the reverse of Init's write order, then the config
// file last, and applies in that same order, so a partial uninstall never
// removes the opt-in marker while something else still stands. The
// feature root and everything under it is never an Uninstall artifact at
// all, and is never removed; nor is ".claude/" or ".claude/skills/" above
// the plugin's own directory — brief owns only host.PluginDir and below,
// and the CLAUDE.md candidates InstructionFiles names (R6).
//
// setup writes through the real filesystem — internal/platform/atomicfile
// for the config file's, every plugin file's and CLAUDE.md's own
// byte-identical replace, os.MkdirAll for the feature root and a plugin
// file's parent directories, os.Remove for Uninstall's own file and
// now-empty-directory removals (never RemoveAll) — imports only
// internal/platform/config, internal/platform/artifact,
// internal/platform/atomicfile and internal/platform/host alongside the
// standard library, and never internal/scaffold or internal/doctor: those
// own the write and read paths over a feature's own content, a question
// setup never asks.
package setup
