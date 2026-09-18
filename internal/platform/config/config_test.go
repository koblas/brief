package config_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
)

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
