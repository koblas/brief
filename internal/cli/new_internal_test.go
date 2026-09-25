// new's scenarios run against rwfs.Mem here, reaching run() directly.
// new_json_disk_test.go keeps only the real-filesystem ENAMETOOLONG case.

package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNewMem runs "new" (or the args given) against tree through run()'s
// own withRootFS seam.
func runNewMem(t *testing.T, tree *memTree, args []string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, &stdout, &stderr, noBuildInfo, withRootFS(tree.mem()))

	return stdout.String(), stderr.String(), err
}

// memOneLine asserts s holds exactly one newline-terminated line and
// returns it without the trailing newline.
func memOneLine(t *testing.T, s string) string {
	t.Helper()

	require.Equal(t, 1, strings.Count(s, "\n"), "expected exactly one line in %q", s)

	return strings.TrimSuffix(s, "\n")
}

func Test_creates_the_step_and_prints_its_path_mem(t *testing.T) {
	tree := newMemTree(memRoot)
	mem := tree.mem()

	var discard strings.Builder
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "feature", "payments"}, nil, &discard, &discard, noBuildInfo, withRootFS(mem)))

	var stdout, stderr strings.Builder
	err := run(t.Context(), memRoot, []string{"new", "step", "payments"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.NoError(t, err)
	assert.Equal(t,
		"brief new step: created SCENARIO-01 in payments and added it to docs/specifications/payments/specification.md; fill in its acceptance criteria and checklist, then 'brief start payments'\n",
		stderr.String())
	assert.Equal(t, "docs/specifications/payments/SCENARIO-01.md\n", stdout.String())

	// Readback proves the progress entry was really appended, not merely reported.
	specPath := memKey(filepath.Join(memRoot, "docs", "specifications", "payments", "specification.md"))
	gotSpec, err := mem.ReadFile(specPath)
	require.NoError(t, err)
	assert.Contains(t, string(gotSpec), "- [ ] SCENARIO-01")
}

func Test_new_feature_refuses_an_existing_feature_mem(t *testing.T) {
	mem := newMemTree(memRoot).mem()

	var discard strings.Builder
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "feature", "payments"}, nil, &discard, &discard, noBuildInfo, withRootFS(mem)))

	var stdout, stderr strings.Builder
	err := run(t.Context(), memRoot, []string{"new", "feature", "payments"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.ErrorIs(t, err, scaffold.ErrFeatureExists)
	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	memOneLine(t, stderr.String())

	specPath := memKey(filepath.Join(memRoot, "docs", "specifications", "payments", "specification.md"))
	gotSpec, readErr := mem.ReadFile(specPath)
	require.NoError(t, readErr)
	assert.Equal(t, "# payments\n\n## BDD Acceptance Progress\n", string(gotSpec),
		"the second call must not have touched the first call's own specification.md")
}

func Test_returns_a_usage_error_when_no_feature_is_given_for_step_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "step"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief new step: no feature given; run 'brief new step <feature>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_there_are_too_many_arguments_for_step_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "step", "a", "b"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief new step: too many arguments; run 'brief new step <feature>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_a_flag_is_not_defined_for_step_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "step", "-x", "p"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief new step: unknown shorthand flag: 'x' in -x; run 'brief new step <feature>'", memOneLine(t, stderr))
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_new_step_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "step", "--help"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.NotEmpty(t, stdout)
}

// The error still satisfies errors.Is(err, scaffold.ErrNoSuchFeature): the
// enriched wrapper wraps the sentinel rather than replacing it.
func Test_refuses_on_one_line_for_an_unknown_feature_for_step_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "step", "payments"})

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.Empty(t, stdout)
	assert.Equal(t, 1, ExitCode(err))
	memOneLine(t, stderr)
}

func Test_refuses_on_one_line_when_the_specification_has_no_progress_heading_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "payments")
	tree.
		file(filepath.Join(featureDir, "specification.md"), "# payments\n\nno progress list here.\n").
		file(filepath.Join(featureDir, "STATE.md"), "")

	stdout, stderr, err := runNewMem(t, tree, []string{"new", "step", "payments"})

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)
	assert.Empty(t, stdout)
	assert.Equal(t, 1, ExitCode(err))

	line := memOneLine(t, stderr)
	assert.Contains(t, line, filepath.Join("docs", "specifications", "payments", "specification.md"))
	assert.Contains(t, line, "## BDD Acceptance Progress")
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}

// The feature is built directly through scaffold, bypassing run()'s config
// resolve, since the ancestor config is itself invalid; both share one *rwfs.Mem.
func Test_refuses_on_one_line_for_an_invalid_step_file_pattern_from_an_ancestor_config_mem(t *testing.T) {
	wd := filepath.Join(memRoot, "a", "b")
	tree := newMemTree(memRoot, wd)
	tree.file(filepath.Join(memRoot, ".brief.yaml"), "step-file-pattern: \"SCENARIO-%s.md\"\n")
	mem := tree.mem()

	_, err := scaffold.NewServer(config.Default(), memRoot, scaffold.WithFS(mem)).NewFeature(t.Context(), "payments")
	require.NoError(t, err)

	var stdout, stderr strings.Builder
	err = run(t.Context(), wd, []string{"new", "step", "payments"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, ExitCode(err))

	line := memOneLine(t, stderr.String())
	assert.Contains(t, line, "step-file-pattern")
	assert.Contains(t, line, "SCENARIO-%s.md")
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}

func Test_new_feature_json_is_one_exact_document_mem(t *testing.T) {
	stdout, stderr, err := runNewMem(t, newMemTree(memRoot), []string{"new", "feature", "payments", "--json"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stderr)

	featureDir := filepath.Join(memRoot, "docs", "specifications", "payments")
	want := `{"schema":1,"command":"new feature","ok":true,"exit_code":0,"feature":"payments","step":null,"path":` +
		memJSONString(t, featureDir) + `,"created":[` +
		memJSONString(t, filepath.Join(featureDir, "specification.md")) + `,` +
		memJSONString(t, filepath.Join(featureDir, "STATE.md")) + `],"modified":[]}` + "\n"

	assert.Equal(t, want, stdout)
}

// wantID is captured from a text-mode run against a sibling, equally
// fresh feature, not a hardcoded literal.
func Test_new_step_json_names_the_step_and_its_file_mem(t *testing.T) {
	textTree := newMemTree(memRoot)
	textMem := textTree.mem()
	var discard strings.Builder
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "feature", "alpha"}, nil, &discard, &discard, noBuildInfo, withRootFS(textMem)))

	var textStdout strings.Builder
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "step", "alpha"}, nil, &textStdout, &discard, noBuildInfo, withRootFS(textMem)))
	relStepPath := strings.TrimSuffix(textStdout.String(), "\n")
	wantID := strings.TrimSuffix(filepath.Base(relStepPath), filepath.Ext(relStepPath))

	tree := newMemTree(memRoot)
	mem := tree.mem()
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "feature", "beta"}, nil, &discard, &discard, noBuildInfo, withRootFS(mem)))

	var stdout, stderr strings.Builder
	err := run(t.Context(), memRoot, []string{"new", "step", "beta", "--json"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, 1, strings.Count(stdout.String(), "\n"), "stdout must carry the JSON document alone")

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout.String()), &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "feature", "step", "path", "created", "modified"}, memJSONKeys(doc))

	var step string
	require.NoError(t, json.Unmarshal(doc["step"], &step))
	assert.Equal(t, wantID, step)

	wantPath := filepath.Join(memRoot, "docs", "specifications", "beta", step+".md")

	var path string
	require.NoError(t, json.Unmarshal(doc["path"], &path))
	assert.Equal(t, wantPath, path)

	var created []string
	require.NoError(t, json.Unmarshal(doc["created"], &created))
	assert.Equal(t, []string{wantPath}, created)

	wantSpecPath := filepath.Join(memRoot, "docs", "specifications", "beta", "specification.md")

	var modified []string
	require.NoError(t, json.Unmarshal(doc["modified"], &modified))
	assert.Equal(t, []string{wantSpecPath}, modified)
}
