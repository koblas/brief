package cli_test

import (
	"bytes"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_prints_a_completion_script_for_each_supported_shell pins SCENARIO-12:
// "brief completion <shell>" writes that shell's generated script to
// stdout, nothing to stderr, and exits 0. Each marker is the header line
// cobra's own generator writes for that shell (observed against the real
// generated output, not a golden of the whole script — cobra owns the
// bytes).
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

// Test_completion_without_exactly_one_shell_is_a_one_line_usage_error pins
// R2's arg-count shapes (start's own "no feature given" / "too many
// arguments" pattern) for "brief completion": no shell named, and more than
// one.
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

// Test_completion_of_an_unknown_shell_is_a_one_line_usage_error pins
// SCENARIO-13: a single positional that names none of completionShells is a
// one-line usage error, "expected one of:" naming the table in order. Three
// rows, each pinning a decision a plausible refactor would flip:
//   - "tcsh": the ordinary unknown-shell case.
//   - "ZSH": matching is case-sensitive; folding case would turn this
//     row into exit 0 with a generated script.
//   - "" (empty name): an unknown shell, not "no shell given" — folding an
//     empty positional into the zero-args guard would turn this row into
//     that other message.
//
// Mutation-verified: reordering completionShells reddens every row (list
// derives from the table's order, not a literal); %q -> %s in the miss
// branch's Sprintf reddens every row (quoting is part of the contract);
// strings.EqualFold-ing the match reddens only the "ZSH" row; folding
// len(rest) == 0 || rest[0] == "" into the zero-args guard reddens only the
// "" row.
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

// Test_completion_is_absent_from_every_expected_command_list pins that
// "completion" never appears in an "expected one of:" list even though it
// is a real, dispatchable command: hiding it from listings is
// IsAvailableCommand-driven (Hidden), not a name-based exclusion.
// Mutation-verified: temporarily removing completion's Hidden field turns
// every row below red (a trailing ", completion" appears in each stderr).
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

// Test_the_hidden_complete_command_answers_for_generated_scripts is a
// regression pin, not new wiring: cobra's initCompleteCmd adds the hidden
// "__complete" command to every tree on Execute regardless of
// CompletionOptions.DisableDefaultCmd, and it must keep answering once
// "completion" is registered. Cobra writes its own
// "Completion ended with directive: …" line to stderr on every run, so
// stderr is deliberately not asserted empty here.
func Test_the_hidden_complete_command_answers_for_generated_scripts(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"__complete", ""}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "start\t")
	assert.Regexp(t, `:\d+\n?$`, stdout.String())
}

// Test_the_hidden_complete_command_describes_new_with_its_own_short pins
// that "new"'s Short reaches a user somewhere despite root help never
// rendering it (helpTemplate always expands "new" into its children's rows
// instead): cobra's generated completion scripts call back into
// "__complete" at runtime rather than baking descriptions into the script
// bytes, so this is the one place newShort is observable.
func Test_the_hidden_complete_command_describes_new_with_its_own_short(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"__complete", ""}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "new\tscaffold a feature or its next step\n")
}
