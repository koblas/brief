package config

import "fmt"

// InvalidConfigError reports a ".brief.yaml" file that could not be used as
// configuration, naming the offending path and the underlying cause.
// errors.Is(err, ErrInvalidConfig) holds for any error wrapping one, and
// errors.As reaches this type to recover Path and Err — the two returns
// fmt.Errorf("%w: %w") cannot both give, since that shape's Unwrap() []error
// leaves errors.Unwrap with nothing to hand back.
type InvalidConfigError struct {
	Path string
	Err  error
}

// Error reproduces the plain-text form callers have relied on: "<path>:
// invalid brief config: <cause>".
func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("%s: %v: %v", e.Path, ErrInvalidConfig, e.Err)
}

// Unwrap exposes both ErrInvalidConfig (for errors.Is) and Err (for
// errors.Is/As against the underlying cause).
func (e *InvalidConfigError) Unwrap() []error {
	return []error{ErrInvalidConfig, e.Err}
}
