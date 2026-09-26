// Package assemble reads a feature directory and computes the brief an
// implementer needs to start its next step: the next open step's id,
// title, acceptance criteria and checklist, plus the decisions and
// constraints inherited from the feature's state file. It also computes,
// across every feature, the summary "brief status" prints: steps done over
// total, the next open step, and how many steps are blocked on an
// unfinished dependency.
//
// Start, Status and Features are the entry points. Start refuses a
// feature that does not exist with ErrNoSuchFeature, and refuses with a
// *RefusalError wrapping ErrMalformedFeature a feature whose structure it
// cannot assemble around rather than return a brief that silently omits or
// misreports part of it — see ErrMalformedFeature and RefusalError for what
// that covers. An absent or whitespace-only acceptance section in the
// briefed step, a checklist heading present with zero items, or an absent
// heading in the state file, does not refuse: Start reports each as a
// Shortfall in Brief.Shortfalls, in that order, and still returns a Brief
// carrying every other section.
//
// Status takes the opposite stance on the same failure: a feature
// directory or step file that cannot be read or parsed becomes that row's
// Problem rather than failing the whole call, because one malformed
// feature must not blind "brief status" to every other feature in the
// repository.
//
// Features lists the names of a repository's known feature directories.
//
// Check reports every fault in a feature's on-disk layout that
// scaffold.Finish would now refuse to write over, so a tree that predates
// a cap or a rule is still surfaced rather than silently grandfathered in.
// Every predicate Check shares with Finish lives in
// internal/platform/conform, since assemble and scaffold cannot import
// each other.
//
// assemble imports internal/platform/config, internal/platform/stepfile,
// internal/platform/markdown and internal/platform/conform, and the
// standard library only. It never imports internal/scaffold or
// internal/cli: assemble owns reading a feature, scaffold owns writing
// one, and the two packages never import each other.
package assemble
