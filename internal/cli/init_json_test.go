package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/artifact"
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

	want := `{"schema":1,"command":"init","ok":true,"exit_code":0,"host":"none","detected_by":null,"dry_run":false,"created":[` +
		jsonString(t, featureRoot) + `,` + jsonString(t, configPath) + `],"modified":[],"artifacts":[` +
		`{"kind":"config","path":` + jsonString(t, configPath) + `,"action":"created","detail":null},` +
		`{"kind":"feature-root","path":` + jsonString(t, featureRoot) + `,"action":"created","detail":null}],"roles_to_add":[]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_init_json_for_claude_code_carries_plugin_and_hook_kinds pins the
// JSON "kind" vocabulary (R11): "plugin" for the manifest and both skills,
// "hook" for hooks.json, "skill" for the brief-workflow skill, "snippet"
// for CLAUDE.md, "host" echoing "claude-code", and every created path
// listed in write order (feature root, then the four plugin files, the
// brief-workflow skill, then CLAUDE.md, config last) — files only, never a
// directory.
func Test_init_json_for_claude_code_carries_plugin_and_hook_kinds(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		Host      string   `json:"host"`
		Created   []string `json:"created"`
		Artifacts []struct {
			Kind   string `json:"kind"`
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"artifacts"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	assert.Equal(t, "claude-code", doc.Host)

	base := filepath.Join(wd, ".claude", "skills", "brief")
	manifest := filepath.Join(base, ".claude-plugin", "plugin.json")
	start := filepath.Join(base, "skills", "start", "SKILL.md")
	finish := filepath.Join(base, "skills", "finish", "SKILL.md")
	hooks := filepath.Join(base, "hooks", "hooks.json")
	skill := filepath.Join(wd, ".claude", "skills", "brief-workflow", "SKILL.md")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	configPath := filepath.Join(wd, ".brief.yaml")
	claudeMD := filepath.Join(wd, "CLAUDE.md")

	assert.Equal(t, []string{featureRoot, manifest, start, finish, hooks, skill, claudeMD, configPath}, doc.Created)

	kindByPath := map[string]string{}
	for _, a := range doc.Artifacts {
		kindByPath[a.Path] = a.Kind
	}
	assert.Equal(t, "plugin", kindByPath[manifest])
	assert.Equal(t, "plugin", kindByPath[start])
	assert.Equal(t, "plugin", kindByPath[finish])
	assert.Equal(t, "hook", kindByPath[hooks])
	assert.Equal(t, "skill", kindByPath[skill])
	assert.Equal(t, "snippet", kindByPath[claudeMD])
}

// Test_init_with_agents_json_reports_agent_rows_and_roles_to_add pins
// R11's JSON vocabulary for --with-agents: "kind":"agent" for the three
// role-agent files, and "roles_to_add" empty — this run authored the
// bindings itself, so there is nothing left to add — with stderr staying
// empty exactly as every other --json success does.
func Test_init_with_agents_json_reports_agent_rows_and_roles_to_add(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		RolesToAdd []string `json:"roles_to_add"`
		Artifacts  []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"artifacts"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	assert.Equal(t, []string{}, doc.RolesToAdd)

	base := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	kindByPath := map[string]string{}
	for _, a := range doc.Artifacts {
		kindByPath[a.Path] = a.Kind
	}
	assert.Equal(t, "agent", kindByPath[filepath.Join(base, "planner.md")])
	assert.Equal(t, "agent", kindByPath[filepath.Join(base, "implementer.md")])
	assert.Equal(t, "agent", kindByPath[filepath.Join(base, "reviewer.md")])
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

// Test_init_print_json_is_one_exact_document pins R9's own --print --json
// shape: the common header, then artifacts alone — no host, dry_run,
// created, modified or roles_to_add — every path absolute, action
// "create", body the plain config render, and stderr empty exactly as
// every other success.
func Test_init_print_json_is_one_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--print", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.NotContains(t, doc, "host")
	assert.NotContains(t, doc, "dry_run")
	assert.NotContains(t, doc, "created")
	assert.NotContains(t, doc, "modified")
	assert.NotContains(t, doc, "roles_to_add")

	var artifacts []struct {
		Path   string `json:"path"`
		Action string `json:"action"`
		Body   string `json:"body"`
	}
	require.NoError(t, json.Unmarshal(doc["artifacts"], &artifacts))
	require.Len(t, artifacts, 1)
	assert.Equal(t, configPath, artifacts[0].Path)
	assert.Equal(t, "create", artifacts[0].Action)
	assert.Equal(t, string(artifact.ConfigFile()), artifacts[0].Body)
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

// Test_init_json_unknown_host_fix_agrees_with_the_message pins the MINOR
// fix: R8's own unknown-host line trails prose ("to wire it by hand")
// after its "; run '...'" clause, the one shape usageFix's own generic
// extraction cannot recover (it requires the message to end at the closing
// quote — see Test_json_mode_usage_error_fix_stops_at_the_quote_when_the_line_has_trailing_prose
// in json_usage_test.go). error.fix must still name the same --print hint
// the message itself does, not init's own --host invocation.
func Test_init_json_unknown_host_fix_agrees_with_the_message(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "bogus", "--json"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), "init", new(false))

	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`, message)
	assert.Equal(t, "run 'brief init --print' to wire it by hand", fix)
	assert.Contains(t, message, fix)
}
