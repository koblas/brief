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

// CompileHandoff validates suffix against step and returns the compiled
// form, refusing with ErrInvalidHandoffSuffix when suffix:
//
//   - is empty;
//   - contains a path separator ("/" or "\"), which would name a
//     subdirectory instead of a flat file beside the step file;
//   - contains "%", which would make fmt.Sprintf-based rendering ambiguous;
//   - contains a digit — suffix "1.md" against step pattern "STEP-%d.md"
//     would render step 1's handoff as "STEP-11.md", which
//     Pattern.Number recognizes as step 11: a handoff file a directory
//     scan would misread as a step file;
//   - equals step's filename extension, which would make the rendered
//     handoff filename identical to the step filename itself for a step
//     pattern with no other literal suffix, writing the handoff body over
//     the step file.
func CompileHandoff(step Pattern, suffix string) (HandoffPattern, error) {
	if suffix == "" {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.ContainsAny(suffix, `/\`) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.Contains(suffix, "%") {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.ContainsFunc(suffix, func(r rune) bool { return r >= '0' && r <= '9' }) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if suffix == step.suffix {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	return HandoffPattern{step: step, suffix: suffix}, nil
}
