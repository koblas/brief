// Package stepfile owns a feature's step-file naming convention, the
// handoff file named beside each step file (CompileHandoff), and step
// files' machine fields. Naming works both directions: turning a step
// number into a filename (Name, ID) and recognizing a directory entry as a
// step file (Number). Machine fields — id, status, depends-on — are
// parsed from a step file's YAML frontmatter (ParseFrontmatter) rather
// than inferred from prose, per R3. Marking a step done edits the same
// frontmatter textually (SetStatus), never by decoding and re-marshaling
// it, since Frontmatter has no KnownFields and a round trip would drop
// any key it does not model. scaffold writes step files through this
// package; start, status, next, check and finish all read them back
// through it, so it lives under platform rather than under any one
// feature package.
//
// DependencyIndex holds the one rule for whether a step's declared
// depends-on ids are met: direct dependencies only, keyed by
// pattern.ID(n), never fm.ID, and a done step is never blocked by its own
// dependencies. assemble's status command builds one per feature to count
// blocked steps, and scaffold's finish builds one per feature to refuse
// closing a step whose dependency is unmet or names no step file — the
// rule lives here, rather than in either feature package, because
// scaffold and assemble are siblings that cannot import each other.
package stepfile
