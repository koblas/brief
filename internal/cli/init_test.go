package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_reports_created_then_unchanged pins R2/R3/R11's happy path: a
// fresh repository's first "brief init --host none" reports both artifacts
// created, with the feature root's row carrying a trailing "/", and the
// "installed" next-action line on stderr; a second, identical run reports
// both "unchanged" and the "already installed" next-action line, exit 0.
func Test_init_reports_created_then_unchanged(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "unchanged .brief.yaml\nunchanged docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
}

// Test_init_keeps_a_valid_existing_config_and_reports_it pins R6's "edited
// locally" branch at the CLI boundary: a config that decodes without
// violation but differs from the shipped render is reported "kept", and
// the feature root it names — not Default()'s — is what gets created.
func Test_init_keeps_a_valid_existing_config_and_reports_it(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\ncreated specs/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())
}

// Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched pins
// R3's refusal branch: exit 1, a stderr line naming the "(no files
// changed)" promise, and the working directory carrying exactly the one
// file that was already there.
func Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: [unterminated\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "(no files changed)")
	assert.Contains(t, stderr.String(), "brief init --force")

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
}

// Test_the_directory_probe_sees_new_entries_on_a_successful_init is the
// control arm for the refusal test above: the identical os.ReadDir probe,
// against a fresh repository instead of the refusing fixture, does grow a
// second entry — proving the probe is capable of catching a write, not
// merely one that happens to see none.
func Test_the_directory_probe_sees_new_entries_on_a_successful_init(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Len(t, entries, 2)
}

// Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix
// pins the STATE.md open debt this scenario closes at the CLI boundary:
// the stderr line names the offending key, its value, and 'brief init
// --force' as the fix.
func Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "handoff-cap-lines is 0")
	assert.Contains(t, stderr.String(), "brief init --force")
}

// Test_init_force_rewrites_an_existing_config_from_defaults pins --force's
// own report shape: "created", detail "rewritten from defaults", against
// the same fixture the refusal tests above refuse on.
func Test_init_force_rewrites_an_existing_config_from_defaults(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml (rewritten from defaults)\ncreated docs/specifications/\n", stdout.String())
}

// Test_init_dry_run_prints_the_plan_and_writes_nothing pins R9: the same
// rows a real run would print, the dry-run stderr line, and an unchanged
// working directory.
func Test_init_dry_run_prints_the_plan_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_refuses_an_unknown_host pins R8's usage-error branch: exit 2,
// naming the given value and the accepted list. "bogus" is used rather
// than "claude-code" — S03 does not accept it yet.
func Test_init_refuses_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "bogus"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: none; run 'brief init --host none'`+"\n", stderr.String())
}

// Test_init_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument init takes none of.
func Test_init_refuses_a_stray_positional_argument(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "extra"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}
