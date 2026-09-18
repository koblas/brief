// Package markdown extracts sections and titles from a markdown body
// without parsing it to an AST: every extraction is a line scan against
// known heading text, so the byte-identity guarantees the write path
// depends on elsewhere in brief are never at risk here. A line inside a
// fenced code block (``` or ~~~) is never treated as a heading, so a
// fenced example containing "#" comment lines — this repository's own
// acceptance criteria are exactly that — does not truncate a section
// early.
package markdown
