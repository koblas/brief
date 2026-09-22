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
	// alpha carries no step files at all, so check counts it in flight
	// (assemble.checkStepFindings) and its missing STATE.md fires
	// RuleStateMissing (C2) as this single ERROR finding.
	assert.Equal(t,
		"brief check: "+filepath.Join("docs", "specifications", "alpha")+": 1 ERROR finding; run 'brief check alpha'",
		hookAdditionalContext(t, stdout.Bytes()))
}

// Test_check_hook_resolves_a_relative_edited_path_against_wd pins the
// contract line "a relative file_path resolves against wd": the payload
// below names its file_path relative, never joined onto wd by the test
// itself, so a dropped join in runCheckHook would make FeatureContaining
// reject it (proven separately by
// Test_FeatureContaining_ReturnsFalseForARelativePath) and this test
// would see silent, empty stdout instead of the finding below.
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

	// No progress heading (RuleHeading) and no STATE.md (RuleStateMissing):
	// two independent ERROR findings, with no step files needed to make
	// the feature "in flight".
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

// Test_check_hook_is_silent_when_the_edited_path_is_outside_the_feature_root
// and its control below share one repository and one .brief.yaml,
// differing only in the payload's edited path — the one variable the
// "outside the feature root" claim depends on.
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

// Test_check_hook_is_silent_when_no_brief_yaml_is_found and its control
// below share the same feature tree — carrying an ERROR finding at the
// default feature directory, so config.Resolve's own defaults WOULD find
// it — differing only in whether a ".brief.yaml" file exists. The claim is
// that the opt-in gate is Locate, not Resolve's silent default.
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

// Test_check_hook_is_silent_for_a_warn_only_feature and its control below
// share the same repository and STATE.md, differing only in the step's own
// status — the one variable that turns the feature's single finding from
// WARN (done) to ERROR (open).
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

// Test_check_hook_usage_errors covers every text-mode usage-error shape
// "check --hook" reports before it ever reads the repository: exit 2, the
// pinned stderr line, empty stdout. Each case discriminates on its own
// mutation: the host it validates, or the flag combination it rejects. A
// malformed stdin payload is not a case here: it is no longer a usage
// error (exit 2) at all — see
// Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1 and
// Test_check_hook_is_silent_for_malformed_stdin_when_no_brief_yaml_is_found
// below for its own two shapes. "--hook with --json" is not a case here
// either: R5 routes every usage error to stdout as a JSON document under
// --json, a different assertion shape entirely — covered by
// Test_check_hook_with_json_reports_the_usage_error_as_json below.
func Test_check_hook_usage_errors(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		stdin      string
		wantStderr string
	}{
		{
			name:       "unknown host",
			args:       []string{"check", "--hook", "bogus"},
			stdin:      "",
			wantStderr: `brief check: unknown host "bogus"; expected one of: claude-code; run 'brief check --hook claude-code'` + "\n",
		},
		{
			name:       "--hook with a positional feature",
			args:       []string{"check", "auth", "--hook", "claude-code"},
			stdin:      "",
			wantStderr: "brief check: --hook takes no feature argument; run 'brief check --hook claude-code'\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, c.args, strings.NewReader(c.stdin), &stdout, &stderr)

			require.Error(t, err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Equal(t, c.wantStderr, stderr.String())
			assert.Empty(t, stdout.String())
		})
	}
}

// Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1 covers
// every payload shape host.ClaudeCode's own HookPath refuses, inside a
// repository that opted in (a ".brief.yaml" exists at wd): exit 1, not
// usage-error's exit 2 — the payload is host-supplied, not user-typed —
// one stderr line, empty stdout.
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
			assert.Equal(t, "brief check: malformed hook payload on stdin; run 'brief check --hook claude-code'\n", stderr.String())
			assert.Empty(t, stdout.String())
		})
	}
}

// Test_check_hook_is_silent_for_malformed_stdin_when_no_brief_yaml_is_found
// is the control proving the opt-in gate runs before the payload is ever
// parsed: the very same malformed stdin that
// Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1 reports as
// an error is silent, exit 0, in a repository with no ".brief.yaml" — the
// one variable that changes between the two is whether the repository
// opted in, never the payload.
func Test_check_hook_is_silent_for_malformed_stdin_when_no_brief_yaml_is_found(t *testing.T) {
	wd := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"}, strings.NewReader("not json"), &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

// Test_check_hook_is_silent_when_the_config_is_outside_the_enclosing_git_repository
// and its control below share one directory layout — a git repository at
// "proj" nested inside an ancestor that itself carries a ".brief.yaml" —
// differing only in where that config file sits: the gate must stay bound
// to the enclosing git repository (config.LocateInRepo), never Locate's
// unbounded walk, so a config living above "proj" is invisible even to a
// malformed payload, exactly as if no repository had opted in at all.
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

// Test_check_hook_honours_a_config_at_the_enclosing_git_repository_root is
// the control: the very same layout, but the config sits at "proj" itself —
// the git repository's own root — rather than above it, so the gate
// proceeds and a malformed payload reports exit 1, the same as
// Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1.
func Test_check_hook_honours_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	outer := t.TempDir()

	proj := filepath.Join(outer, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proj, ".brief.yaml"), []byte(""), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), proj, []string{"check", "--hook", "claude-code"}, strings.NewReader("not json"), &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "brief check: malformed hook payload on stdin; run 'brief check --hook claude-code'\n", stderr.String())
	assert.Empty(t, stdout.String())
}

func Test_check_hook_with_json_reports_the_usage_error_as_json(t *testing.T) {
	wd := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code", "--json"}, strings.NewReader(""), &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc struct {
		OK    bool `json:"ok"`
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.False(t, doc.OK)
	assert.Equal(t, "usage", doc.Error.Kind)
	assert.Equal(t, "brief check: --hook and --json cannot be combined; run 'brief check --hook claude-code'", doc.Error.Message)
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
