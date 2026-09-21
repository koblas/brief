// Package conform holds the four body-shaped predicates that a body
// destined for a feature's on-disk layout either conforms to or does not: a
// body over its configured line cap, a body that opens a fenced code block
// it never closes, a state body missing one of its required sections, and a
// step's checklist section carrying an item not ticked.
//
// internal/scaffold's write path (Finish) refuses on these predicates
// before a byte reaches disk; internal/assemble's read path (Check) reports
// the same predicates as findings against a tree the write path never
// validated. scaffold and assemble cannot import each other, so this
// package is the one place each predicate — and the problem/fix copy it
// carries — is defined; scaffold renders a Violation into its own
// *RefusalError and assemble renders one into its own Finding, but neither
// package decides on its own whether one fires.
package conform
