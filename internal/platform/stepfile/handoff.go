package stepfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidHandoffSuffix is returned by CompileHandoff when a
// handoff-file-suffix configuration value cannot be used to name a step's
// handoff file.
var ErrInvalidHandoffSuffix = errors.New("invalid handoff-file suffix")

// HandoffPattern is a compiled handoff-file-suffix: a step's Pattern paired
// with the suffix appended to Pattern.ID to name that step's handoff file.
type HandoffPattern struct {
	step   Pattern
	suffix string
}

// Name renders the handoff filename for step number n: step.ID(n) + suffix.
func (h HandoffPattern) Name(n int) string {
	return h.step.ID(n) + h.suffix
}

// collisionProbeNumbers are the step numbers CompileHandoff renders and
// compares when checking for a name collision: 1 and 2 catch a
// non-varying Pattern.ID, 9, 10, 99, 100 additionally cross a digit-run
// width boundary.
var collisionProbeNumbers = []int{1, 2, 9, 10, 99, 100}

// CompileHandoff validates suffix against step and returns the compiled
// form, refusing with ErrInvalidHandoffSuffix when suffix is empty,
// contains a path separator or "%", contains a digit (which could make a
// handoff name collide with a step filename), or, paired with step,
// renders a handoff name that, compared case-insensitively, equals a step
// filename, stateFile, or specificationFile at any probed step number.
func CompileHandoff(step Pattern, suffix, stateFile, specificationFile string) (HandoffPattern, error) {
	if suffix == "" {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.ContainsAny(suffix, `/\`) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.Contains(suffix, "%") {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	// A digit here could shift a rendered handoff name onto a step number
	// Pattern.Number would recognize as a real step file.
	if strings.ContainsFunc(suffix, func(r rune) bool { return r >= '0' && r <= '9' }) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	// A Pattern.ID that renders the same string for every step number
	// would alias every step's handoff onto one file.
	if step.ID(1) == step.ID(2) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	handoff := HandoffPattern{step: step, suffix: suffix}

	for _, n := range collisionProbeNumbers {
		rendered := handoff.Name(n)

		if strings.EqualFold(rendered, step.Name(n)) ||
			strings.EqualFold(rendered, stateFile) ||
			strings.EqualFold(rendered, specificationFile) {
			return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
		}
	}

	return handoff, nil
}
