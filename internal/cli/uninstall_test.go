package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_uninstall_removes_the_config_init_wrote pins R6/R11's happy path
// through cli.Run: after "init", "uninstall" removes the config it wrote,
// stdout carries exactly one "removed" row, stderr names R11's third line,
// and exit is nil (0).
func Test_uninstall_removes_the_config_init_wrote(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's install; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_keeps_an_edited_config_and_reports_it pins R11's stderr
// line 4: a config that was never brief's own render is "kept", and
// stderr names --force as the way to remove it.
func Test_uninstall_keeps_an_edited_config_and_reports_it(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: nothing removed; run 'brief uninstall --force' to remove edited files\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: specs\n", string(body))
}

// Test_uninstall_force_removes_an_edited_config pins --force's own
// override at the CLI boundary.
func Test_uninstall_force_removes_an_edited_config(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--force"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's install; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_with_nothing_installed_reports_it pins R11's zero-artifact
// branch: empty stdout, "nothing installed" on stderr, exit nil.
func Test_uninstall_with_nothing_installed_reports_it(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed\n", stderr.String())
}

// Test_uninstall_dry_run_prints_the_plan_and_removes_nothing pins R9's own
// dry-run promise: the row says "removed", the dry-run stderr line, and
// the file survives byte-identical.
func Test_uninstall_dry_run_prints_the_plan_and_removes_nothing(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.NoError(t, statErr)
}

// Test_uninstall_refuses_an_unknown_host pins R8's usage-error branch,
// mirrored from init: exit 2, naming the given value and the accepted
// list.
func Test_uninstall_refuses_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "bogus"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief uninstall: unknown host "bogus"; expected one of: none; run 'brief uninstall --host none'`+"\n", stderr.String())
}

// Test_uninstall_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument uninstall takes none of.
func Test_uninstall_refuses_a_stray_positional_argument(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "extra"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}
