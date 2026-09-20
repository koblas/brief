// Package assemble reads a feature directory and computes the brief an
// implementer needs to start its next step: the next open step's id,
// title, acceptance criteria and checklist, plus the decisions and
// constraints inherited from the feature's state file. It also computes,
// across every feature, the summary "brief status" prints: steps done over
// total, the next open step, and how many steps are blocked on an
// unfinished dependency.
//
// Start and Status are the entry points. Start refuses a feature that does
// not exist with ErrNoSuchFeature. It refuses, with a *RefusalError
// wrapping ErrMalformedFeature, a feature whose structure it cannot
// assemble around rather than return a brief that silently omits or
// misreports part of it: a specification that is missing, unreadable,
// carries an unterminated fenced code block, or has no configured progress
// heading; a state file that is missing, unreadable, or carries an
// unterminated fence; or a briefed step whose frontmatter carries no "id:"
// or whose checklist heading is absent. A step file whose frontmatter is
// absent or does not parse also refuses, wrapping whatever sentinel
// readSteps produced, including stepfile.ErrNoFrontmatter. An absent
// acceptance heading in the briefed step, or an absent heading in the
// state file, does not refuse: Start reports each as a Shortfall in
// Brief.Shortfalls and still returns a Brief carrying every other section.
// A Shortfall fires on a heading's absence, never on an empty body — a
// heading present with nothing under it stays conforming. A step's
// frontmatter is parsed before any markdown
// extraction runs, so a "#" character inside a YAML value is never mistaken
// for a heading.
//
// Status takes the opposite stance on the same failure: a feature
// directory or step file that cannot be read or parsed becomes that row's
// Problem rather than failing the whole call, because one malformed
// feature must not blind "brief status" to every other feature in the
// repository. Start's refusal and Status's degradation both read the same
// step files through readSteps, which stays intolerant of a parse failure
// either way — the difference is which caller turns that failure into a
// refusal (Start) and which caller catches it and marks a row (Status).
//
// assemble imports internal/platform/config, internal/platform/stepfile
// and internal/platform/markdown, and the standard library only. It never
// imports internal/scaffold or internal/cli: assemble owns reading a
// feature, scaffold owns writing one, and the two packages never import
// each other.
package assemble
