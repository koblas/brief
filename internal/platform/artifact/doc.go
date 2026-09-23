// Package artifact renders the files brief writes into a repository it is
// installed into, and recognizes an existing file's bytes against the set
// of digests the binary ships. A file's bytes equal to a render this
// package produced — today's, or an earlier release's — is "brief-written
// and unedited"; any other bytes are "edited locally" (R6). Every render is
// deterministic: no version, no date, no map order, so a digest actually
// proves something. The fixed renders — plugin manifest, hooks, skills and
// role agents, plus earlier releases' renders under older/ — are embedded
// verbatim from the files/ directory; only the config and the CLAUDE.md
// snippet are generated in Go.
//
// internal/setup consumes this package to decide whether an existing file
// — the config, a Claude Code plugin file (PluginManifest, SkillStart,
// SkillFinish, ClaudeHooks), or the brief-workflow skill (SkillWorkflow) —
// can be kept, written or must refuse; internal/doctor will consume it the
// same way for host-integration rows.
// Neither consumer's logic — what to do with an origin — belongs here:
// this package only renders bytes and classifies bytes, never a
// filesystem path or a command's own decision.
//
// The CLAUDE.md instruction block (R5) is the one parameterized artifact:
// SnippetBlock(dir) and RecognizeSnippet are its own render and recognize
// functions, never Render(KindSnippet) or Recognize(KindSnippet, ...) —
// Render's signature carries no room for the feature directory a snippet
// needs, and a single compiled-in digest list has no way to check "current
// for which directory". RecognizeSnippet is config-independent: it matches
// a block brief-written for any feature directory, not only the caller's
// currently configured one, and reports which directory that was.
// ScanSnippetMarkers locates a marker block's own span inside an arbitrary
// file's bytes, or reports the first marker-ordering defect it finds — the
// one scanner internal/setup and internal/doctor both call, so a lone or
// misordered marker is reported identically by init/uninstall and by
// doctor.
package artifact
