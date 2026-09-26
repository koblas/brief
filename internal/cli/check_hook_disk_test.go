// check --hook's scenarios here stay on real disk: the opt-in gate and
// edited-path resolution have no seam. Usage errors live in check_hook_test.go.

package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hookPayload renders a minimal Claude Code PostToolUse payload naming
// editedPath as tool_input.file_path.
func hookPayload(editedPath string) string {
	return fmt.Sprintf(`{"cwd":"/repo","tool_name":"Edit","tool_input":{"file_path":%q}}`, editedPath)
}

// hookAdditionalContext decodes body — a text-mode "check --hook"'s stdout
// — as claude-code's own hook-context document and returns its
// additionalContext field.
func hookAdditionalContext(t *testing.T, body []byte) string {
	t.Helper()

	var doc struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal(body, &doc))
	assert.Equal(t, "PostToolUse", doc.HookSpecificOutput.HookEventName)

	return doc.HookSpecificOutput.AdditionalContext
}

// The payload names a path inside "alpha", so only alpha's own findings
// reach additionalContext, never beta's.
func Test_check_hook_reports_only_the_feature_containing_the_edited_path_as_additional_context(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))

	betaDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "specification.md"), []byte("# beta\n\nno progress heading\n"), 0o600))

	editedPath := filepath.Join(alphaDir, "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t,
		"brief check: "+filepath.Join("docs", "specifications", "alpha")+": 1 ERROR finding; run 'brief check alpha'",
		hookAdditionalContext(t, stdout.Bytes()))
}

// The payload's file_path is relative, never joined onto wd by the test
// itself, so runCheckHook must resolve it against wd.
func Test_check_hook_resolves_a_relative_edited_path_against_wd(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))

	relativeEditedPath := filepath.Join("docs", "specifications", "alpha", "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(relativeEditedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t,
		"brief check: "+filepath.Join("docs", "specifications", "alpha")+": 1 ERROR finding; run 'brief check alpha'",
		hookAdditionalContext(t, stdout.Bytes()))
}

func Test_check_hook_pluralizes_the_finding_count(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	// No progress heading and no STATE.md: two independent ERROR findings.
	betaDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "specification.md"), []byte("# beta\n\nno progress heading\n"), 0o600))

	editedPath := filepath.Join(betaDir, "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t,
		"brief check: "+filepath.Join("docs", "specifications", "beta")+": 2 ERROR findings; run 'brief check beta'",
		hookAdditionalContext(t, stdout.Bytes()))
}

// This and its control below share one repository, differing only in the
// payload's edited path.
func Test_check_hook_is_silent_when_the_edited_path_is_outside_the_feature_root(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))

	editedPath := filepath.Join(wd, "README.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func Test_check_hook_reports_additional_context_when_the_same_repository_edited_path_is_inside_the_feature_root(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))

	editedPath := filepath.Join(alphaDir, "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())
}

// This and its control below share the same feature tree, differing only
// in whether a ".brief.yaml" file exists.
func Test_check_hook_is_silent_when_no_brief_yaml_is_found(t *testing.T) {
	wd := t.TempDir()

	authDir := filepath.Join(wd, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(authDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(authDir, "specification.md"), []byte("# auth\n\nno progress heading\n"), 0o600))

	editedPath := filepath.Join(authDir, "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func Test_check_hook_reports_additional_context_when_the_same_tree_has_a_brief_yaml(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	authDir := filepath.Join(wd, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(authDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(authDir, "specification.md"), []byte("# auth\n\nno progress heading\n"), 0o600))

	editedPath := filepath.Join(authDir, "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())
}

// This and its control below share the same repository and STATE.md,
// differing only in the step's status.
func Test_check_hook_is_silent_for_a_warn_only_feature(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] first thing"})

	editedPath := filepath.Join(featureDir, "STATE.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func Test_check_hook_reports_additional_context_when_the_same_feature_has_an_open_step(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "open", []string{"- [ ] first thing"})

	editedPath := filepath.Join(featureDir, "STATE.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())
}

// Contract pin: an unplanned step beside the fixture's own ERROR-producing
// step must not move the finding count.
func Test_check_hook_counts_no_error_for_an_unplanned_step(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "open", []string{"- [ ] first thing"})

	noHeading := "---\n" +
		"id: SCENARIO-02\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-02\n\n" +
		"## Scenario\n\nthe acceptance criteria\n\n" +
		"## Not The Checklist\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-02.md"), []byte(noHeading), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-03", "done", nil)

	editedPath := filepath.Join(featureDir, "STATE.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t,
		"brief check: "+filepath.Join("docs", "specifications", "alpha")+": 1 ERROR finding; run 'brief check alpha'",
		hookAdditionalContext(t, stdout.Bytes()))
}

// Covers every payload shape HookPath refuses in an opted-in repository:
// exit 1, not usage-error's exit 2, since the payload is host-supplied.
func Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
	}{
		{name: "empty stdin", stdin: ""},
		{name: "stdin is not JSON", stdin: "not json"},
		{name: "stdin has no tool_input.file_path", stdin: `{"cwd":"/repo"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(c.stdin), &stdout, &stderr)

			require.Error(t, err)
			assert.Equal(t, 1, cli.ExitCode(err))
			assert.Equal(t, "brief check: malformed hook payload on stdin; expected a claude-code PostToolUse payload with tool_input.file_path, nothing was checked\n", stderr.String())
			assert.Empty(t, stdout.String())
		})
	}
}

// Control proving the opt-in gate runs before the payload is parsed: the
// same malformed stdin is silent, exit 0, with no ".brief.yaml".
func Test_check_hook_is_silent_for_malformed_stdin_when_no_brief_yaml_is_found(t *testing.T) {
	wd := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader("not json"), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

// This and its control below share one directory layout, differing only
// in where the ".brief.yaml" file sits relative to the git repository.
func Test_check_hook_is_silent_when_the_config_is_outside_the_enclosing_git_repository(t *testing.T) {
	outer := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outer, ".brief.yaml"), []byte(""), 0o600))

	proj := filepath.Join(outer, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), proj, []string{"check", "--hook", "claude-code"}, strings.NewReader("not json"), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

// Control: the same layout, but the config sits at the git repository's
// own root rather than above it.
func Test_check_hook_honours_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	outer := t.TempDir()

	proj := filepath.Join(outer, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proj, ".brief.yaml"), []byte(""), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), proj, []string{"check", "--hook", "claude-code"}, strings.NewReader("not json"), &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "brief check: malformed hook payload on stdin; expected a claude-code PostToolUse payload with tool_input.file_path, nothing was checked\n", stderr.String())
	assert.Empty(t, stdout.String())
}

func Test_check_hook_refuses_an_invalid_config(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))

	editedPath := filepath.Join(wd, "docs", "specifications", "auth", "specification.md")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader(hookPayload(editedPath)), &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), ".brief.yaml")
	assert.Empty(t, stdout.String())
}
