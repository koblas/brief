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

// helpFlagJSONDecode mirrors one entry of a help document's "flags" array.
type helpFlagJSONDecode struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Usage string `json:"usage"`
}

// helpCommandJSONDecode mirrors one entry of a help document's "commands"
// array.
type helpCommandJSONDecode struct {
	Name        string               `json:"name"`
	Usage       string               `json:"usage"`
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	Flags       []helpFlagJSONDecode `json:"flags"`
}

// helpDocumentDecode mirrors "help --json"'s document shape for a
// black-box decode: the common header plus commands[].
type helpDocumentDecode struct {
	Schema   int                     `json:"schema"`
	Command  string                  `json:"command"`
	OK       bool                    `json:"ok"`
	ExitCode int                     `json:"exit_code"`
	Commands []helpCommandJSONDecode `json:"commands"`
}

// decodeHelpDocument unmarshals stdout into helpDocumentDecode and asserts
// the common header: schema 1, command "help", ok true, exit_code 0 —
// every help document's own header, whatever spelling asked and whether
// full index or filtered to one entry.
func decodeHelpDocument(t *testing.T, stdout []byte) helpDocumentDecode {
	t.Helper()

	var doc helpDocumentDecode
	require.NoError(t, json.Unmarshal(stdout, &doc))

	assert.Equal(t, 1, doc.Schema)
	assert.Equal(t, "help", doc.Command)
	assert.True(t, doc.OK)
	assert.Equal(t, 0, doc.ExitCode)

	return doc
}

// Test_help_json_lists_every_listed_command_and_new_itself pins the full
// index's membership and order: depth-first, registration order, with
// "new" itself inserted ahead of its own two children — root and the
// hidden "help" stub are never entries, while hidden-but-listed
// "completion" is (the control arm this test's name calls out).
func Test_help_json_lists_every_listed_command_and_new_itself(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	doc := decodeHelpDocument(t, stdout.Bytes())

	names := make([]string, len(doc.Commands))
	for i, c := range doc.Commands {
		names[i] = c.Name
	}

	assert.Equal(t, []string{"new", "new feature", "new step", "start", "finish", "status", "check", "completion"}, names)
	assert.Contains(t, names, "completion")
	assert.NotContains(t, names, "help")
	assert.NotContains(t, names, "brief")

	// "brief help --json" dispatches to the help stub itself, so cobra's
	// own Execute() never calls InitDefaultHelpFlag on any index entry —
	// "status" (never the resolved command) only carries a "help" row here
	// because helpEntry calls it itself; "json" is status's own registered
	// pflag (S14), sorted after "help".
	var status helpCommandJSONDecode
	for _, c := range doc.Commands {
		if c.Name == "status" {
			status = c
		}
	}
	require.Equal(t, "status", status.Name)
	require.Len(t, status.Flags, 2)
	assert.Equal(t, helpFlagJSONDecode{Name: "help", Type: "bool", Usage: "help for status"}, status.Flags[0])
	assert.Equal(t, helpFlagJSONDecode{Name: "json", Type: "bool", Usage: "print one JSON document on stdout"}, status.Flags[1])
}

// usageLineRE extracts a leaf's own generated "Usage:" line from its text
// help — the one line between the "Usage:\n  " marker and the following
// blank line.
var usageLineRE = regexp.MustCompile(`(?s)Usage:\n  (.*?)\n\n`)

// extractUsageLine returns text's own generated Usage line, the oracle
// Test_help_json_entries_agree_with_each_commands_text_help checks a
// help document's "usage" field against.
func extractUsageLine(t *testing.T, text string) string {
	t.Helper()

	m := usageLineRE.FindStringSubmatch(text)
	require.NotNil(t, m, "no Usage: line found in %q", text)

	return m[1]
}

// flagNameRE extracts a flag table row's own long name, skipping any
// shorthand ("-h, ") ahead of it.
var flagNameRE = regexp.MustCompile(`(?m)^\s*(?:-\w, )?--([\w-]+)`)

// extractFlagNames returns text's own Flags: table row names, in the
// order pflag's own FlagUsages renders them — the same order
// cmd.LocalFlags().VisitAll (pflag's sorted order) walks, so it is also a
// help document's "flags" oracle.
func extractFlagNames(t *testing.T, text string) []string {
	t.Helper()

	idx := strings.Index(text, "Flags:")
	require.GreaterOrEqual(t, idx, 0, "no Flags: section found in %q", text)

	matches := flagNameRE.FindAllStringSubmatch(text[idx:], -1)
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m[1]
	}

	return names
}

// Test_help_json_entries_agree_with_each_commands_text_help checks every
// leaf's filtered document against its own text "--help" render, captured
// in the same test run rather than pinned to a literal: usage equals the
// generated Usage line, flags[].name equals the flag table's own rows,
// summary/description are non-empty, and flags is never null. "new"
// renders the group template instead of a Usage line and flag table, so
// its entry is checked by literal usage ("brief new") in
// Test_help_json_spellings_produce_identical_documents instead.
func Test_help_json_entries_agree_with_each_commands_text_help(t *testing.T) {
	tests := []struct {
		name string
		path []string
	}{
		{name: "new feature", path: []string{"new", "feature"}},
		{name: "new step", path: []string{"new", "step"}},
		{name: "start", path: []string{"start"}},
		{name: "finish", path: []string{"finish"}},
		{name: "status", path: []string{"status"}},
		{name: "check", path: []string{"check"}},
		{name: "completion", path: []string{"completion"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()

			var textStdout, textStderr bytes.Buffer
			textArgs := append(append([]string{}, tc.path...), "--help")
			require.NoError(t, cli.Run(t.Context(), wd, textArgs, nil, &textStdout, &textStderr))
			require.Empty(t, textStderr.String())
			text := textStdout.String()

			wantUsage := extractUsageLine(t, text)
			wantFlagNames := extractFlagNames(t, text)

			var stdout, stderr bytes.Buffer
			jsonArgs := append([]string{"help"}, append(append([]string{}, tc.path...), "--json")...)
			err := cli.Run(t.Context(), wd, jsonArgs, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())

			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))

			var rawCommands []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw["commands"], &rawCommands))
			require.Len(t, rawCommands, 1)
			assert.NotEqual(t, "null", strings.TrimSpace(string(rawCommands[0]["flags"])), "flags must never be null")

			doc := decodeHelpDocument(t, stdout.Bytes())
			require.Len(t, doc.Commands, 1)
			entry := doc.Commands[0]

			assert.Equal(t, wantUsage, entry.Usage)
			assert.NotEmpty(t, entry.Summary)
			assert.NotEmpty(t, entry.Description)

			gotFlagNames := make([]string, len(entry.Flags))
			for i, f := range entry.Flags {
				gotFlagNames[i] = f.Name
			}
			assert.Equal(t, wantFlagNames, gotFlagNames)
		})
	}
}

// Test_help_json_spellings_produce_identical_documents pins ≡ across every
// spelling that names the same command: full index (4 spellings), start
// (4, "help --json start" included per the deleted pre-S13 pin), new (2),
// new feature (2) and completion (2) — byte-equal stdout within each
// group, empty stderr, exit 0, "command" "help" in every group. The "new"
// group additionally pins Step 5a: exactly one entry named "new" with
// usage "brief new", not "brief new [flags]".
func Test_help_json_spellings_produce_identical_documents(t *testing.T) {
	type group struct {
		name     string
		variants [][]string
		check    func(t *testing.T, doc helpDocumentDecode)
	}

	groups := []group{
		{
			name: "full index",
			variants: [][]string{
				{"help", "--json"},
				{"--help", "--json"},
				{"-h", "--json"},
				{"--json", "--help"},
			},
			check: func(t *testing.T, doc helpDocumentDecode) {
				t.Helper()

				names := make([]string, len(doc.Commands))
				for i, c := range doc.Commands {
					names[i] = c.Name
				}
				assert.Equal(t, []string{"new", "new feature", "new step", "start", "finish", "status", "check", "completion"}, names)
			},
		},
		{
			name: "start",
			variants: [][]string{
				{"help", "start", "--json"},
				{"start", "--help", "--json"},
				{"start", "-h", "--json"},
				{"help", "--json", "start"},
			},
			check: func(t *testing.T, doc helpDocumentDecode) {
				t.Helper()

				require.Len(t, doc.Commands, 1)
				assert.Equal(t, "start", doc.Commands[0].Name)
			},
		},
		{
			name: "new",
			variants: [][]string{
				{"help", "new", "--json"},
				{"new", "--help", "--json"},
			},
			check: func(t *testing.T, doc helpDocumentDecode) {
				t.Helper()

				require.Len(t, doc.Commands, 1)
				assert.Equal(t, "new", doc.Commands[0].Name)
				assert.Equal(t, "brief new", doc.Commands[0].Usage)
			},
		},
		{
			name: "new feature",
			variants: [][]string{
				{"help", "new", "feature", "--json"},
				{"new", "feature", "--help", "--json"},
			},
			check: func(t *testing.T, doc helpDocumentDecode) {
				t.Helper()

				require.Len(t, doc.Commands, 1)
				assert.Equal(t, "new feature", doc.Commands[0].Name)
			},
		},
		{
			name: "completion",
			variants: [][]string{
				{"help", "completion", "--json"},
				{"completion", "--help", "--json"},
			},
			check: func(t *testing.T, doc helpDocumentDecode) {
				t.Helper()

				require.Len(t, doc.Commands, 1)
				assert.Equal(t, "completion", doc.Commands[0].Name)
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			var first string

			for i, args := range g.variants {
				wd := t.TempDir()
				var stdout, stderr bytes.Buffer

				err := cli.Run(t.Context(), wd, args, nil, &stdout, &stderr)

				require.NoError(t, err)
				assert.Equal(t, 0, cli.ExitCode(err))
				assert.Empty(t, stderr.String())

				doc := decodeHelpDocument(t, stdout.Bytes())
				g.check(t, doc)

				if i == 0 {
					first = stdout.String()
				} else {
					assert.Equal(t, first, stdout.String(), "variant %v must render the same document as %v", args, g.variants[0])
				}
			}
		})
	}
}

// Test_help_flag_as_sole_argument_json_yields_the_help_stubs_own_entry
// pins "help -h --json": the filter is the command asked about, not the
// index-membership predicate, so it yields one entry describing the help
// stub itself even though the stub is never an index member.
func Test_help_flag_as_sole_argument_json_yields_the_help_stubs_own_entry(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help", "-h", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	doc := decodeHelpDocument(t, stdout.Bytes())
	require.Len(t, doc.Commands, 1)
	assert.Equal(t, "help", doc.Commands[0].Name)
}

// Test_help_finish_json_is_the_exact_document is the exact-bytes golden
// for "help finish --json": finish is chosen because its own document
// pins a string-typed flag, pflag.UnquoteUsage's backquote stripping
// ("path"), a multi-line usage string, and the auto-registered help flag
// row, all in one document.
func Test_help_finish_json_is_the_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help", "finish", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	description := "Closes step in feature: writes the body at --handoff to the step's own\n" +
		"handoff file, replaces the feature's state file with the body at --state,\n" +
		"and marks the step done in the progress list. \"-\" reads a flag's body\n" +
		"from stdin; it may be given for at most one of --handoff and --state.\n\n" +
		"With --json, this command writes one JSON document on stdout: the common header\n" +
		"(`schema`, `command`, `ok`, `exit_code`; on a usage error or refusal an `error`\n" +
		"object carries the failure), then its own top-level fields, in document order:\n" +
		"`feature`, `step`, `changed`, `handoff_path`, `state_path`, `next`, `modified`."
	handoffUsage := "the path to the step's handoff body,\nwritten to its own file"
	stateUsage := "the path to the COMPLETE replacement body for the state\n" +
		"file; it replaces the file, it is never appended to; it\n" +
		"must carry the configured state headings, though a\n" +
		"section may be empty"

	want := `{"schema":1,"command":"help","ok":true,"exit_code":0,"commands":[` +
		`{"name":"finish","usage":"brief finish <feature> <step> --handoff <path> --state <path>",` +
		`"summary":"close a step: handoff, state, then done","description":` + jsonString(t, description) + `,` +
		`"flags":[` +
		`{"name":"handoff","type":"string","usage":` + jsonString(t, handoffUsage) + `},` +
		`{"name":"help","type":"bool","usage":"help for finish"},` +
		`{"name":"json","type":"bool","usage":"print one JSON document on stdout"},` +
		`{"name":"state","type":"string","usage":` + jsonString(t, stateUsage) + `}` +
		`]}]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_completion_with_a_shell_under_json_is_a_usage_error_document pins
// R11: a valid shell under --json is a usage-error document, not the
// script — exit 2, stderr empty, no script bytes precede the document on
// stdout. The control arm proves the same shell, without --json, still
// writes a non-empty script at exit 0.
func Test_completion_with_a_shell_under_json_is_a_usage_error_document(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			wd := t.TempDir()

			var controlStdout, controlStderr bytes.Buffer
			controlErr := cli.Run(t.Context(), wd, []string{"completion", shell}, nil, &controlStdout, &controlStderr)
			require.NoError(t, controlErr)
			assert.NotEmpty(t, controlStdout.String(), "completion %s without --json must still write a script", shell)
			assert.Empty(t, controlStderr.String())

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, []string{"completion", shell, "--json"}, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), "completion", nil)
			assert.Equal(t, "brief completion: completion prints a shell script; --json does not apply", message)
			assert.Equal(t, "run 'brief completion <bash|zsh|fish|powershell>'", fix)
		})
	}
}
