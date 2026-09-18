// Package stepfile owns both directions of a feature's step-file naming
// convention: turning a step number into a filename (Name, ID) and
// recognizing a directory entry as a step file (Number). scaffold writes
// step files through this package; start, status, next, check and finish
// will all read them back through it, so it lives under platform rather
// than under any one feature package.
package stepfile
