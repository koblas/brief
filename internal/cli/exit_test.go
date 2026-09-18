package cli_test

import (
	"errors"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
)

// errFixtureUsage and errFixtureOther are static fixtures standing in for
// "some usage error" and "some other error" — their text is not asserted
// anywhere, only their classification.
var (
	errFixtureUsage = errors.New("brief: no command given; expected one of: new, start")
	errFixtureOther = errors.New("boom")
)

func Test_reports_success_when_there_is_no_error(t *testing.T) {
	assert.Equal(t, 0, cli.ExitCode(nil))
}

func Test_reports_a_usage_failure_for_a_usage_error(t *testing.T) {
	wrapped := errors.Join(errFixtureUsage, cli.ErrUsage)

	assert.Equal(t, 2, cli.ExitCode(wrapped))
}

func Test_reports_a_validation_failure_for_any_other_error(t *testing.T) {
	assert.Equal(t, 1, cli.ExitCode(errFixtureOther))
}
