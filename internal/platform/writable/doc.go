// Package writable probes whether a directory can be written to.
//
// Probe is the one write both internal/setup (checkWritable, R10) and
// internal/doctor (root-dir) ever perform outside their own artifacts:
// setup uses it during Init's pre-write check, doctor uses it as part of
// diagnosing whether the feature root is usable.
package writable
