// This file reaches the unexported run directly to inject newMemSetupSeam
// (mem_internal_test.go), the same rwfs.Mem seam init_internal_test.go
// uses. init_json_test.go's own two error-document cases
// (Test_init_json_refusal_reports_files_changed_false,
// Test_init_json_unknown_host_fix_agrees_with_the_message) stay black-box:
// they decode through decodeErrorDocument/decodeUsageErrorDocument
// (json_refusal_test.go, json_usage_test.go), shared package cli_test
// helpers this white-box package cannot import without duplicating their
// full key-set assertions.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

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
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")

	want := `{"schema":1,"command":"init","ok":true,"exit_code":0,"host":"none","detected_by":null,"dry_run":false,"created":[` +
		`"` + featureRoot + `","` + configPath + `"],"modified":[],"artifacts":[` +
		`{"kind":"config","path":"` + configPath + `","action":"created","detail":null},` +
		`{"kind":"feature-root","path":"` + featureRoot + `","action":"created","detail":null}],"roles_to_add":[],"agents_missing_skill":[]}` + "\n"

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
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

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
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

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
// nothing written.
func Test_init_dry_run_json_reports_empty_created_and_modified(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--dry-run", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

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

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_print_json_is_one_exact_document pins R9's own --print --json
// shape: the common header, then artifacts alone — no host, dry_run,
// created, modified or roles_to_add — every path absolute, action
// "create", body the plain config render, and stderr empty exactly as
// every other success.
func Test_init_print_json_is_one_exact_document(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--print", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

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
	// Green on arrival: --print --json never computed agents_missing_skill
	// before this field existed either. Kept as a regression pin, matching
	// the roles_to_add precedent right above.
	assert.NotContains(t, doc, "agents_missing_skill")

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
