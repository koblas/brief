package config

import "fmt"

// InvalidConfigError reports a ".brief.yaml" file that could not be used as
// configuration, naming the offending path and the underlying cause: a
// YAML decode error, or a *ValueError when the file parses but a decoded
// value fails its own rule. errors.Is(err, ErrInvalidConfig) holds for any
// error wrapping one, and errors.As reaches this type to recover Path and
// Err — the two returns fmt.Errorf("%w: %w") cannot both give, since that
// shape's Unwrap() []error leaves errors.Unwrap with nothing to hand back.
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

// ValueError reports a single configuration key whose value fails one of
// R1's rules: Key is the value's own dotted YAML key (for example
// "state-headings.traps"), Value is the offending value exactly as
// decoded (a string or an int), and Reason is the rule's own copy, in
// quotes, describing what the value must satisfy instead. Err is non-nil
// only for the step-file-pattern and handoff-file-suffix rules, which
// derive their refusal from stepfile.Compile / stepfile.CompileHandoff —
// wrapping keeps errors.Is(err, stepfile.ErrInvalidPattern) and
// errors.Is(err, stepfile.ErrInvalidHandoffSuffix) reachable through a
// ValueError. decodeConfig carries a ValueError as an
// *InvalidConfigError's own Err.
type ValueError struct {
	Key    string
	Value  any
	Reason string
	Err    error
}

// Error renders "<key> is <value>, <reason>": Value is rendered %q-quoted
// when it is a string, bare otherwise.
func (e *ValueError) Error() string {
	return fmt.Sprintf("%s is %s, %s", e.Key, formatValue(e.Value), e.Reason)
}

// Unwrap exposes Err, when set, so errors.Is/errors.As reach the
// stepfile sentinel a step-file-pattern or handoff-file-suffix refusal
// wraps.
func (e *ValueError) Unwrap() error {
	return e.Err
}

// formatValue renders v the way ValueError.Error's copy requires: %q for
// a string, %v for everything else (an int cap).
func formatValue(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}

	return fmt.Sprintf("%v", v)
}
