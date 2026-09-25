// White-box: reaches the unexported newRootCommand to register commands
// production never registers, so a black-box test's fixed tree cannot.

package cli

import (
	"bytes"
	"runtime/debug"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTreeWithExtraCommands adds one visible command ("extra") and one
// command ("hiddenextra") whose Hidden field is hiddenExtraHidden.
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

// An invalid value for a non-bool flag reaches the error frame as pflag's
// raw wording, unrewritten by boolFlagParseMessage's bool-only rewrite.
func Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag(t *testing.T) {
	root, stdout, stderr := newTreeWithExtraCommands(t, true)
	root.SetArgs([]string{"extra", "--count=notanumber"})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), `strconv.ParseInt: parsing "notanumber": invalid syntax`)
	assert.NotContains(t, stderr.String(), "want true or false")
}

// A tree holding a command production does not register must list it in
// every "expected one of:" site too, and never list a hidden one.
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

// Control arm: with Hidden flipped to false, "hiddenextra" must appear in
// the list.
func Test_expected_command_list_includes_a_command_once_it_is_not_hidden(t *testing.T) {
	root, stdout, stderr := newTreeWithExtraCommands(t, false)
	root.SetArgs([]string{"bogus"})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall, extra, hiddenextra`+"\n", stderr.String())
}
