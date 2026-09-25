package cli_test

import (
	"bytes"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "brief completion <shell>" writes that shell's generated script to
// stdout, nothing to stderr, and exits 0.
func Test_prints_a_completion_script_for_each_supported_shell(t *testing.T) {
	tests := []struct {
		name   string
		shell  string
		marker string
	}{
		{name: "bash", shell: "bash", marker: "# bash completion V2 for brief"},
		{name: "zsh", shell: "zsh", marker: "#compdef brief"},
		{name: "fish", shell: "fish", marker: "# fish completion for brief"},
		{name: "powershell", shell: "powershell", marker: "# powershell completion for brief"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{"completion", tt.shell}, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Contains(t, stdout.String(), tt.marker)
		})
	}
}

func Test_completion_without_exactly_one_shell_is_a_one_line_usage_error(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "no shell given",
			args:       []string{"completion"},
			wantStderr: "brief completion: no shell given; run 'brief completion <bash|zsh|fish|powershell>'",
		},
		{
			name:       "too many arguments",
			args:       []string{"completion", "zsh", "bash"},
			wantStderr: "brief completion: too many arguments; run 'brief completion <bash|zsh|fish|powershell>'",
		},
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

// A single positional naming none of completionShells is a one-line usage
// error; matching is case-sensitive ("ZSH" is unknown, not "zsh").
func Test_completion_of_an_unknown_shell_is_a_one_line_usage_error(t *testing.T) {
	tests := []struct {
		name       string
		shell      string
		wantStderr string
	}{
		{
			name:       "unknown shell",
			shell:      "tcsh",
			wantStderr: `brief completion: unknown shell "tcsh"; expected one of: bash, zsh, fish, powershell`,
		},
		{
			name:       "case-sensitive match",
			shell:      "ZSH",
			wantStderr: `brief completion: unknown shell "ZSH"; expected one of: bash, zsh, fish, powershell`,
		},
		{
			name:       "empty shell name",
			shell:      "",
			wantStderr: `brief completion: unknown shell ""; expected one of: bash, zsh, fish, powershell`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{"completion", tt.shell}, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantStderr, oneLine(t, &stderr))
		})
	}
}

// "completion" never appears in an "expected one of:" list even though it
// is a real, dispatchable command: hiding it is Hidden-driven.
func Test_completion_is_absent_from_every_expected_command_list(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "bare brief",
			args:       []string{},
			wantStderr: "brief: no command given; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
		},
		{
			name:       "unknown command",
			args:       []string{"bogus"},
			wantStderr: `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall` + "\n",
		},
		{
			name:       "unknown help topic",
			args:       []string{"help", "bogus"},
			wantStderr: `brief help: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantStderr, stderr.String())
		})
	}
}

// Cobra writes its own "Completion ended with directive: …" line to
// stderr on every run, so stderr is deliberately not asserted empty here.
func Test_the_hidden_complete_command_answers_for_generated_scripts(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"__complete", ""}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "start\t")
	assert.Regexp(t, `:\d+\n?$`, stdout.String())
}

// "new"'s Short is never rendered by root help, only observable here via
// the generated completion script's runtime callback.
func Test_the_hidden_complete_command_describes_new_with_its_own_short(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"__complete", ""}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "new\tscaffold a feature or its next step\n")
}
