package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodedError is the JSON shape of a --json error document's "error" member.
type decodedError struct {
	Kind         string  `json:"kind"`
	Message      string  `json:"message"`
	Path         *string `json:"path"`
	Line         *int    `json:"line"`
	Problem      *string `json:"problem"`
	Fix          string  `json:"fix"`
	FilesChanged *bool   `json:"files_changed"`
}

// decodeErrorDocument asserts stdout holds exactly one --json error
// document for wantCommand and returns its error object.
func decodeErrorDocument(t *testing.T, stdout []byte, wantCommand string) decodedError {
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
	assert.Equal(t, 1, exitCode)

	var errObj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["error"], &errObj))
	assert.ElementsMatch(t, []string{"kind", "message", "path", "line", "problem", "fix", "files_changed"}, jsonKeys(t, errObj))

	var decoded decodedError
	require.NoError(t, json.Unmarshal(doc["error"], &decoded))

	return decoded
}

// jsonString marshals s the way assert.Equal compares it, for building a
// golden literal around a dynamically computed value.
func jsonString(t *testing.T, s string) string {
	t.Helper()

	b, err := json.Marshal(s)
	require.NoError(t, err)

	return string(b)
}

// wantMessage and wantProblem are captured from a real run, not hardcoded:
// the OS-native text they embed isn't this test's own contract.
func Test_json_mode_renders_a_refusal_as_one_document(t *testing.T) {
	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	statePath := filepath.Join(featureDir, "STATE.md")
	require.NoError(t, os.Remove(statePath))

	_, openErr := os.Open(statePath)
	var pathErr *fs.PathError
	require.ErrorAs(t, openErr, &pathErr)
	wantProblem := pathErr.Err.Error()

	var textStdout, textStderr bytes.Buffer
	textErr := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &textStdout, &textStderr)
	assert.Equal(t, 1, cli.ExitCode(textErr))
	wantMessage := strings.TrimRight(textStderr.String(), "\n")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"start","ok":false,"exit_code":1,` +
		`"error":{"kind":"refusal",` +
		`"message":` + jsonString(t, wantMessage) + `,` +
		`"path":` + jsonString(t, statePath) + `,` +
		`"line":null,` +
		`"problem":` + jsonString(t, wantProblem) + `,` +
		`"fix":"make it readable and re-run",` +
		`"files_changed":null}}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// noStdin is a refusalCase's newStdin for a row that never reads stdin.
func noStdin() io.Reader { return nil }

// unclosedFenceLine is the line the shared unclosed-fence fixtures open their fence on.
const unclosedFenceLine = 3

// refusalCase is one refusal-matrix row's fixture and expected values.
type refusalCase struct {
	wd       string
	args     []string
	textArgs []string
	newStdin func() io.Reader
	wantKind string
	wantPath *string
	wantLine *int
	wantFix  string
}

// refusalMatrixRow is one refusal-matrix row: setup builds its fixture
// and argv; command and filesChanged are static expectations.
type refusalMatrixRow struct {
	name         string
	setup        func(t *testing.T) refusalCase
	command      string
	filesChanged *bool
}

// newStartEmptyFeatureCase is the "start empty feature" row: an empty
// feature name refuses as not-found, not a raw open failure.
func newStartEmptyFeatureCase(t *testing.T) refusalCase {
	t.Helper()

	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications")

	return refusalCase{
		wd:       wd,
		args:     []string{"start", "--json", ""},
		textArgs: []string{"start", ""},
		newStdin: noStdin,
		wantKind: "refusal",
		wantPath: &featureDir,
		wantFix:  "known: none; run 'brief new feature ' to create it",
	}
}

// refusalMatrixRows lists one representative failure per command, covering
// every refusal classification: config, not-found, scaffold, assemble and
// a generic failure.
func refusalMatrixRows(falseVal *bool) []refusalMatrixRow {
	return []refusalMatrixRow{
		{
			name: "start missing STATE.md",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := newStartFixture(t, "open")
				statePath := filepath.Join(wd, "docs", "specifications", "demo", "STATE.md")
				require.NoError(t, os.Remove(statePath))

				return refusalCase{
					wd:       wd,
					args:     []string{"start", "--json", "demo"},
					textArgs: []string{"start", "demo"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &statePath,
					wantFix:  "make it readable and re-run",
				}
			},
			command: "start",
		},
		{
			name: "start unknown feature",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				featureDir := filepath.Join(wd, "docs", "specifications")

				return refusalCase{
					wd:       wd,
					args:     []string{"start", "--json", "ghost"},
					textArgs: []string{"start", "ghost"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &featureDir,
					wantFix:  "known: none; run 'brief new feature ghost' to create it",
				}
			},
			command: "start",
		},
		{
			name:    "start empty feature",
			setup:   newStartEmptyFeatureCase,
			command: "start",
		},
		{
			name: "status invalid config",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				configPath := filepath.Join(wd, ".brief.yaml")
				require.NoError(t, os.WriteFile(configPath, []byte("not-a-real-key: true\n"), 0o600))

				return refusalCase{
					wd:       wd,
					args:     []string{"status", "--json"},
					textArgs: []string{"status"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &configPath,
					wantFix:  "correct the value, or delete the key to use its default",
				}
			},
			command: "status",
		},
		{
			name: "check invalid config",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				configPath := filepath.Join(wd, ".brief.yaml")
				require.NoError(t, os.WriteFile(configPath, []byte("not-a-real-key: true\n"), 0o600))

				return refusalCase{
					wd:       wd,
					args:     []string{"check", "--json"},
					textArgs: []string{"check"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &configPath,
					wantFix:  "correct the value, or delete the key to use its default",
				}
			},
			command: "check",
		},
		{
			name: "check unknown feature",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				featureDir := filepath.Join(wd, "docs", "specifications")

				return refusalCase{
					wd:       wd,
					args:     []string{"check", "--json", "ghost"},
					textArgs: []string{"check", "ghost"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &featureDir,
					wantFix:  "known: none; run 'brief new feature ghost' to create it",
				}
			},
			command: "check",
		},
		{
			name: "finish unknown step",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := newFinishCLIFixture(t)
				featureDir := filepath.Join(wd, "docs", "specifications", "demo")
				handoffPath := writeInput(t, "handoff.md", "h\n")
				statePath := writeInput(t, "state.md",
					"## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

				return refusalCase{
					wd:       wd,
					args:     []string{"finish", "demo", "SCENARIO-99", "--handoff", handoffPath, "--state", statePath, "--json"},
					textArgs: []string{"finish", "demo", "SCENARIO-99", "--handoff", handoffPath, "--state", statePath},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &featureDir,
					wantFix:  "known: SCENARIO-01",
				}
			},
			command:      "finish",
			filesChanged: falseVal,
		},
		{
			// --state is relative to t.Chdir(wd), matched by the path below.
			name: "finish --state relative path fails conformance",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := newFinishCLIFixture(t)
				handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
				require.NoError(t, os.WriteFile(filepath.Join(wd, "state.md"),
					[]byte("## Binding decisions\n\n```\nunterminated\n"), 0o600))
				t.Chdir(wd)
				wantPath := filepath.Join(wd, "state.md")
				wantLine := unclosedFenceLine // this fixture's unclosed fence

				return refusalCase{
					wd:       wd,
					args:     []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "state.md", "--json"},
					textArgs: []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "state.md"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &wantPath,
					wantLine: &wantLine,
					wantFix:  "close the fence, or remove the unmatched delimiter, and retry",
				}
			},
			command:      "finish",
			filesChanged: falseVal,
		},
		{
			name: "finish --state - fails conformance",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := newFinishCLIFixture(t)
				handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
				wantLine := unclosedFenceLine // the piped body's unclosed fence

				return refusalCase{
					wd:       wd,
					args:     []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-", "--json"},
					textArgs: []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-"},
					newStdin: func() io.Reader { return strings.NewReader("Repro:\n\n```bash\ngo test ./...\n") },
					wantKind: "refusal",
					wantLine: &wantLine,
					wantFix:  "close the fence, or remove the unmatched delimiter, and retry",
				}
			},
			command:      "finish",
			filesChanged: falseVal,
		},
		{
			name: "finish --handoff nonexistent file",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := newFinishCLIFixture(t)
				missingHandoff := filepath.Join(t.TempDir(), "does-not-exist.md")
				statePath := writeInput(t, "state.md",
					"## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

				return refusalCase{
					wd:       wd,
					args:     []string{"finish", "demo", "SCENARIO-01", "--handoff", missingHandoff, "--state", statePath, "--json"},
					textArgs: []string{"finish", "demo", "SCENARIO-01", "--handoff", missingHandoff, "--state", statePath},
					newStdin: noStdin,
					wantKind: "failure",
					wantFix:  "resolve the problem, then run 'brief finish <feature> <step> --handoff <path> --state <path>' again",
				}
			},
			command:      "finish",
			filesChanged: falseVal,
		},
		{
			name: "new feature on an existing feature",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				var discard bytes.Buffer
				require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "demo"}, nil, &discard, &discard))
				featurePath := filepath.Join(wd, "docs", "specifications", "demo")

				return refusalCase{
					wd:       wd,
					args:     []string{"new", "feature", "demo", "--json"},
					textArgs: []string{"new", "feature", "demo"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &featurePath,
					wantFix:  "run 'brief new step demo' to add a step to it, or choose a different name",
				}
			},
			command:      "new feature",
			filesChanged: falseVal,
		},
		{
			name: "new step on an unknown feature",
			setup: func(t *testing.T) refusalCase {
				t.Helper()

				wd := t.TempDir()
				featureDir := filepath.Join(wd, "docs", "specifications")

				return refusalCase{
					wd:       wd,
					args:     []string{"new", "step", "ghost", "--json"},
					textArgs: []string{"new", "step", "ghost"},
					newStdin: noStdin,
					wantKind: "refusal",
					wantPath: &featureDir,
					wantFix:  "known: none; run 'brief new feature ghost' to create it",
				}
			},
			command:      "new step",
			filesChanged: falseVal,
		},
		{
			name:         "new feature invalid config value",
			setup:        newFeatureInvalidConfigCase,
			command:      "new feature",
			filesChanged: falseVal,
		},
		{
			name:         "finish invalid config value",
			setup:        newFinishInvalidConfigCase,
			command:      "finish",
			filesChanged: falseVal,
		},
	}
}

// newFeatureInvalidConfigCase is the "new feature invalid config value"
// row: an invalid .brief.yaml refuses at load before scaffold runs.
func newFeatureInvalidConfigCase(t *testing.T) refusalCase {
	t.Helper()

	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("handoff-cap-lines: 0\n"), 0o600))

	return refusalCase{
		wd:       wd,
		args:     []string{"new", "feature", "payments", "--json"},
		textArgs: []string{"new", "feature", "payments"},
		newStdin: noStdin,
		wantKind: "refusal",
		wantPath: &configPath,
		wantFix:  "correct the value, or delete the key to use its default",
	}
}

// newFinishInvalidConfigCase is the "finish invalid config value" row:
// --handoff and --state name real, readable files so resolveRoot's own
// refusal is reached first.
func newFinishInvalidConfigCase(t *testing.T) refusalCase {
	t.Helper()

	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("handoff-cap-lines: 0\n"), 0o600))
	handoffPath := writeInput(t, "handoff.md", "h\n")
	statePath := writeInput(t, "state.md", "s\n")

	return refusalCase{
		wd:       wd,
		args:     []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"},
		textArgs: []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath},
		newStdin: noStdin,
		wantKind: "refusal",
		wantPath: &configPath,
		wantFix:  "correct the value, or delete the key to use its default",
	}
}

// fix's containment in message isn't asserted here: a generic failure's
// fix is a --json-only fallback absent from the text-mode line.
func Test_json_mode_refusal_matrix(t *testing.T) {
	falseVal := false

	for _, tt := range refusalMatrixRows(&falseVal) {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup(t)

			var textStdout, textStderr bytes.Buffer
			textErr := cli.Run(t.Context(), c.wd, c.textArgs, c.newStdin(), &textStdout, &textStderr)
			assert.Equal(t, 1, cli.ExitCode(textErr))
			wantMessage := strings.TrimRight(textStderr.String(), "\n")

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), c.wd, c.args, c.newStdin(), &stdout, &stderr)

			assert.Equal(t, 1, cli.ExitCode(err))
			assert.Empty(t, stderr.String())

			got := decodeErrorDocument(t, stdout.Bytes(), tt.command)

			assert.Equal(t, c.wantKind, got.Kind)
			assert.Equal(t, wantMessage, got.Message)
			assert.Equal(t, c.wantPath, got.Path)
			assert.Equal(t, c.wantLine, got.Line)
			require.NotNil(t, got.Problem)
			assert.NotEmpty(t, *got.Problem)
			assert.Contains(t, got.Message, *got.Problem)
			assert.Equal(t, c.wantFix, got.Fix)
			assert.Equal(t, tt.filesChanged, got.FilesChanged)
		})
	}
}

// The control arm — the same fixture, an unknown feature instead — proves
// --json still renders an error document for a refusal.
func Test_json_mode_check_findings_are_not_an_error_document(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.NotContains(t, jsonKeys(t, doc), "error")

	var ok bool
	require.NoError(t, json.Unmarshal(doc["ok"], &ok))
	assert.False(t, ok)

	var exitCode int
	require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
	assert.Equal(t, 1, exitCode)

	var counts struct {
		Error int `json:"error"`
	}
	require.NoError(t, json.Unmarshal(doc["counts"], &counts))
	assert.Positive(t, counts.Error)

	var controlStdout, controlStderr bytes.Buffer
	controlErr := cli.Run(t.Context(), wd, []string{"check", "--json", "ghost"}, nil, &controlStdout, &controlStderr)

	assert.Equal(t, 1, cli.ExitCode(controlErr))
	assert.Empty(t, controlStderr.String())
	decodeErrorDocument(t, controlStdout.Bytes(), "check")
}

// The control arm is the same fixture without --json: text mode still
// writes the malformed line and summary to stderr.
func Test_json_mode_status_with_a_malformed_feature_is_a_success_document(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.NotContains(t, jsonKeys(t, doc), "error")

	var ok bool
	require.NoError(t, json.Unmarshal(doc["ok"], &ok))
	assert.True(t, ok)

	var exitCode int
	require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
	assert.Equal(t, 0, exitCode)

	var textStdout, textStderr bytes.Buffer
	textErr := cli.Run(t.Context(), wd, []string{"status"}, nil, &textStdout, &textStderr)

	require.NoError(t, textErr)
	assert.NotEmpty(t, textStderr.String())
}
