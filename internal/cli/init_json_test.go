package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_json_is_one_exact_document pins initDocument's key order for a
// fresh repository's first run: created lists the feature root then the
// config file — apply's own write order — while artifacts lists the config
// then the feature root — R11's stdout row order — and every path is
// absolute.
func Test_init_json_is_one_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")

	want := `{"schema":1,"command":"init","ok":true,"exit_code":0,"host":"none","dry_run":false,"created":[` +
		jsonString(t, featureRoot) + `,` + jsonString(t, configPath) + `],"modified":[],"artifacts":[` +
		`{"kind":"config","path":` + jsonString(t, configPath) + `,"action":"created","detail":null},` +
		`{"kind":"feature-root","path":` + jsonString(t, featureRoot) + `,"action":"created","detail":null}]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_init_dry_run_json_reports_empty_created_and_modified pins R9's JSON
// shape: dry_run true, both created and modified "[]" (never null), and
// nothing written to disk.
func Test_init_dry_run_json_reports_empty_created_and_modified(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--dry-run", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	var dryRun bool
	require.NoError(t, json.Unmarshal(doc["dry_run"], &dryRun))
	assert.True(t, dryRun)

	var created, modified []string
	require.NoError(t, json.Unmarshal(doc["created"], &created))
	require.NoError(t, json.Unmarshal(doc["modified"], &modified))
	assert.NotNil(t, created)
	assert.NotNil(t, modified)
	assert.Empty(t, created)
	assert.Empty(t, modified)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_json_refusal_reports_files_changed_false pins R3's refusal
// shape for init: kind "refusal", files_changed false (the config never
// parsed, so nothing was ever written), and a message naming the fix.
func Test_init_json_refusal_reports_files_changed_false(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "init")

	assert.Equal(t, "refusal", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.False(t, *decoded.FilesChanged)
	assert.Contains(t, decoded.Fix, "brief init --force")
}
