// Package markdown extracts sections and titles from a markdown body
// without parsing it to an AST: every extraction is a line scan against
// known heading text, so the byte-identity guarantees the write path
// depends on elsewhere in brief are never at risk here. A line inside a
// fenced code block (``` or ~~~) is never treated as a heading, so a
// fenced example containing "#" comment lines — this repository's own
// acceptance criteria are exactly that — does not truncate a section
// early.
//
// Section and SectionRange serve the read side and the write side of the
// same boundary: Section returns a section's trimmed text for display;
// SectionRange returns the same section's untrimmed byte offsets so a
// caller can splice new content into a body while leaving every other
// byte, including the terminating heading line's own indentation,
// untouched. Section is implemented on top of SectionRange, so the two
// can never disagree about where a section ends.
package markdown
