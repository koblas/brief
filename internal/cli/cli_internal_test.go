// This file reaches the unexported newRootCommand only to register two
// commands production never registers — one visible, one hidden — so the
// "expected one of:" derivation can be proven against a tree that differs
// from production's own. A black-box test cannot do that: Run always
// builds production's own tree. Every other cli behavior stays covered by
// the black-box cli_test files.

package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTreeWithExtraCommands builds production's own command tree via
// newRootCommand, then adds one extra visible command ("extra") and one
// extra command ("hiddenextra") whose Hidden field is hiddenExtraHidden —
// the only field that varies between the two tests below.
func newTreeWithExtraCommands(t *testing.T, hiddenExtraHidden bool) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	wd := t.TempDir()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	root := newRootCommand(wd, nil, stdout, stderr)
	root.AddCommand(
		&cobra.Command{
			Use:  "extra",
			Args: cobra.ArbitraryArgs,
			RunE: func(*cobra.Command, []string) error { return nil },
		},
		&cobra.Command{
			Use:    "hiddenextra",
			Hidden: hiddenExtraHidden,
			Args:   cobra.ArbitraryArgs,
			RunE:   func(*cobra.Command, []string) error { return nil },
		},
	)

	return root, stdout, stderr
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
			stderr: "brief: no command given; expected one of: new, start, finish, status, check, extra\n",
		},
		{
			name:   "unknown command",
			args:   []string{"bogus"},
			stderr: `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, extra` + "\n",
		},
		{
			name:   "unknown help topic",
			args:   []string{"help", "bogus"},
			stderr: `brief help: unknown command "bogus"; expected one of: new, start, finish, status, check, extra` + "\n",
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
	assert.Equal(t, `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, extra, hiddenextra`+"\n", stderr.String())
}
