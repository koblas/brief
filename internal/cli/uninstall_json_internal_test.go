// This file reaches the unexported run directly to inject newMemSetupSeam,
// the same rwfs.Mem seam uninstall_internal_test.go uses.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_uninstall_json_is_one_exact_document(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":false,` +
		`"created":[],"modified":[],"removed":["` + configPath + `"],"artifacts":[` +
		`{"kind":"config","path":"` + configPath + `","action":"removed","detail":null}]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

func Test_uninstall_json_for_claude_code_removes_plugin_and_hook_files(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		Host      string   `json:"host"`
		Removed   []string `json:"removed"`
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
	configPath := filepath.Join(wd, ".brief.yaml")
	claudeMD := filepath.Join(wd, "CLAUDE.md")

	assert.Equal(t, []string{claudeMD, skill, hooks, finish, start, manifest, configPath}, doc.Removed)

	kindByPath := map[string]string{}
	for _, a := range doc.Artifacts {
		kindByPath[a.Path] = a.Kind
	}
	assert.Equal(t, "hook", kindByPath[hooks])
	assert.Equal(t, "plugin", kindByPath[manifest])
	assert.Equal(t, "skill", kindByPath[skill])
	assert.Equal(t, "snippet", kindByPath[claudeMD])
}

func Test_uninstall_json_nothing_installed_is_an_empty_document(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":false,` +
		`"created":[],"modified":[],"removed":[],"artifacts":[]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

func Test_uninstall_json_dry_run_reports_empty_removed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	configPath := filepath.Join(wd, ".brief.yaml")

	want := `{"schema":1,"command":"uninstall","ok":true,"exit_code":0,"host":"none","dry_run":true,` +
		`"created":[],"modified":[],"removed":[],"artifacts":[` +
		`{"kind":"config","path":"` + configPath + `","action":"removed","detail":null}]}` + "\n"

	assert.Equal(t, want, stdout.String())

	_, statErr := mem.Stat(memKey(configPath))
	assert.NoError(t, statErr)
}
