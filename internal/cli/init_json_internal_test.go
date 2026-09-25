// White-box: reaches run directly to inject newMemSetupSeam, the same
// rwfs.Mem seam init_internal_test.go uses.

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

// created lists the feature root then the config file (apply's write
// order), while artifacts lists the config then the feature root (the
// stdout row order); every path is absolute.
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

// created lists every path in write order: feature root, then the four
// plugin files, the brief-workflow skill, then CLAUDE.md, config last.
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
