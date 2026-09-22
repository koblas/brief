package config

import (
	"strings"

	"github.com/koblas/brief/internal/platform/stepfile"
)

// headingField pairs one heading's dotted configuration key with its
// decoded value, for violations' incremental pairwise-distinct check.
type headingField struct {
	key   string
	value string
}

// violations reports every value in cfg that fails an R1 rule, as
// *ValueError, in Config's own field-declaration order — doctor's
// config-values check renders one row per element, and Resolve reports
// only its first element. It returns nil when every rule is satisfied.
// feature-directory carries no rule here: it is a path, and Default()
// ships one containing a separator. The handoff-file-suffix rule needs a
// successfully compiled step pattern; when step-file-pattern itself fails,
// that rule is skipped rather than run against a zero-value pattern, so a
// bad pattern never cascades into a second, bogus row.
func violations(cfg Config) []*ValueError {
	var errs []*ValueError

	step, stepErr := validateStepPattern(cfg.StepFilePattern)
	if stepErr != nil {
		errs = append(errs, stepErr)
	}

	if err := validateFileName("specification-file", cfg.SpecificationFile); err != nil {
		errs = append(errs, err)
	}

	if err := validateFileName("state-file", cfg.StateFile); err != nil {
		errs = append(errs, err)
	}

	if strings.EqualFold(cfg.StateFile, cfg.SpecificationFile) {
		errs = append(errs, &ValueError{Key: "state-file", Value: cfg.StateFile, Reason: "must differ from specification-file"})
	}

	var headings []headingField

	if err := validateHeading(&headings, "progress-heading", cfg.ProgressHeading); err != nil {
		errs = append(errs, err)
	}

	if err := validateHeading(&headings, "checklist-heading", cfg.ChecklistHeading); err != nil {
		errs = append(errs, err)
	}

	if stepErr == nil {
		if err := validateHandoffSuffix(step, cfg.HandoffFileSuffix, cfg.StateFile, cfg.SpecificationFile); err != nil {
			errs = append(errs, err)
		}
	}

	if err := validateHeading(&headings, "acceptance-heading", cfg.AcceptanceHeading); err != nil {
		errs = append(errs, err)
	}

	for _, h := range []headingField{
		{key: "state-headings.binding-decisions", value: cfg.StateHeadings.BindingDecisions},
		{key: "state-headings.left-unbuilt", value: cfg.StateHeadings.LeftUnbuilt},
		{key: "state-headings.traps", value: cfg.StateHeadings.Traps},
		{key: "state-headings.open-debts", value: cfg.StateHeadings.OpenDebts},
	} {
		if err := validateHeading(&headings, h.key, h.value); err != nil {
			errs = append(errs, err)
		}
	}

	if err := validateCap("handoff-cap-lines", cfg.HandoffCapLines); err != nil {
		errs = append(errs, err)
	}

	if err := validateCap("state-cap-lines", cfg.StateCapLines); err != nil {
		errs = append(errs, err)
	}

	if err := validateCap("default-output-budget-bytes", cfg.DefaultOutputBudgetBytes); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// validateStepPattern compiles pattern with stepfile.Compile and returns
// a *ValueError wrapping the stepfile failure when it cannot be used to
// name or recognize step files.
func validateStepPattern(pattern string) (stepfile.Pattern, *ValueError) {
	step, err := stepfile.Compile(pattern)
	if err != nil {
		return stepfile.Pattern{}, &ValueError{
			Key:    "step-file-pattern",
			Value:  pattern,
			Reason: "must be a plain file name with exactly one %d or %0Nd verb and no other %",
			Err:    err,
		}
	}

	return step, nil
}

// validateHandoffSuffix compiles suffix against step with
// stepfile.CompileHandoff and returns a *ValueError wrapping the stepfile
// failure when suffix cannot name step's handoff file without colliding
// with the step, state or specification files.
func validateHandoffSuffix(step stepfile.Pattern, suffix, stateFile, specificationFile string) *ValueError {
	if _, err := stepfile.CompileHandoff(step, suffix, stateFile, specificationFile); err != nil {
		return &ValueError{
			Key:   "handoff-file-suffix",
			Value: suffix,
			Reason: "must be a file-name suffix with no path separator, digit or %, naming a file distinct " +
				"from the step, state and specification files",
			Err: err,
		}
	}

	return nil
}

// validateFileName refuses value for key unless it is a plain file name:
// not empty, not "." or "..", and containing no path separator ("/" or
// "\").
func validateFileName(key, value string) *ValueError {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return &ValueError{Key: key, Value: value, Reason: "must be a plain file name with no path separator"}
	}

	return nil
}

// validateHeading refuses value for key when it is blank after
// strings.TrimSpace, or when it exactly matches a heading already seen —
// naming the earlier key the collision is against. On success it appends
// {key, value} to seen so a later heading can be checked against it.
func validateHeading(seen *[]headingField, key, value string) *ValueError {
	if strings.TrimSpace(value) == "" {
		return &ValueError{Key: key, Value: value, Reason: "must not be empty"}
	}

	for _, h := range *seen {
		if h.value == value {
			return &ValueError{Key: key, Value: value, Reason: "must differ from " + h.key}
		}
	}

	*seen = append(*seen, headingField{key: key, value: value})

	return nil
}

// validateCap refuses value for key unless it is at least 1.
func validateCap(key string, value int) *ValueError {
	if value < 1 {
		return &ValueError{Key: key, Value: value, Reason: "must be at least 1"}
	}

	return nil
}
