package assemble

import "errors"

// ErrNoSuchFeature is returned when the named feature has no directory
// under the configured feature directory.
var ErrNoSuchFeature = errors.New("no such feature")

// ErrMalformedFeature is returned when a feature directory exists but is
// missing a file Start requires, such as its state file, or that file
// cannot be read as intended — including a state file whose fenced code
// block never closes, which would otherwise make every configured section
// after the fence opens read as absent.
var ErrMalformedFeature = errors.New("malformed feature")
