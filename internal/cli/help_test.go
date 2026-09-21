package cli_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHelp is "brief start --help"'s exact stdout: a generated Usage
// line, start's prose verbatim, its trailing JSON paragraph, and pflag's
// own flag table for -h/--help and --json — nothing else.
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

With --json, this command writes one JSON document on stdout: the common header
(` + "`schema`, `command`, `ok`, `exit_code`" + `; on a usage error or refusal an ` + "`error`" + `
object carries the failure), then its own top-level fields, in document order:
` + "`done`, `open`, `step`, `inherited`, `shortfalls`" + `.
"step" is null when there is no open step, and --json may be given before or
after <feature>.

Flags:
  -h, --help   help for start
      --json   print one JSON document on stdout
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
// finish's overlong row wrapped to its own line, and the two trailers —
// "Run 'brief <command> --help' for details." then, as the render's last
// line, "Run 'brief --version' to print the installed version." (R7).
const rootHelp = `brief manages feature specifications as files in your repository.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry
  brief start [--json] <feature>   print the next open step's context
  brief finish <feature> <step> --handoff <path> --state <path>
                                   close a step: handoff, state, then done
  brief status                     print a FEATURE/DONE/BLOCKED/NEXT table of every feature
  brief check [feature]            report faults finish would now refuse to write over
  brief completion <bash|zsh|fish|powershell>
                                   print a shell completion script

Run 'brief <command> --help' for details.
Run 'brief --version' to print the installed version.
`

// newHelp is "brief new --help"'s exact stdout: new's group body, listing
// its two children's rows (copied from rootHelp's own "new feature"/"new
// step" rows, same cmdRow padding), and the "new"-scoped trailer naming
// "new"'s own commandNounAnnotation, "type", rather than root's "command"
// — no Usage line and no Flags table, since new's own UseLine is rendered
// nowhere.
const newHelp = `Scaffolds a new feature, or the next step of an existing feature.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry

Run 'brief new <type> --help' for details.
`

// Test_prints_new_help_listing_its_two_types pins SCENARIO-10: "new
// --help", "new -h" and "help new" all render newHelp byte-identical, exit
// 0, stderr empty.
func Test_prints_new_help_listing_its_two_types(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "new --help", args: []string{"new", "--help"}},
		{name: "new -h", args: []string{"new", "-h"}},
		{name: "help new", args: []string{"help", "new"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Equal(t, newHelp, stdout.String())
		})
	}
}

// Test_new_help_flag_is_help_only_as_the_sole_argument pins that runNew's
// "-h"/"--help" routing to cmd.Help() fires only when that flag is new's
// one and only argument. A "-h"/"--help" alongside another argument, in
// either order, reports that flag as taking no arguments, naming it
// exactly as typed and pointing at "brief help new <type>" — matching
// runRoot's own wording for the same shape. Any other dash-prefixed
// argument, alone or first, gets pflag's own unknown-flag/unknown-shorthand
// wording and points at "brief new <type> --help" instead. A bare "new
// --help feature" does NOT reach feature's own help: cobra's Find has not
// yet registered new's help flag when it walks this argv, so it treats
// "feature" as --help's value and dispatch never leaves new.
func Test_new_help_flag_is_help_only_as_the_sole_argument(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "--help widget",
			args:   []string{"new", "--help", "widget"},
			stderr: `brief new: '--help' takes no arguments; run 'brief help new <type>'` + "\n",
		},
		{
			name:   "--help -x",
			args:   []string{"new", "--help", "-x"},
			stderr: `brief new: '--help' takes no arguments; run 'brief help new <type>'` + "\n",
		},
		{
			name:   "-h widget",
			args:   []string{"new", "-h", "widget"},
			stderr: `brief new: '-h' takes no arguments; run 'brief help new <type>'` + "\n",
		},
		{
			name:   "--help feature",
			args:   []string{"new", "--help", "feature"},
			stderr: `brief new: '--help' takes no arguments; run 'brief help new <type>'` + "\n",
		},
		{
			name:   "-x --help",
			args:   []string{"new", "-x", "--help"},
			stderr: "brief new: unknown shorthand flag: 'x' in -x; run 'brief new <type> --help'\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, stderr.String())
		})
	}
}

// Test_prints_the_root_help_with_one_line_per_command pins R6/R7/R8 for
// root: "brief --help" produces rootHelp exactly — trailers included — and
// "brief -h", "brief -hh" and "brief help" are byte-identical to it — every
// root-help path renders through the same cmd.Help() call.
func Test_prints_the_root_help_with_one_line_per_command(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--help", args: []string{"--help"}},
		{name: "-h", args: []string{"-h"}},
		{name: "-hh", args: []string{"-hh"}},
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
		{name: "completion", args: []string{"completion", "--help"}, path: "brief completion"},
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

// Test_help_topic_prints_the_same_bytes_as_the_command_help_flag pins R8:
// "brief help <path…>" renders exactly what "brief <path…> --help" renders,
// for every leaf in the tree — both exit nil, both write nothing to
// stderr, and the two stdouts are byte-identical. The "--help" capture
// must also be non-empty, so a mutation making both sides render empty
// cannot pass this table by symmetry alone.
func Test_help_topic_prints_the_same_bytes_as_the_command_help_flag(t *testing.T) {
	tests := []struct {
		name string
		path []string
	}{
		{name: "new", path: []string{"new"}},
		{name: "new feature", path: []string{"new", "feature"}},
		{name: "new step", path: []string{"new", "step"}},
		{name: "start", path: []string{"start"}},
		{name: "status", path: []string{"status"}},
		{name: "check", path: []string{"check"}},
		{name: "finish", path: []string{"finish"}},
		{name: "completion", path: []string{"completion"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()

			var helpFlagStdout, helpFlagStderr bytes.Buffer
			helpFlagArgs := append(append([]string{}, tc.path...), "--help")
			helpFlagErr := cli.Run(t.Context(), wd, helpFlagArgs, nil, &helpFlagStdout, &helpFlagStderr)

			var helpTopicStdout, helpTopicStderr bytes.Buffer
			helpTopicArgs := append([]string{"help"}, tc.path...)
			helpTopicErr := cli.Run(t.Context(), wd, helpTopicArgs, nil, &helpTopicStdout, &helpTopicStderr)

			require.NoError(t, helpFlagErr)
			require.NoError(t, helpTopicErr)
			assert.Empty(t, helpFlagStderr.String())
			assert.Empty(t, helpTopicStderr.String())
			assert.NotEmpty(t, helpFlagStdout.String())
			assert.Equal(t, helpFlagStdout.String(), helpTopicStdout.String())
		})
	}
}

// Test_help_start_prints_the_literal_start_help anchors R8 against the
// literal startHelp golden, not just against a live "--help" capture: a
// mutation that moves both sides of the comparison identically cannot pass
// this assertion.
func Test_help_start_prints_the_literal_start_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help", "start"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, startHelp, stdout.String())
}

// Test_help_with_an_unresolved_topic_is_a_one_line_usage_error pins
// SCENARIO-09: a help topic is accepted only when Find's residual is
// empty and the resolved target is root or IsAvailableCommand. Anything
// else — extra positionals, a flag after the topic, or a hidden command
// like "help" itself — is a usage error naming the whole topic as typed,
// never just the unresolved residual. A dash-prefixed topic never reaches
// Find at all: see Test_help_flag_as_the_topic_argument_takes_no_arguments
// and Test_help_reports_a_non_help_dash_prefixed_topic_as_an_unknown_flag
// for those two shapes.
func Test_help_with_an_unresolved_topic_is_a_one_line_usage_error(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "unknown top-level topic",
			args:   []string{"help", "bogus"},
			stderr: "brief help: unknown command \"bogus\"; expected one of: new, start, finish, status, check\n",
		},
		{
			name:   "resolved command with an unresolved trailing word",
			args:   []string{"help", "new", "bogus"},
			stderr: "brief help: unknown command \"new bogus\"; expected one of: new, start, finish, status, check\n",
		},
		{
			name:   "resolved command with an extra positional",
			args:   []string{"help", "start", "extra"},
			stderr: "brief help: unknown command \"start extra\"; expected one of: new, start, finish, status, check\n",
		},
		{
			name:   "resolved command with a trailing flag",
			args:   []string{"help", "start", "--bogus"},
			stderr: "brief help: unknown command \"start --bogus\"; expected one of: new, start, finish, status, check\n",
		},
		{
			name:   "hidden command as topic",
			args:   []string{"help", "help"},
			stderr: "brief help: unknown command \"help\"; expected one of: new, start, finish, status, check\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, stderr.String())
		})
	}
}

// Test_help_flag_as_the_topic_argument_takes_no_arguments pins that "brief
// help" checks its first argument for "-h"/"--help" before ever calling
// Find: a sole "-h"/"--help" is its own case (see
// Test_help_flag_as_the_sole_argument_prints_the_help_stubs_own_usage), but
// that flag alongside another argument is never valid — reported as taking
// no arguments, naming whichever spelling was typed, the same wording
// runRoot and runNew report for the same shape.
func Test_help_flag_as_the_topic_argument_takes_no_arguments(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "--help with a trailing topic",
			args:   []string{"help", "--help", "start"},
			stderr: "brief help: '--help' takes no arguments; run 'brief help <command>'\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, stderr.String())
		})
	}
}

// helpHelp is "brief help -h"'s exact stdout: the help stub's own generated
// Usage line ("brief help [command]", no "[flags]" suffix since
// newHelpCommand sets DisableFlagsInUseLine), its own Long prose plus its
// trailing JSON paragraph, and a Flags table with the auto-registered
// -h/--help and --json — the same leaf shape every other command's own
// "--help" renders.
const helpHelp = `Usage:
  brief help [command]

Prints help for a command. 'brief help <command>' prints the same text as
'brief <command> --help'; with no command it prints the overview.

With --json, this command writes one JSON document on stdout: the common header
(` + "`schema`, `command`, `ok`, `exit_code`" + `; on a usage error or refusal an ` + "`error`" + `
object carries the failure), then its own top-level fields, in document order:
` + "`commands`" + `.

Flags:
  -h, --help   help for help
      --json   print one JSON document on stdout
`

// Test_help_flag_as_the_sole_argument_prints_the_help_stubs_own_usage pins
// that "-h", "--help" and the all-'h' cluster "-hh", each as help's one and
// only argument, print the help stub's own usage rather than erroring: a
// bare "brief help" already answers "help me use help" by printing root's
// own help, so asking "brief help" for help on "--help" is that exact
// sole-argument case, not the "takes no arguments" wording
// Test_help_flag_as_the_topic_argument_takes_no_arguments pins for the
// flag alongside another argument.
func Test_help_flag_as_the_sole_argument_prints_the_help_stubs_own_usage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "-h", args: []string{"help", "-h"}},
		{name: "--help", args: []string{"help", "--help"}},
		{name: "-hh", args: []string{"help", "-hh"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))
			assert.Empty(t, stderr.String())
			assert.Equal(t, helpHelp, stdout.String())
		})
	}
}

// Test_help_reports_a_non_help_dash_prefixed_topic_as_an_unknown_flag pins
// that a dash-prefixed topic other than "-h"/"--help" is reported in
// pflag's own unknown-flag/unknown-shorthand wording, the same as runRoot
// and runNew report the same shape — never routed through Find, which
// would otherwise stop at root and quote the whole residual instead.
func Test_help_reports_a_non_help_dash_prefixed_topic_as_an_unknown_flag(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "long flag before a topic",
			args:   []string{"help", "--bogus", "start"},
			stderr: "brief help: unknown flag: --bogus; run 'brief help <command>'\n",
		},
		{
			name:   "single-dash short flag alone",
			args:   []string{"help", "-x"},
			stderr: "brief help: unknown shorthand flag: 'x' in -x; run 'brief help <command>'\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, stderr.String())
		})
	}
}

// finishHelp is "brief finish --help"'s exact stdout: --handoff and
// --state show their value as "path" (from the backquoted varname in each
// flag's usage string), not pflag's default "string", each usage string's
// own embedded newline wraps it to the description column — every line at
// or under 80 columns — and pflag's own sort order puts --json between
// --help and --state.
const finishHelp = `Usage:
  brief finish <feature> <step> --handoff <path> --state <path>

Closes step in feature: writes the body at --handoff to the step's own
handoff file, replaces the feature's state file with the body at --state,
and marks the step done in the progress list. "-" reads a flag's body
from stdin; it may be given for at most one of --handoff and --state.

With --json, this command writes one JSON document on stdout: the common header
(` + "`schema`, `command`, `ok`, `exit_code`" + `; on a usage error or refusal an ` + "`error`" + `
object carries the failure), then its own top-level fields, in document order:
` + "`feature`, `step`, `changed`, `handoff_path`, `state_path`, `next`" + `.

Flags:
      --handoff path   the path to the step's handoff body,
                       written to its own file
  -h, --help           help for finish
      --json           print one JSON document on stdout
      --state path     the path to the COMPLETE replacement body for the state
                       file; it replaces the file, it is never appended to; it
                       must carry the configured state headings, though a
                       section may be empty
`

// Test_prints_finish_flag_prose_in_its_flag_table pins finishHelp
// byte-identical, so a mutation that reflows the wrap points or drops a
// word from either flag's prose is caught, not just a substring survival
// check.
func Test_prints_finish_flag_prose_in_its_flag_table(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, finishHelp, stdout.String())
}

// Test_every_leaf_help_line_fits_in_80_columns sweeps every leaf's "--help"
// output — the commands that render a Flags table, the surface MAJOR 1
// fixed — for a generated Usage line, wrapped prose, and a pflag flag
// table long enough for one leaf's flags to overrun 80 columns unless its
// usage string carries its own embedded wrap points, the way jsonFlagUsage
// and handoffFlagUsage/stateFlagUsage do. Root and "new" are out of scope
// here: their cmdList rows are fixed-column-padded, not wrapped to a
// terminal width, an existing and separately reviewed layout (rootHelp,
// newHelp) this fix does not touch. The help stub's own sole-argument "-h"
// render is a leaf shape too, covered here alongside the rest.
// require.NotEmpty on stdout guards the loop below from passing vacuously
// against an empty or truncated render.
func Test_every_leaf_help_line_fits_in_80_columns(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "new feature", args: []string{"new", "feature", "--help"}},
		{name: "new step", args: []string{"new", "step", "--help"}},
		{name: "start", args: []string{"start", "--help"}},
		{name: "finish", args: []string{"finish", "--help"}},
		{name: "status", args: []string{"status", "--help"}},
		{name: "check", args: []string{"check", "--help"}},
		{name: "completion", args: []string{"completion", "--help"}},
		{name: "help", args: []string{"help", "-h"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			require.NotEmpty(t, stdout.String())
			for line := range strings.SplitSeq(stdout.String(), "\n") {
				assert.LessOrEqual(t, len(line), 80, "line %q of %q help must fit in 80 columns", line, tc.name)
			}
		})
	}
}

// jsonFlagRowRE matches the ruled --json row every JSON-capable leaf's
// Flags table must carry, whitespace-tolerant between the flag name and
// its usage so a column-width change elsewhere in the table can never
// spuriously break this assertion.
var jsonFlagRowRE = regexp.MustCompile(`(?m)^\s*--json\s+print one JSON document on stdout\s*$`)

// Test_every_command_help_lists_the_json_flag_row pins SCENARIO-14: every
// JSON-capable leaf's own "--help" carries one --json row with the ruled
// usage line. completion is the control arm: R11 forbids advertising
// --json there, so its own "--help" must not mention "--json" at all.
func Test_every_command_help_lists_the_json_flag_row(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "new feature", args: []string{"new", "feature", "--help"}},
		{name: "new step", args: []string{"new", "step", "--help"}},
		{name: "start", args: []string{"start", "--help"}},
		{name: "finish", args: []string{"finish", "--help"}},
		{name: "status", args: []string{"status", "--help"}},
		{name: "check", args: []string{"check", "--help"}},
		{name: "help", args: []string{"help", "-h"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Regexp(t, jsonFlagRowRE, stdout.String())
		})
	}

	t.Run("completion carries no --json row", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"completion", "--help"}, nil, &stdout, &stderr)

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.NotContains(t, stdout.String(), "--json")
	})
}

// jsonParagraphMarker is the first line of every JSON-capable command's
// own JSON paragraph — jsonParagraphHeaderClause's own first wrapped line
// in production — copied here as a literal so this test can slice a
// command's own field-key sentence out of the rest of its prose without
// reaching into cli's unexported symbols.
const jsonParagraphMarker = "With --json, this command writes one JSON document on stdout: the common header"

// headerJSONKeys are the header keys every --json document carries,
// dropped before checking a document's own top-level fields against its
// help text: they are never named in a command's own JSON paragraph, only
// jsonParagraphMarker's shared header clause covers them.
var headerJSONKeys = map[string]bool{"schema": true, "command": true, "ok": true, "exit_code": true}

// wholeWordPresent reports whether word appears in text as a whole word:
// not as a substring of a longer word (so "feature" does not match inside
// "features", and "step" does not match inside a longer identifier).
func wholeWordPresent(t *testing.T, text, word string) bool {
	t.Helper()

	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)

	return re.MatchString(text)
}

// jsonFieldsCase is one row of
// Test_every_command_help_names_its_json_documents_top_level_fields: run
// produces the command's own --json stdout and error, plus the same
// command's own "--help" text to check the fields against; wantExit is
// this row's own expected ExitCode, since check --json exits 1 on this
// fixture's own ERROR finding (R4) while every other row exits 0.
type jsonFieldsCase struct {
	name     string
	run      func(t *testing.T) (jsonStdout []byte, helpText string, err error)
	wantExit int
}

// runJSONAndHelp runs jsonArgs and helpArgs against the same wd, in that
// order, and returns jsonArgs' own stdout/error and helpArgs' own stdout —
// the shared shape every jsonFieldsCase.run in the table below builds on.
func runJSONAndHelp(t *testing.T, wd string, jsonArgs, helpArgs []string) ([]byte, string, error) {
	t.Helper()

	var jsonStdout, jsonStderr bytes.Buffer
	err := cli.Run(t.Context(), wd, jsonArgs, nil, &jsonStdout, &jsonStderr)
	require.Empty(t, jsonStderr.String())

	var helpStdout, helpStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wd, helpArgs, nil, &helpStdout, &helpStderr))
	require.Empty(t, helpStderr.String())

	return jsonStdout.Bytes(), helpStdout.String(), err
}

// Test_every_command_help_names_its_json_documents_top_level_fields pins
// SCENARIO-14's own key-coverage clause: each command's own JSON
// paragraph, in its "--help" text, names every one of that command's own
// top-level --json fields (the live document's own keys, header keys
// dropped, never a literal list) as a whole word.
func Test_every_command_help_names_its_json_documents_top_level_fields(t *testing.T) {
	tests := []jsonFieldsCase{
		{
			name: "new feature",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := t.TempDir()

				return runJSONAndHelp(t, wd, []string{"new", "feature", "demo", "--json"}, []string{"new", "feature", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "new step",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := t.TempDir()
				var featStdout, featStderr bytes.Buffer
				require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "demo"}, nil, &featStdout, &featStderr))

				return runJSONAndHelp(t, wd, []string{"new", "step", "demo", "--json"}, []string{"new", "step", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "start",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := newStartFixture(t, "open")

				return runJSONAndHelp(t, wd, []string{"start", "--json", "demo"}, []string{"start", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "finish",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := newFinishCLIFixture(t)
				handoffPath := writeInput(t, "handoff.md", "HANDOFF\n")
				statePath := writeInput(t, "state.md", "## Binding decisions\n\nd\n\n## Left unbuilt\n\nn\n\n## Traps\n\nn\n\n## Open debts\n\nn\n")

				return runJSONAndHelp(t, wd,
					[]string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"},
					[]string{"finish", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "status",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := newStatusJSONFixture(t)

				return runJSONAndHelp(t, wd, []string{"status", "--json"}, []string{"status", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "check",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := newCheckJSONFixture(t)

				return runJSONAndHelp(t, wd, []string{"check", "--json"}, []string{"check", "--help"})
			},
			wantExit: 1,
		},
		{
			name: "help",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := t.TempDir()

				return runJSONAndHelp(t, wd, []string{"help", "--json"}, []string{"help", "-h"})
			},
			wantExit: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonStdout, helpText, err := tc.run(t)

			assert.Equal(t, tc.wantExit, cli.ExitCode(err))

			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(jsonStdout, &doc))

			idx := strings.Index(helpText, jsonParagraphMarker)
			require.GreaterOrEqual(t, idx, 0, "no JSON paragraph found in %q help text %q", tc.name, helpText)
			paragraph := helpText[idx:]

			for key := range doc {
				if headerJSONKeys[key] {
					continue
				}

				assert.True(t, wholeWordPresent(t, paragraph, key), "%q help paragraph %q must name key %q as a whole word", tc.name, paragraph, key)
			}
		})
	}
}

// Test_status_and_check_help_say_the_text_layout_may_change pins the
// exact sentence status and check's own Long end with, after their JSON
// paragraph; start and finish are the control arm, since no other command
// carries it.
func Test_status_and_check_help_say_the_text_layout_may_change(t *testing.T) {
	const sentence = "For scripts, use --json; the text layout may change."

	tests := []struct {
		name    string
		args    []string
		carries bool
	}{
		{name: "status", args: []string{"status", "--help"}, carries: true},
		{name: "check", args: []string{"check", "--help"}, carries: true},
		{name: "start", args: []string{"start", "--help"}, carries: false},
		{name: "finish", args: []string{"finish", "--help"}, carries: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())

			if tc.carries {
				assert.Contains(t, stdout.String(), sentence)
			} else {
				assert.NotContains(t, stdout.String(), sentence)
			}
		})
	}
}
