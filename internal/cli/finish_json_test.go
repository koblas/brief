package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_finish_json_is_one_exact_document is the exact-bytes golden pinning
// finishDocument's key order: newFinishCLIFixture's single step leaves
// nothing else open, so next is JSON null, changed is true, and both paths
// are absolute.
func Test_finish_json_is_one_exact_document(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	want := `{"schema":1,"command":"finish","ok":true,"exit_code":0,"feature":"demo","step":"SCENARIO-01","changed":true,"handoff_path":` +
		jsonString(t, filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")) + `,"state_path":` +
		jsonString(t, filepath.Join(featureDir, "STATE.md")) + `,"next":null}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_finish_json_decodes_next_and_changed_correctly is the decode-level
// table for finishDocument's two result-dependent fields: next renders as a
// string when another step is still open, as JSON null (not an absent key)
// when nothing else is open, and changed renders false on R11's no-op —
// the second finish of the same inputs.
func Test_finish_json_decodes_next_and_changed_correctly(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) (wd string, args []string)
		key   string
		want  string
	}{
		{
			name: "next is a string when another step is open",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()

				wd := newFinishCLIFixtureWithSteps(t,
					finishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
					finishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
				)
				handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
				statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

				return wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "next",
			want: `"SCENARIO-02"`,
		},
		{
			name: "next is null when nothing else is open",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()

				wd := newFinishCLIFixture(t)
				handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
				statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

				return wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "next",
			want: `null`,
		},
		{
			name: "changed is false on the R11 no-op",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()

				wd := newFinishCLIFixture(t)
				handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
				statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
				firstRun := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
				require.NoError(t, cli.Run(t.Context(), wd, firstRun, nil, &bytes.Buffer{}, &bytes.Buffer{}))

				return wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "changed",
			want: `false`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd, args := tc.setup(t)
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, args, nil, &stdout, &stderr)

			require.NoError(t, err)

			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
			assert.JSONEq(t, tc.want, string(doc[tc.key]))
		})
	}
}

// Test_finish_json_refusal_is_unchanged confirms a refusal under --json
// still renders R3's error document, not a finishDocument: refusing a step
// that has no on-disk file, the same shape json_refusal_test.go already
// covers for other commands.
func Test_finish_json_refusal_is_unchanged(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-99", "--handoff", handoffPath, "--state", statePath, "--json"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Empty(t, stderr.String())

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "error"}, jsonKeys(t, doc))
}
