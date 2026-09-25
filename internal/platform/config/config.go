package config

import "errors"

// ErrInvalidConfig marks a ".brief.yaml" that could not be used as
// configuration; callers branch on it with errors.Is.
var ErrInvalidConfig = errors.New("invalid brief config")

// StateHeadings holds the heading text for each of the four required
// sections in a feature's state file. Named fields, not a slice or map, so
// a repository can retitle a section but never drop one.
type StateHeadings struct {
	BindingDecisions string `yaml:"binding-decisions"`
	LeftUnbuilt      string `yaml:"left-unbuilt"`
	Traps            string `yaml:"traps"`
	OpenDebts        string `yaml:"open-debts"`
}

// Ordered returns the four heading texts in the order the state file
// requires them: binding decisions, left unbuilt, traps, open debts.
func (h StateHeadings) Ordered() []string {
	return []string{h.BindingDecisions, h.LeftUnbuilt, h.Traps, h.OpenDebts}
}

// RoleBindings names the agent bound to each of brief's three positions:
// planner, implementer, and reviewer. An empty field means that position is
// unbound.
type RoleBindings struct {
	Planner     string `yaml:"planner"`
	Implementer string `yaml:"implementer"`
	Reviewer    string `yaml:"reviewer"`
}

// Config is brief's resolved configuration: feature directory layout, file
// names and headings, the caps enforced at the write path, which optional
// conventions a repository opts into, and the role bindings. Every field
// has a shipped default (Default); a repository's ".brief.yaml" overrides
// only the keys it sets.
type Config struct {
	FeatureDirectory         string        `yaml:"feature-directory"`
	StepFilePattern          string        `yaml:"step-file-pattern"`
	SpecificationFile        string        `yaml:"specification-file"`
	StateFile                string        `yaml:"state-file"`
	ProgressHeading          string        `yaml:"progress-heading"`
	ChecklistHeading         string        `yaml:"checklist-heading"`
	HandoffFileSuffix        string        `yaml:"handoff-file-suffix"`
	AcceptanceHeading        string        `yaml:"acceptance-heading"`
	StateHeadings            StateHeadings `yaml:"state-headings"`
	HandoffCapLines          int           `yaml:"handoff-cap-lines"`
	StateCapLines            int           `yaml:"state-cap-lines"`
	DefaultOutputBudgetBytes int           `yaml:"default-output-budget-bytes"`
	OptionalConventions      []string      `yaml:"optional-conventions"`
	Roles                    RoleBindings  `yaml:"roles"`
}

// Default returns the shipped profile: the configuration in effect for a
// repository with no ".brief.yaml", and the base every found config file is
// overlaid onto.
func Default() Config {
	return Config{
		FeatureDirectory:  "docs/specifications",
		StepFilePattern:   "SCENARIO-%02d.md",
		SpecificationFile: "specification.md",
		StateFile:         "STATE.md",
		ProgressHeading:   "## BDD Acceptance Progress",
		ChecklistHeading:  "## Implementation Plan",
		HandoffFileSuffix: "-HANDOFF.md",
		AcceptanceHeading: "## Scenario",
		StateHeadings: StateHeadings{
			BindingDecisions: "## Binding decisions",
			LeftUnbuilt:      "## Left unbuilt",
			Traps:            "## Traps",
			OpenDebts:        "## Open debts",
		},
		HandoffCapLines:          60,
		StateCapLines:            80,
		DefaultOutputBudgetBytes: 8192,
		OptionalConventions:      nil,
		Roles:                    RoleBindings{},
	}
}
