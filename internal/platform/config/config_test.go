package config_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_a_config_binding_the_reviewer_role_decodes pins the third role
// position (R7 as amended): "roles.reviewer" is a known key that decodes
// onto RoleBindings.Reviewer with no violation, the same as planner and
// implementer.
func Test_a_config_binding_the_reviewer_role_decodes(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `roles:
  reviewer: brief:reviewer
`)

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, "brief:reviewer", cfg.Roles.Reviewer)
}

func Test_Default_carries_the_four_shipped_state_headings(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, []string{
		"## Binding decisions",
		"## Left unbuilt",
		"## Traps",
		"## Open debts",
	}, cfg.StateHeadings.Ordered())
}

func Test_the_default_profile_names_the_specification_file(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "specification.md", cfg.SpecificationFile)
}

func Test_the_default_profile_names_the_state_file(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "STATE.md", cfg.StateFile)
}

func Test_the_default_profile_names_the_checklist_heading(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "## Implementation Plan", cfg.ChecklistHeading)
}

func Test_the_default_profile_names_the_acceptance_heading(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "## Scenario", cfg.AcceptanceHeading)
}

func Test_the_default_profile_names_the_handoff_file_suffix(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "-HANDOFF.md", cfg.HandoffFileSuffix)
}

// Test_the_shipped_profile_passes_validation writes every key explicitly
// set to Default()'s own value: a config file naming every key at its
// shipped value must decode and validate identically to no config file at
// all. init writes a commented config whose effective values are these
// defaults, so R3's "converges" promise relies on this holding.
func Test_the_shipped_profile_passes_validation(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `feature-directory: docs/specifications
step-file-pattern: "SCENARIO-%02d.md"
specification-file: specification.md
state-file: STATE.md
progress-heading: "## BDD Acceptance Progress"
checklist-heading: "## Implementation Plan"
handoff-file-suffix: "-HANDOFF.md"
acceptance-heading: "## Scenario"
state-headings:
  binding-decisions: "## Binding decisions"
  left-unbuilt: "## Left unbuilt"
  traps: "## Traps"
  open-debts: "## Open debts"
handoff-cap-lines: 60
state-cap-lines: 80
default-output-budget-bytes: 8192
`)

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

// Test_Resolve_accepts_a_config_that_sets_every_validated_key_to_a_valid_non_default_value
// is validate's control arm: every rule
// Test_Resolve_refuses_an_invalid_config_value exercises (resolve_test.go)
// has a legitimate, non-default value here that must satisfy it — proving
// the rules refuse only the bad input, not configuration in general.
func Test_Resolve_accepts_a_config_that_sets_every_validated_key_to_a_valid_non_default_value(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `step-file-pattern: "STEP-%03d.md"
specification-file: SPEC.md
state-file: PROGRESS.md
progress-heading: "## Custom Progress"
checklist-heading: "## Custom Checklist"
handoff-file-suffix: "-NOTES.md"
acceptance-heading: "## Custom Scenario"
state-headings:
  binding-decisions: "## Custom Decisions"
  left-unbuilt: "## Custom Left"
  traps: "## Custom Traps"
  open-debts: "## Custom Debts"
handoff-cap-lines: 10
state-cap-lines: 20
default-output-budget-bytes: 4096
`)

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, "STEP-%03d.md", cfg.StepFilePattern)
	assert.Equal(t, "SPEC.md", cfg.SpecificationFile)
	assert.Equal(t, "PROGRESS.md", cfg.StateFile)
	assert.Equal(t, "-NOTES.md", cfg.HandoffFileSuffix)
}
