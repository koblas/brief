// Package markdown extracts sections and titles from a markdown body
// without parsing it to an AST: every extraction is a line scan against
// known heading text. A line inside a fenced code block (``` or ~~~) is
// never treated as a heading, so a fenced example containing "#" comment
// lines does not truncate a section early.
//
// Section returns a section's body under a heading. Title returns a
// body's first level-1 heading. UnterminatedFence detects a fence CommonMark
// never closes. CountLines counts a body's lines. FirstUnchecked returns
// the first unticked checklist item in a section, and CountChecklistItems
// counts every item in one, ticked or not. Entries lists a section's
// column-0 list items ("- ", "* ", "N. "), with continuation folding.
// HeadingLine returns a heading's own 1-based line number. All of these
// are fence-aware, like Section.
package markdown
