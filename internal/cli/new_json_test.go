package cli_test

import (
	"bytes"
	"encoding/json"
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
// order, both absolute.
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
		jsonString(t, filepath.Join(featureDir, "STATE.md")) + `]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_new_step_json_names_the_step_and_its_file decodes "new step
// --json"'s document rather than pinning a literal id: wantID is captured
// from a text-mode run against a sibling, equally fresh feature — both are
// their feature's first step, so production's own next-step-number logic
// assigns the same id to each independently. path is the absolute form of
// the single stdout line a text-mode run would have printed, created is
// exactly [path], stderr is empty, and stdout carries the JSON document
// alone — no bare path line ahead of it.
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
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "feature", "step", "path", "created"}, jsonKeys(t, doc))

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
}
