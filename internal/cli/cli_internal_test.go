// This file reaches the unexported newRootCommand only to register two
// commands production never registers — one visible, one hidden — so the
// "expected one of:" derivation can be proven against a tree that differs
// from production's own. A black-box test cannot do that: Run always
// builds production's own tree. Every other cli behavior stays covered by
// the black-box cli_test files.

package cli

import (
	"bytes"
	"runtime/debug"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTreeWithExtraCommands builds production's own command tree via
// newRootCommand, then adds one extra visible command ("extra") and one
// extra command ("hiddenextra") whose Hidden field is hiddenExtraHidden —
// the only field that varies between the two tests below. It passes a
// non-nil readBuildInfo only to satisfy newRootCommand's signature: none of
// this file's tests dispatch "--version".
func newTreeWithExtraCommands(t *testing.T, hiddenExtraHidden bool) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	wd := t.TempDir()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	readBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }
	out := reporter{stdout: stdout, stderr: stderr}
	root := newRootCommand(wd, nil, out, readBuildInfo)
	extra := &cobra.Command{
		Use:  "extra",
		Args: cobra.ArbitraryArgs,
		RunE: func(*cobra.Command, []string) error { return nil },
	}
	extra.Flags().Int("count", 0, "an int flag, to prove boolFlagParseMessage's bool type guard: it must not rewrite an invalid *int* value's pflag wording")
	root.AddCommand(
		extra,
		&cobra.Command{
			Use:    "hiddenextra",
			Hidden: hiddenExtraHidden,
			Args:   cobra.ArbitraryArgs,
			RunE:   func(*cobra.Command, []string) error { return nil },
		},
	)

	return root, stdout, stderr
}

// Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag pins
// boolFlagParseMessage's type guard: an invalid value for "extra"'s Int
// flag "count" reaches the root FlagErrorFunc frame as pflag's own raw
// strconv.ParseInt wording, unrewritten — proving the rewrite the
// "status --help=x --json" row of
// Test_json_mode_usage_error_message_is_the_text_mode_line
// (json_usage_test.go) pins for a real bool flag is scoped to bool-typed
// flags, not every flag pflag rejects a value for.
//
// Mutation-verified: dropping boolFlagParseMessage's
// "invalid.GetFlag().Value.Type() != \"bool\"" guard reds this test — the
// int-flag error would be rewritten into the bool wording, naming a value
// of "want true or false" for a flag that takes neither.
func Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag(t *testing.T) {
	root, stdout, stderr := newTreeWithExtraCommands(t, true)
	root.SetArgs([]string{"extra", "--count=notanumber"})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), `strconv.ParseInt: parsing "notanumber": invalid syntax`)
	assert.NotContains(t, stderr.String(), "want true or false")
}

// Test_expected_command_list_names_every_visible_registered_command pins
// R7 at all three call sites that name an "expected one of:" list: a
// tree holding a command production does not register must list it too,
// and must never list a hidden one — proving the list is derived from the
// tree, not the retired expectedCommands literal.
func Test_expected_command_list_names_every_visible_registered_command(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "no command given",
			args:   []string{},
			stderr: "brief: no command given; expected one of: new, start, finish, status, check, init, doctor, uninstall, extra\n",
		},
		{
			name:   "unknown command",
			args:   []string{"bogus"},
			stderr: `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall, extra` + "\n",
		},
		{
			name:   "unknown help topic",
			args:   []string{"help", "bogus"},
			stderr: `brief help: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall, extra` + "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, stdout, stderr := newTreeWithExtraCommands(t, true)
			root.SetArgs(tc.args)

			err := root.ExecuteContext(t.Context())

			require.ErrorIs(t, err, ErrUsage)
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, stderr.String())
		})
	}
}

// Test_expected_command_list_includes_a_command_once_it_is_not_hidden is
// the control arm for the negative claim above: with "hiddenextra"'s
// Hidden field flipped to false — the only field that differs from the
// tree in the table above — its name must appear in the list. Without
// this, a filter that always drops "hiddenextra" by name would pass the
// negative assertion above for the wrong reason.
func Test_expected_command_list_includes_a_command_once_it_is_not_hidden(t *testing.T) {
	root, stdout, stderr := newTreeWithExtraCommands(t, false)
	root.SetArgs([]string{"bogus"})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall, extra, hiddenextra`+"\n", stderr.String())
}
