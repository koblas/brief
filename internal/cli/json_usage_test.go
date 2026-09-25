package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonKeys returns doc's own keys, for an exact-key-set assertion via
// assert.ElementsMatch.
func jsonKeys(t *testing.T, doc map[string]json.RawMessage) []string {
	t.Helper()

	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}

	return keys
}

// decodeUsageErrorDocument asserts stdout holds exactly one --json
// usage-error document for wantCommand and wantFilesChanged, and returns
// its error.message and error.fix.
func decodeUsageErrorDocument(t *testing.T, stdout []byte, wantCommand string, wantFilesChanged *bool) (string, string) {
	t.Helper()

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout, &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "error"}, jsonKeys(t, doc))

	var schema int
	require.NoError(t, json.Unmarshal(doc["schema"], &schema))
	assert.Equal(t, 1, schema)

	var command string
	require.NoError(t, json.Unmarshal(doc["command"], &command))
	assert.Equal(t, wantCommand, command)

	var ok bool
	require.NoError(t, json.Unmarshal(doc["ok"], &ok))
	assert.False(t, ok)

	var exitCode int
	require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
	assert.Equal(t, 2, exitCode)

	var errObj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["error"], &errObj))
	assert.ElementsMatch(t, []string{"kind", "message", "path", "line", "problem", "fix", "files_changed"}, jsonKeys(t, errObj))

	var kind string
	require.NoError(t, json.Unmarshal(errObj["kind"], &kind))
	assert.Equal(t, "usage", kind)

	assert.JSONEq(t, "null", string(errObj["path"]))
	assert.JSONEq(t, "null", string(errObj["line"]))
	assert.JSONEq(t, "null", string(errObj["problem"]))

	wantFilesChangedJSON := "null"
	if wantFilesChanged != nil {
		want, err := json.Marshal(*wantFilesChanged)
		require.NoError(t, err)
		wantFilesChangedJSON = string(want)
	}
	assert.JSONEq(t, wantFilesChangedJSON, string(errObj["files_changed"]))

	var message, fix string
	require.NoError(t, json.Unmarshal(errObj["message"], &message))
	require.NoError(t, json.Unmarshal(errObj["fix"], &fix))

	return message, fix
}

func Test_json_mode_renders_a_usage_error_as_one_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status", "--json", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"status","ok":false,"exit_code":2,` +
		`"error":{"kind":"usage",` +
		`"message":"brief status: unknown flag: --bogus; run 'brief status'",` +
		`"path":null,"line":null,"problem":null,` +
		`"fix":"run 'brief status'","files_changed":null}}` + "\n"

	// assert.Equal, not assert.JSONEq: this golden pins byte-exact key order.
	assert.Equal(t, want, stdout.String())
}

// Covers only rows whose text-mode line already carries its own trailing
// "; run '<hint>'" clause; a row with no run hint belongs in the
// fallback table below instead.
func Test_json_mode_usage_error_message_is_the_text_mode_line(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		textArgs     []string
		command      string
		filesChanged *bool
	}{
		{name: "--json --bogus", args: []string{"--json", "--bogus"}, textArgs: []string{"--bogus"}, command: "brief"},
		{name: "--help=x --json", args: []string{"--help=x", "--json"}, textArgs: []string{"--help=x"}, command: "brief"},
		{name: "--version extra --json", args: []string{"--version", "extra", "--json"}, textArgs: []string{"--version", "extra"}, command: "brief"},
		{name: "--version=x --json", args: []string{"--version=x", "--json"}, textArgs: []string{"--version=x"}, command: "brief"},
		{name: "--json --version extra", args: []string{"--json", "--version", "extra"}, textArgs: []string{"--version", "extra"}, command: "brief"},
		{name: "new -x --json", args: []string{"new", "-x", "--json"}, textArgs: []string{"new", "-x"}, command: "new", filesChanged: new(false)},
		{name: "help -x --json", args: []string{"help", "-x", "--json"}, textArgs: []string{"help", "-x"}, command: "help"},
		{name: "status --json --bogus", args: []string{"status", "--json", "--bogus"}, textArgs: []string{"status", "--bogus"}, command: "status"},
		{name: "check --bogus --json", args: []string{"check", "--bogus", "--json"}, textArgs: []string{"check", "--bogus"}, command: "check"},
		{name: "start --bogus --json demo", args: []string{"start", "--bogus", "--json", "demo"}, textArgs: []string{"start", "--bogus", "demo"}, command: "start"},
		{name: "status a --json", args: []string{"status", "a", "--json"}, textArgs: []string{"status", "a"}, command: "status"},
		{name: "start --json", args: []string{"start", "--json"}, textArgs: []string{"start"}, command: "start"},
		{name: "finish --json", args: []string{"finish", "--json"}, textArgs: []string{"finish"}, command: "finish", filesChanged: new(false)},
		{name: "new feature --json", args: []string{"new", "feature", "--json"}, textArgs: []string{"new", "feature"}, command: "new feature", filesChanged: new(false)},
		{name: "new step --json", args: []string{"new", "step", "--json"}, textArgs: []string{"new", "step"}, command: "new step", filesChanged: new(false)},
		{name: "completion --json", args: []string{"completion", "--json"}, textArgs: []string{"completion"}, command: "completion"},
		{name: "status --help=x --json", args: []string{"status", "--help=x", "--json"}, textArgs: []string{"status", "--help=x"}, command: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()

			var textStdout, textStderr bytes.Buffer
			textErr := cli.Run(t.Context(), wd, tt.textArgs, nil, &textStdout, &textStderr)
			require.ErrorIs(t, textErr, cli.ErrUsage)
			wantMessage := strings.TrimRight(textStderr.String(), "\n")

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), tt.command, tt.filesChanged)

			assert.Equal(t, wantMessage, message)
			assert.True(t, strings.HasSuffix(message, "; "+fix))
		})
	}
}

// Covers lines whose "; run '<hint>'" clause is followed by more prose,
// so the trailing quote it usually stops at is absent.
func Test_json_mode_usage_error_fix_stops_at_the_quote_when_the_line_has_trailing_prose(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		textArgs []string
	}{
		{name: "empty name", args: []string{"new", "feature", "", "--json"}, textArgs: []string{"new", "feature", ""}},
		{name: "whitespace name", args: []string{"new", "feature", "a b", "--json"}, textArgs: []string{"new", "feature", "a b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()

			var textStdout, textStderr bytes.Buffer
			textErr := cli.Run(t.Context(), wd, tt.textArgs, nil, &textStdout, &textStderr)
			require.ErrorIs(t, textErr, cli.ErrUsage)
			wantMessage := strings.TrimRight(textStderr.String(), "\n")

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), "new feature", new(false))

			assert.Equal(t, wantMessage, message)
			assert.Equal(t, "run 'brief new feature <name>'", fix)
			assert.Contains(t, message, fix)
		})
	}
}

func Test_json_mode_usage_error_fix_falls_back_when_its_message_has_no_run_hint(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		textArgs     []string
		command      string
		filesChanged *bool
		wantFix      string
	}{
		{name: "--json", args: []string{"--json"}, textArgs: []string{}, command: "brief", wantFix: "run 'brief --help'"},
		{name: "bogus --json", args: []string{"bogus", "--json"}, textArgs: []string{"bogus"}, command: "brief", wantFix: "run 'brief --help'"},
		{name: "new --json", args: []string{"new", "--json"}, textArgs: []string{"new"}, command: "new", filesChanged: new(false), wantFix: "run 'brief new --help'"},
		{name: "new bogus --json", args: []string{"new", "bogus", "--json"}, textArgs: []string{"new", "bogus"}, command: "new", filesChanged: new(false), wantFix: "run 'brief new --help'"},
		{name: "help bogus --json", args: []string{"help", "bogus", "--json"}, textArgs: []string{"help", "bogus"}, command: "help", wantFix: "run 'brief help <command>'"},
		{
			name: "completion nosh --json", args: []string{"completion", "nosh", "--json"},
			textArgs: []string{"completion", "nosh"}, command: "completion",
			wantFix: "run 'brief completion <bash|zsh|fish|powershell>'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()

			var textStdout, textStderr bytes.Buffer
			textErr := cli.Run(t.Context(), wd, tt.textArgs, nil, &textStdout, &textStderr)
			require.ErrorIs(t, textErr, cli.ErrUsage)
			wantMessage := strings.TrimRight(textStderr.String(), "\n")

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), tt.command, tt.filesChanged)

			assert.Equal(t, wantMessage, message)
			assert.Equal(t, tt.wantFix, fix)
			assert.NotContains(t, message, "; run '")
		})
	}
}

func Test_json_after_double_dash_is_a_positional(t *testing.T) {
	t.Run("a positional --json after -- stays text", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"status", "x", "--", "--json"}, nil, &stdout, &stderr)

		require.ErrorIs(t, err, cli.ErrUsage)
		assert.Equal(t, 2, cli.ExitCode(err))
		assert.Empty(t, stdout.String())
		assert.Equal(t, "brief status: too many arguments; run 'brief status'", oneLine(t, &stderr))
	})

	t.Run("a bare --json before -- still turns json mode on", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"status", "x", "--json", "--"}, nil, &stdout, &stderr)

		require.ErrorIs(t, err, cli.ErrUsage)
		assert.Equal(t, 2, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		message, _ := decodeUsageErrorDocument(t, stdout.Bytes(), "status", nil)
		assert.Equal(t, "brief status: too many arguments; run 'brief status'", message)
	})

	t.Run("root -- --json keeps -- itself as the unknown command", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"--", "--json"}, nil, &stdout, &stderr)

		require.ErrorIs(t, err, cli.ErrUsage)
		assert.Equal(t, 2, cli.ExitCode(err))
		assert.Empty(t, stdout.String())
		assert.Equal(t, `brief: unknown command "--"; expected one of: new, start, finish, status, check, init, doctor, uninstall`, oneLine(t, &stderr))
	})
}

// The last row is the "--" control arm: once "--json=x" is itself a
// positional, it is an ordinary "too many arguments" line.
func Test_json_with_a_value_is_a_text_usage_error(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "status --json=x", args: []string{"status", "--json=x"}, wantStderr: "brief status: '--json' takes no value; run 'brief status --json'"},
		{name: "status --json=", args: []string{"status", "--json="}, wantStderr: "brief status: '--json' takes no value; run 'brief status --json'"},
		{name: "start --json=true demo", args: []string{"start", "--json=true", "demo"}, wantStderr: "brief start: '--json' takes no value; run 'brief start <feature> --json'"},
		{name: "new --json=x", args: []string{"new", "--json=x"}, wantStderr: "brief new: '--json' takes no value; run 'brief new --help'"},
		{name: "help --json=x", args: []string{"help", "--json=x"}, wantStderr: "brief help: '--json' takes no value; run 'brief help <command>'"},
		{name: "root --json=x", args: []string{"--json=x"}, wantStderr: "brief: '--json' takes no value; run 'brief --help'"},
		{name: "--version --json=x", args: []string{"--version", "--json=x"}, wantStderr: "brief: '--json' takes no value; run 'brief --help'"},
		{name: "status --json --json=x", args: []string{"status", "--json", "--json=x"}, wantStderr: "brief status: '--json' takes no value; run 'brief status --json'"},
		{name: "status -- --json=x is a positional, not the flag", args: []string{"status", "--", "--json=x"}, wantStderr: "brief status: too many arguments; run 'brief status'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantStderr, oneLine(t, &stderr))
		})
	}
}

// wantVersion is captured from a plain "brief --version" run in this same
// test binary, not a literal: a go test binary's build info is no tag.
func Test_version_with_json_relaxes_the_sole_argument_rule(t *testing.T) {
	wd := t.TempDir()
	var textStdout, textStderr bytes.Buffer
	textErr := cli.Run(t.Context(), wd, []string{"--version"}, nil, &textStdout, &textStderr)
	require.NoError(t, textErr)
	wantVersion := strings.TrimPrefix(strings.TrimRight(textStdout.String(), "\n"), "brief ")

	tests := []struct {
		name string
		args []string
	}{
		{name: "--version --json", args: []string{"--version", "--json"}},
		{name: "--json --version", args: []string{"--json", "--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
			assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "version"}, jsonKeys(t, doc))

			var schema int
			require.NoError(t, json.Unmarshal(doc["schema"], &schema))
			assert.Equal(t, 1, schema)

			var command string
			require.NoError(t, json.Unmarshal(doc["command"], &command))
			assert.Equal(t, "brief", command)

			var ok bool
			require.NoError(t, json.Unmarshal(doc["ok"], &ok))
			assert.True(t, ok)

			var exitCode int
			require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
			assert.Equal(t, 0, exitCode)

			var version string
			require.NoError(t, json.Unmarshal(doc["version"], &version))
			assert.Equal(t, wantVersion, version)
		})
	}
}
