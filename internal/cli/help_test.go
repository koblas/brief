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

// startHelp is "brief start --help"'s exact stdout.
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

func Test_prints_start_help_as_usage_line_prose_and_flag_table(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, startHelp, stdout.String())
}

// rootHelp is root's exact stdout for "brief --help", "brief -h" and
// "brief help".
const rootHelp = `brief manages feature specifications as files in your repository.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry
  brief start [--json] <feature>   print the next open step's context
  brief finish <feature> <step> --handoff <path> --state <path>
                                   close a step: handoff, state, then done
  brief status                     print a FEATURE/DONE/BLOCKED/NEXT table of every feature
  brief check [feature] [--hook <host>]
                                   report faults finish would now refuse to write over
  brief init [--host <name>] [--no-hook] [--with-agents] [--edit-agents] [--dry-run | --print] [--force] [--json]
                                   install brief's config and agent-host integration
  brief doctor [--json]            check brief's setup: config, feature root, host integration
  brief uninstall [--host <name>] [--dry-run] [--force] [--json]
                                   remove what init installed
  brief completion <bash|zsh|fish|powershell>
                                   print a shell completion script

Run 'brief <command> --help' for details.
Run 'brief --version' to print the installed version.
`

// newHelp is "brief new --help"'s exact stdout.
const newHelp = `Scaffolds a new feature, or the next step of an existing feature.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry

Run 'brief new <type> --help' for details.
`

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

// "-h"/"--help" routes to cmd.Help() only when it is new's one and only
// argument; alongside another argument it reports taking no arguments
// instead.
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
		{name: "init", args: []string{"init", "--help"}, path: "brief init"},
		{name: "doctor", args: []string{"doctor", "--help"}, path: "brief doctor"},
		{name: "uninstall", args: []string{"uninstall", "--help"}, path: "brief uninstall"},
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

// "brief help <path…>" must render exactly what "brief <path…> --help"
// renders; the "--help" capture must also be non-empty, so a mutation
// making both sides render empty cannot pass by symmetry alone.
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
		{name: "init", path: []string{"init"}},
		{name: "doctor", path: []string{"doctor"}},
		{name: "uninstall", path: []string{"uninstall"}},
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

// Anchors against the literal startHelp golden, not just a live "--help"
// capture, so a mutation moving both sides identically cannot pass.
func Test_help_start_prints_the_literal_start_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help", "start"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, startHelp, stdout.String())
}

// A help topic is accepted only when Find's residual is empty and the
// resolved target is root or IsAvailableCommand; anything else is a usage
// error naming the whole topic as typed, never just the unresolved residual.
func Test_help_with_an_unresolved_topic_is_a_one_line_usage_error(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "unknown top-level topic",
			args:   []string{"help", "bogus"},
			stderr: "brief help: unknown command \"bogus\"; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
		},
		{
			name:   "resolved command with an unresolved trailing word",
			args:   []string{"help", "new", "bogus"},
			stderr: "brief help: unknown command \"new bogus\"; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
		},
		{
			name:   "resolved command with an extra positional",
			args:   []string{"help", "start", "extra"},
			stderr: "brief help: unknown command \"start extra\"; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
		},
		{
			name:   "resolved command with a trailing flag",
			args:   []string{"help", "start", "--bogus"},
			stderr: "brief help: unknown command \"start --bogus\"; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
		},
		{
			name:   "hidden command as topic",
			args:   []string{"help", "help"},
			stderr: "brief help: unknown command \"help\"; expected one of: new, start, finish, status, check, init, doctor, uninstall\n",
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

// "brief help" checks its first argument for "-h"/"--help" before ever
// calling Find; that flag alongside another argument is never valid.
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

// helpHelp is "brief help -h"'s exact stdout.
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

// "-h", "--help" and the all-'h' cluster "-hh", each as help's one and only
// argument, print the help stub's own usage rather than erroring.
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

// A dash-prefixed topic other than "-h"/"--help" is reported in pflag's own
// unknown-flag wording, never routed through Find.
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

// dropReportingParagraph is finishLong's own paragraph between the
// flag-body prose and the JSON paragraph.
const dropReportingParagraph = `Each entry under the four state headings that is missing from the new
body is listed on stdout as a WARN finding (rule dropped-debt under the
open-debts heading, dropped-entry otherwise); its line is in the file as
it was before replacement. Removal is reported, never refused; a
reworded entry counts as removed. Exit status stays 0.`

func Test_finish_help_documents_drop_reporting_before_the_json_paragraph(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	out := stdout.String()
	dropIdx := strings.Index(out, dropReportingParagraph)
	require.GreaterOrEqual(t, dropIdx, 0, "drop-reporting paragraph not found in %q", out)

	jsonIdx := strings.Index(out, jsonParagraphMarker)
	require.GreaterOrEqual(t, jsonIdx, 0, "JSON paragraph not found in %q", out)

	assert.Less(t, dropIdx, jsonIdx, "drop-reporting paragraph must appear before the JSON paragraph")
}

// finishHelp is "brief finish --help"'s exact stdout.
const finishHelp = `Usage:
  brief finish <feature> <step> --handoff <path> --state <path>

Closes step in feature: writes the body at --handoff to the step's own
handoff file, replaces the feature's state file with the body at --state,
and marks the step done in the progress list. "-" reads a flag's body
from stdin; it may be given for at most one of --handoff and --state.

Each entry under the four state headings that is missing from the new
body is listed on stdout as a WARN finding (rule dropped-debt under the
open-debts heading, dropped-entry otherwise); its line is in the file as
it was before replacement. Removal is reported, never refused; a
reworded entry counts as removed. Exit status stays 0.

With --json, this command writes one JSON document on stdout: the common header
(` + "`schema`, `command`, `ok`, `exit_code`" + `; on a usage error or refusal an ` + "`error`" + `
object carries the failure), then its own top-level fields, in document order:
` + "`feature`, `step`, `changed`, `handoff_path`, `state_path`, `next`, `modified`,\n`dropped_entries`" + `.

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

func Test_prints_finish_flag_prose_in_its_flag_table(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, finishHelp, stdout.String())
}

// The generated Usage line is exempt by position (the line right after the
// literal "Usage:" line): cobra offers no wrap point for cmd.Use, and
// init's own Use runs past 80 columns unwrapped. Root and "new" are out of
// scope: their rows are fixed-column-padded, not wrapped to a terminal width.
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
		{name: "init", args: []string{"init", "--help"}},
		{name: "doctor", args: []string{"doctor", "--help"}},
		{name: "uninstall", args: []string{"uninstall", "--help"}},
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
			lines := strings.Split(stdout.String(), "\n")
			for i, line := range lines {
				if i > 0 && lines[i-1] == "Usage:" {
					continue
				}

				assert.LessOrEqual(t, len(line), 80, "line %q of %q help must fit in 80 columns", line, tc.name)
			}
		})
	}
}

// genericPlaceholderCase is one row of
// Test_host_and_hook_flags_render_a_generic_table_placeholder.
type genericPlaceholderCase struct {
	name          string
	args          []string
	flagRow       string
	concreteValue string
}

// pflag's UnquoteUsage renders a usage string's own backquoted word as that
// flag's table placeholder verbatim, so backquoting an accepted value
// instead of a generic word would leak that value as the placeholder for
// every value; each case also asserts that reverted rendering is absent.
func Test_host_and_hook_flags_render_a_generic_table_placeholder(t *testing.T) {
	tests := []genericPlaceholderCase{
		{
			name:          "init --host",
			args:          []string{"init", "--help"},
			flagRow:       "      --host name     the agent host name to install for: claude-code or none\n                      (default: detected)\n",
			concreteValue: "--host none     the agent host name",
		},
		{
			name:          "uninstall --host",
			args:          []string{"uninstall", "--help"},
			flagRow:       "      --host name   the agent host name to remove for: claude-code or none\n                    (default: claude-code)\n",
			concreteValue: "--host claude-code   the agent host",
		},
		{
			name:          "check --hook",
			args:          []string{"check", "--help"},
			flagRow:       "      --hook host   read a host hook payload from stdin and check only the\n                    edited feature (claude-code only)\n",
			concreteValue: "--hook claude-code   read a claude-code hook",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tc.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			out := stdout.String()
			assert.Contains(t, out, tc.flagRow)
			assert.NotContains(t, out, tc.concreteValue)
		})
	}
}

// jsonFlagRowRE matches the --json row every JSON-capable leaf's Flags
// table must carry, whitespace-tolerant so a column-width change elsewhere
// in the table can never spuriously break this assertion.
var jsonFlagRowRE = regexp.MustCompile(`(?m)^\s*--json\s+print one JSON document on stdout\s*$`)

// completion is the control arm: --json is not advertised there.
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
		{name: "init", args: []string{"init", "--help"}},
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
// own JSON paragraph.
const jsonParagraphMarker = "With --json, this command writes one JSON document on stdout: the common header"

// headerJSONKeys are the header keys every --json document carries, never
// named in a command's own JSON paragraph.
var headerJSONKeys = map[string]bool{"schema": true, "command": true, "ok": true, "exit_code": true}

// wholeWordPresent reports whether word appears in text as a whole word, not
// as a substring of a longer word.
func wholeWordPresent(t *testing.T, text, word string) bool {
	t.Helper()

	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)

	return re.MatchString(text)
}

// jsonFieldsCase is one row of
// Test_every_command_help_names_its_json_documents_top_level_fields.
type jsonFieldsCase struct {
	name     string
	run      func(t *testing.T) (jsonStdout []byte, helpText string, err error)
	wantExit int
}

// runJSONAndHelp runs jsonArgs and helpArgs against the same wd, in that
// order, returning jsonArgs' stdout/error and helpArgs' stdout.
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

// Each command's JSON paragraph must name every one of that command's own
// top-level --json fields (the live document's keys, header keys dropped)
// as a whole word.
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
			name: "doctor",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := newDoctorJSONFixture(t)

				return runJSONAndHelp(t, wd, []string{"doctor", "--json"}, []string{"doctor", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "init",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := t.TempDir()

				return runJSONAndHelp(t, wd, []string{"init", "--host", "none", "--json"}, []string{"init", "--help"})
			},
			wantExit: 0,
		},
		{
			name: "uninstall",
			run: func(t *testing.T) ([]byte, string, error) {
				t.Helper()

				wd := t.TempDir()
				var initStdout, initStderr bytes.Buffer
				require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &initStdout, &initStderr))

				return runJSONAndHelp(t, wd, []string{"uninstall", "--host", "none", "--json"}, []string{"uninstall", "--help"})
			},
			wantExit: 0,
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

// normalizeWhitespace collapses every run of whitespace in s to a single
// space, so a hand-wrapped string's line breaks never defeat a Contains
// check against a sentence given as one unbroken line.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func Test_init_help_names_the_brief_workflow_skill(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	const sentence = `Every claude-code install also writes a "brief-workflow" skill under ".claude/skills/brief-workflow/", which agents preload by listing it in their frontmatter "skills:".`
	assert.Contains(t, normalizeWhitespace(stdout.String()), sentence)

	const missingSkillSentence = `init never edits an agent file of yours by default; stderr instead lists each planner or implementer bound in ".brief.yaml" whose agent lacks it.`
	assert.Contains(t, normalizeWhitespace(stdout.String()), missingSkillSentence)

	assert.True(t, wholeWordPresent(t, stdout.String(), "agents_missing_skill"))
}

func Test_init_help_names_edit_agents(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	normalized := normalizeWhitespace(stdout.String())

	const flagUsage = `add "brief-workflow" to the "skills:" list of the planner and implementer agents bound in .brief.yaml (repository files only)`
	assert.Contains(t, normalized, flagUsage)

	const sentence = `--edit-agents adds it to those agents' "skills:" lists, for agent files under ".claude/agents/" only; one under "~/.claude" is always left for you to edit.`
	assert.Contains(t, normalized, sentence)

	assert.Contains(t, stdout.String(), "--edit-agents")
}

func Test_uninstall_help_names_the_bound_agent_skill_removal(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	const sentence = `It also removes "brief-workflow" from the "skills:" list of the planner and implementer agents bound in ".brief.yaml", repository files only, unless the skill file itself is kept.`
	assert.Contains(t, normalizeWhitespace(stdout.String()), sentence)
}

// uninstallLong's opening sentence must say the "brief-workflow" skill
// directory is removed alongside the plugin, not just the plugin.
func Test_uninstall_help_names_the_workflow_skill_directory(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	const pluginAndSkillSentence = `the Claude Code plugin under ".claude/skills/brief/" and the "brief-workflow" skill under ".claude/skills/brief-workflow/"`
	assert.Contains(t, normalizeWhitespace(stdout.String()), pluginAndSkillSentence)
}

// uninstallLong's closing "left in place" clause must speak of brief's own
// directories generally, not just "the plugin's own directory".
func Test_uninstall_help_left_in_place_clause_names_briefs_own_directories(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	normalized := normalizeWhitespace(stdout.String())

	const leftInPlaceClause = `nor is ".claude/" or ".claude/skills/" above brief's own directories.`
	assert.Contains(t, normalized, leftInPlaceClause)

	assert.NotContains(t, normalized, "above the plugin's own directory")
}

// The stale "reading ~/.claude/agents" wording doctorLong once carried
// must be gone.
func Test_doctor_help_names_role_resolution_and_the_brief_workflow_skill(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"doctor", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	sentence := `— plus whether each role bound in ".brief.yaml" resolves to an agent, matched by its frontmatter "name:" anywhere under ".claude/agents/", then` +
		` "~/.claude/agents/" when the repository defines none, and whether the bound planner and implementer preload the "brief-workflow" skill.`
	normalized := normalizeWhitespace(stdout.String())
	assert.Contains(t, normalized, sentence)
	assert.NotContains(t, normalized, `reading "~/.claude/agents"`)
}

// start and finish are the control arm: no other command carries this
// sentence.
func Test_status_and_check_help_say_the_text_layout_may_change(t *testing.T) {
	const sentence = "For scripts, use --json; the text layout may change."

	tests := []struct {
		name    string
		args    []string
		carries bool
	}{
		{name: "status", args: []string{"status", "--help"}, carries: true},
		{name: "check", args: []string{"check", "--help"}, carries: true},
		{name: "doctor", args: []string{"doctor", "--help"}, carries: true},
		{name: "start", args: []string{"start", "--help"}, carries: false},
		{name: "finish", args: []string{"finish", "--help"}, carries: false},
		{name: "init", args: []string{"init", "--help"}, carries: false},
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
