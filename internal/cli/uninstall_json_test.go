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

// Test_uninstall_json_is_one_exact_document pins uninstallDocument's exact
// shape and key order for the happy path: removed names the config's own
// absolute path, created and modified are both "[]" — uninstall never
// writes — and artifacts carries the one "removed" row.
func Test_uninstall_json_is_one_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":false,` +
		`"created":[],"modified":[],"removed":[` + jsonString(t, configPath) + `],"artifacts":[` +
		`{"kind":"config","path":` + jsonString(t, configPath) + `,"action":"removed","detail":null}]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_uninstall_json_nothing_installed_is_an_empty_document pins the
// zero-artifact shape: artifacts "[]", every path slice "[]", and no
// stderr line at all — R11's "nothing installed" text-mode line has no
// JSON equivalent to write.
func Test_uninstall_json_nothing_installed_is_an_empty_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":false,` +
		`"created":[],"modified":[],"removed":[],"artifacts":[]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_uninstall_json_dry_run_reports_empty_removed pins R9's JSON shape:
// dry_run true, removed "[]" even though the plan's own artifact row says
// "removed", and nothing written to disk.
func Test_uninstall_json_dry_run_reports_empty_removed(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":true,` +
		`"created":[],"modified":[],"removed":[],"artifacts":[` +
		`{"kind":"config","path":` + jsonString(t, configPath) + `,"action":"removed","detail":null}]}` + "\n"

	assert.Equal(t, want, stdout.String())

	_, statErr := os.Stat(configPath)
	assert.NoError(t, statErr)
}

// Test_uninstall_json_failure_reports_files_changed_false pins
// writesFilesAnnotation on the uninstall leaf: an os.Remove failure (an
// unwritable parent directory) is a generic failure document whose
// files_changed is non-null false, since nothing was actually removed
// before it. Skipped under root, which ignores directory write permission.
func Test_uninstall_json_failure_reports_files_changed_false(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(wd, 0o555))
	t.Cleanup(func() { _ = os.Chmod(wd, 0o755) })

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "uninstall")

	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.False(t, *decoded.FilesChanged)
}
