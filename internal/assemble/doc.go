// Package assemble reads a feature directory and computes the brief an
// implementer needs to start its next step: the next open step's id,
// title, acceptance criteria and checklist, plus the decisions and
// constraints inherited from the feature's state file.
//
// Start is the only entry point. It refuses a feature that does not exist
// (ErrNoSuchFeature) or that has no state file (ErrMalformedFeature)
// rather than assembling a brief that silently omits inherited context —
// R10's rule that a malformed feature is refused, not degraded into. A
// step's frontmatter is parsed before any markdown extraction runs, so a
// "#" character inside a YAML value is never mistaken for a heading.
//
// assemble imports internal/platform/config, internal/platform/stepfile
// and internal/platform/markdown, and the standard library only. It never
// imports internal/scaffold or internal/cli: assemble owns reading a
// feature, scaffold owns writing one, and the two packages never import
// each other.
package assemble
