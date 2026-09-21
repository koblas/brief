// Package markdown extracts sections and titles from a markdown body
// without parsing it to an AST: every extraction is a line scan against
// known heading text, so the byte-identity guarantees the write path
// depends on elsewhere in brief are never at risk here. A line inside a
// fenced code block (``` or ~~~) is never treated as a heading, so a
// fenced example containing "#" comment lines — this repository's own
// acceptance criteria are exactly that — does not truncate a section
// early.
//
// Section is the only exported way to read a section's body; there is no
// exported way to recover its byte offsets, because offsets exist only to
// splice content into an existing document, and every write in brief is a
// whole-file write instead (R21). Title returns the text of a body's first
// level-1 heading, fence-aware like Section. UnterminatedFence is the sole
// fence-open detector, used both to validate a write argument before it
// lands and to refuse an on-disk file whose configured headings a
// terminator scan could not read past the open fence. CountLines is the
// one line counter a length cap is measured against, on both the write
// side (scaffold.Finish) and the read side (check), so the two never
// disagree about what a "line" is. FirstUnchecked is the one
// checklist-item scanner, fence-aware like Section, so an unticked item is
// found the same way whether the caller is writing (scaffold.Finish) or
// reading (check).
package markdown
