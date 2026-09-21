package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHelp is "brief start --help"'s exact stdout: a generated Usage
// line, start's prose verbatim, and pflag's own flag table for -h/--help
// and --json — nothing else.
const startHelp = `Usage:
  brief start [--json] <feature>

Prints the next open step's id, title, acceptance criteria and checklist,
and the decisions and constraints inherited from the feature's state
file. A feature whose steps are all done, or that has no step files yet,
prints nothing and says so on stderr instead, still exiting 0.
brief start refuses, naming the file and the fix, rather than print a
partial brief: a missing or unreadable specification or state file, an
unclosed fenced code block in either, a specification with no progress
heading, or a next step whose frontmatter has no id or no checklist. A
missing optional convention — the step's acceptance heading, or a state
file heading — is named on stderr instead, one line each, and the brief
still prints on stdout, still exiting 0.
brief start reads; it never writes.

Flags:
  -h, --help   help for start
      --json   print the brief as a single JSON document instead of markdown.
               Every exit-0 run writes one, even when there is no open step:
               "step" is null rather than the document being omitted, so a
               structured caller detects completion the same way a human
               reads the stderr notice. --json may be given before or after
               <feature>.
`

// Test_prints_start_help_as_usage_line_prose_and_flag_table pins R6 for one
// leaf: the generated Usage line, start's prose verbatim, and a Flags
// table — nothing else on stdout, nothing on stderr, exit 0.
func Test_prints_start_help_as_usage_line_prose_and_flag_table(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, startHelp, stdout.String())
}

// rootHelp is root's exact stdout for "brief --help", "brief -h" and
// "brief help": the one-sentence description, one row per available
// command (new's two children in new's place, in registration order),
// finish's overlong row wrapped to its own line, and the trailer.
const rootHelp = `brief manages feature specifications as files in your repository.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry
  brief start [--json] <feature>   print the next open step's context
  brief status                     print one done/total/next/blocked line per feature
  brief check [feature]            report faults finish would now refuse to write over
  brief finish <feature> <step> --handoff <path> --state <path>
                                   close a step: handoff, state, then done

Run 'brief <command> --help' for details.
`

// Test_prints_the_root_help_with_one_line_per_command pins R6/R8 for root:
// "brief --help" produces rootHelp exactly, and "brief -h" and "brief
// help" are byte-identical to it — every root-help path renders through
// the same cmd.Help() call.
func Test_prints_the_root_help_with_one_line_per_command(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--help", args: []string{"--help"}},
		{name: "-h", args: []string{"-h"}},
		{name: "help", args: []string{"help"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Equal(t, rootHelp, stdout.String())
		})
	}
}

// Test_every_command_help_has_a_usage_line_and_a_flag_table is a
// structural sweep across every leaf: a generated Usage line naming the
// command's own path, a Flags table with -h/--help, and none of cobra's
// extra sections. It does not, by itself, prove those sections are
// reachable on a tree this small — see the mutation checks pinning that
// separately.
func Test_every_command_help_has_a_usage_line_and_a_flag_table(t *testing.T) {
	tests := []struct {
		name string
		args []string
		path string
	}{
		{name: "new feature", args: []string{"new", "feature", "--help"}, path: "brief new feature"},
		{name: "new step", args: []string{"new", "step", "--help"}, path: "brief new step"},
		{name: "start", args: []string{"start", "--help"}, path: "brief start"},
		{name: "status", args: []string{"status", "--help"}, path: "brief status"},
		{name: "check", args: []string{"check", "--help"}, path: "brief check"},
		{name: "finish", args: []string{"finish", "--help"}, path: "brief finish"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			out := stdout.String()
			assert.True(t, strings.HasPrefix(out, "Usage:\n  "+tc.path), "stdout %q must start with the generated Usage line for %q", out, tc.path)
			assert.Contains(t, out, "Flags:")
			assert.Contains(t, out, "-h, --help")
			assert.NotContains(t, out, "Global Flags:")
			assert.NotContains(t, out, "Additional help topics:")
			assert.NotContains(t, out, "Available Commands:")
		})
	}
}

// Test_prints_finish_flag_prose_in_its_flag_table pins that finish's
// --handoff and --state rows show their value as "path" (from the
// backquoted varname in each flag's usage string), not pflag's default
// "string", and that --state's COMPLETE-replacement prose survives into
// the generated table.
func Test_prints_finish_flag_prose_in_its_flag_table(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	out := stdout.String()
	assert.Contains(t, out, "--handoff path")
	assert.Contains(t, out, "--state path")
	assert.Contains(t, out, "the COMPLETE replacement body for the state file; it replaces the file, it is never appended to; it must carry the configured state headings, though a section")
}
