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
