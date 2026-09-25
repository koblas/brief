// Package artifact renders the files brief writes into a repository it is
// installed into, and recognizes an existing file's bytes against the set
// of digests the binary ships. A file's bytes equal to a render this
// package produced — today's, or an earlier release's — is "brief-written
// and unedited"; any other bytes are "edited locally". Every render is
// deterministic: no version, no date, no map order, so a digest actually
// proves something. The fixed renders — plugin manifest, hooks, skills and
// role agents, plus earlier releases' renders under older/ — are embedded
// verbatim from the files/ directory; only the config and the CLAUDE.md
// snippet are generated in Go.
//
// internal/setup consumes this package to decide whether an existing file
// can be kept, written or must refuse; internal/doctor consumes it the
// same way for host-integration rows. Neither consumer's decision logic
// belongs here: this package only renders and classifies bytes, never a
// filesystem path or a command's decision.
//
// The CLAUDE.md instruction block is the one parameterized artifact:
// SnippetBlock(dir) and RecognizeSnippet are its own render and recognize
// functions, never Render(KindSnippet) or Recognize(KindSnippet, ...),
// since a snippet needs a feature directory Render's signature has no room
// for. RecognizeSnippet matches a block brief-written for any feature
// directory, not only the caller's configured one, and reports which
// directory that was. ScanSnippetMarkers locates a marker block's span
// inside arbitrary file bytes, or reports the first marker-ordering
// defect — the one scanner internal/setup and internal/doctor both call.
package artifact
