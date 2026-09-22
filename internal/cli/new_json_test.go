package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_new_feature_json_is_one_exact_document is the exact-bytes golden
// pinning newDocument's key order for "new feature": step is null (no call
// creates a feature and a step together), path names the feature
// directory, created lists the specification and the state file in write
// order, both absolute, and modified is empty — NewFeature creates both
// its files fresh, rewriting nothing that already existed.
func Test_new_feature_json_is_one_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	featureDir := filepath.Join(wd, "docs", "specifications", "payments")
	want := `{"schema":1,"command":"new feature","ok":true,"exit_code":0,"feature":"payments","step":null,"path":` +
		jsonString(t, featureDir) + `,"created":[` +
		jsonString(t, filepath.Join(featureDir, "specification.md")) + `,` +
		jsonString(t, filepath.Join(featureDir, "STATE.md")) + `],"modified":[]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_new_step_json_names_the_step_and_its_file decodes "new step
// --json"'s document rather than pinning a literal id: wantID is captured
// from a text-mode run against a sibling, equally fresh feature — both are
// their feature's first step, so production's own next-step-number logic
// assigns the same id to each independently. path is the absolute form of
// the single stdout line a text-mode run would have printed, created is
// exactly [path], modified is exactly [the feature's specification.md] —
// the progress entry this call appends — stderr is empty, and stdout
// carries the JSON document alone — no bare path line ahead of it.
func Test_new_step_json_names_the_step_and_its_file(t *testing.T) {
	wdText := t.TempDir()
	require.NoError(t, cli.Run(t.Context(), wdText, []string{"new", "feature", "alpha"}, nil, &bytes.Buffer{}, &bytes.Buffer{}))

	var textStdout, textStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wdText, []string{"new", "step", "alpha"}, nil, &textStdout, &textStderr))
	relStepPath := strings.TrimSuffix(textStdout.String(), "\n")
	wantID := strings.TrimSuffix(filepath.Base(relStepPath), filepath.Ext(relStepPath))

	wd := t.TempDir()
	require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "beta"}, nil, &bytes.Buffer{}, &bytes.Buffer{}))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"new", "step", "beta", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, 1, strings.Count(stdout.String(), "\n"), "stdout must carry the JSON document alone")

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "feature", "step", "path", "created", "modified"}, jsonKeys(t, doc))

	var step string
	require.NoError(t, json.Unmarshal(doc["step"], &step))
	assert.Equal(t, wantID, step)

	wantPath := filepath.Join(wd, "docs", "specifications", "beta", step+".md")

	var path string
	require.NoError(t, json.Unmarshal(doc["path"], &path))
	assert.Equal(t, wantPath, path)

	var created []string
	require.NoError(t, json.Unmarshal(doc["created"], &created))
	assert.Equal(t, []string{wantPath}, created)

	wantSpecPath := filepath.Join(wd, "docs", "specifications", "beta", "specification.md")

	var modified []string
	require.NoError(t, json.Unmarshal(doc["modified"], &modified))
	assert.Equal(t, []string{wantSpecPath}, modified)
}

// Test_new_feature_json_files_changed_is_true_when_the_state_write_fails
// exercises "new feature"'s own write-site gap at the CLI boundary: a
// .brief.yaml naming a state-file over 255 bytes long passes R1's own
// validation (it is a plain file name with no path separator) but the
// filesystem itself refuses it with ENAMETOOLONG, so the specification's
// write lands first and the state write fails after it. files_changed
// must report true, not false, since the specification did land. The
// control arm reads that file back: it still carries the specification
// skeleton, proving files_changed's "true" is not vacuous.
func Test_new_feature_json_files_changed_is_true_when_the_state_write_fails(t *testing.T) {
	wd := t.TempDir()
	longStateFile := strings.Repeat("A", 256) + ".md"
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"),
		[]byte("state-file: \""+longStateFile+"\"\n"), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments", "--json"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	got := decodeErrorDocument(t, stdout.Bytes(), "new feature")
	assert.Equal(t, "failure", got.Kind)
	require.NotNil(t, got.FilesChanged)
	assert.True(t, *got.FilesChanged)

	featureDir := filepath.Join(wd, "docs", "specifications", "payments")
	gotSpec, readErr := os.ReadFile(filepath.Join(featureDir, "specification.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# payments\n\n## BDD Acceptance Progress\n", string(gotSpec),
		"the specification write ahead of the blocked state write must actually have landed")
}
