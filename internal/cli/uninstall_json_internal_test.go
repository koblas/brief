// This file reaches the unexported run directly to inject newMemSetupSeam
// (mem_internal_test.go), the same rwfs.Mem seam uninstall_internal_test.go
// uses. uninstall_json_test.go's own remaining case
// (Test_uninstall_json_failure_reports_files_changed_false) stays
// black-box and disk: a real chmod'd, unwritable wd forces os.Remove to
// fail, and it decodes through decodeErrorDocument (json_refusal_test.go),
// a shared package cli_test helper this white-box package cannot import.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_uninstall_json_is_one_exact_document pins uninstallDocument's exact
// shape and key order for the happy path: removed names the config's own
// absolute path, created and modified are both "[]" — uninstall never
// writes — and artifacts carries the one "removed" row.
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

// Test_uninstall_json_for_claude_code_removes_plugin_and_hook_files pins
// the same "kind" vocabulary on removal: "snippet" for CLAUDE.md, "skill"
// for the brief-workflow skill, "hook" for hooks.json, "plugin" for the
// rest, "host" echoing "claude-code", and removed naming files only, in
// removal order (CLAUDE.md first) — never the pruned, now-empty plugin
// directory.
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

// Test_uninstall_json_nothing_installed_is_an_empty_document pins the
// zero-artifact shape: artifacts "[]", every path slice "[]", and no
// stderr line at all — R11's "nothing installed" text-mode line has no
// JSON equivalent to write.
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

// Test_uninstall_json_dry_run_reports_empty_removed pins R9's JSON shape:
// dry_run true, removed "[]" even though the plan's own artifact row says
// "removed", and nothing written.
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
