// Package stepfile owns a feature's step-file naming convention and its
// step files' machine fields. Naming works both directions: turning a step
// number into a filename (Name, ID) and recognizing a directory entry as a
// step file (Number). Machine fields — id, status, depends-on — are
// parsed from a step file's YAML frontmatter (ParseFrontmatter) rather
// than inferred from prose, per R3. scaffold writes step files through
// this package; start, status, next, check and finish all read them back
// through it, so it lives under platform rather than under any one
// feature package.
package stepfile
