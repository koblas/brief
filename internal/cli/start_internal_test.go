// start's scenarios — every one, none of them OS-subject — run against
// rwfs.Mem here, reaching run() directly since withRootFS is unexported.
// start_test.go keeps only newStartFixture: flag_error_test.go,
// help_test.go and json_refusal_test.go still call it, so it cannot move
// with the tests that used to be its only callers.

package cli

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMemStartFixture builds an rwfs.Mem holding one step, "SCENARIO-01",
// for feature "demo" under the default profile's layout, with its
// frontmatter status field set to status — mirroring start_test.go's own
// newStartFixture.
func newMemStartFixture(status string) *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: " + status + "\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	return tree
}

// runStartMem runs "start" (or the args given) against tree through
// run()'s own withRootFS seam.
func runStartMem(t *testing.T, tree *memTree, args []string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, &stdout, &stderr, noBuildInfo, withRootFS(tree.mem()))

	return stdout.String(), stderr.String(), err
}

func Test_start_refuses_a_specification_with_no_progress_heading_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.file(filepath.Join(featureDir, "specification.md"), "# demo\n\nno progress list here\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.NotContains(t, lines[0], "(no files changed)")
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Contains(t, lines[0], "## BDD Acceptance Progress")
}

// Test_start_refuses_a_feature_with_no_specification_file_mem is the
// missing-specification half of assemble's own specFault: a feature
// directory that exists but carries no specification.md at all refuses,
// naming that file — the ErrNotExist branch specFault takes before ever
// reaching the no-progress-heading branch
// Test_start_refuses_a_specification_with_no_progress_heading_mem covers.
func Test_start_refuses_a_feature_with_no_specification_file_mem(t *testing.T) {
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), featureDir)
	tree.file(filepath.Join(featureDir, "STATE.md"), "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Contains(t, lines[0], "specification.md not found")
}

func Test_start_names_an_absent_acceptance_heading_and_still_prints_the_brief_mem(t *testing.T) {
	baselineStdout, _, baselineErr := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo"})
	require.NoError(t, baselineErr)

	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))

	wantStdout := strings.Replace(baselineStdout, "\n## Scenario\n\nthe acceptance criteria\n", "", 1)
	assert.Equal(t, wantStdout, stdout)
}

func Test_start_names_an_absent_state_heading_and_still_prints_the_brief_mem(t *testing.T) {
	baselineStdout, _, baselineErr := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo"})
	require.NoError(t, baselineErr)

	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "STATE.md"))

	wantStdout := strings.Replace(baselineStdout, "\n## Traps\n\na trap\n", "", 1)
	assert.Equal(t, wantStdout, stdout)
}

func Test_start_names_every_absent_convention_on_its_own_line_mem(t *testing.T) {
	baselineStdout, _, baselineErr := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo"})
	require.NoError(t, baselineErr)

	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)
	tree.file(filepath.Join(featureDir, "STATE.md"), "just some prose, no headings here\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 5)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))
	assert.Contains(t, lines[0], "## Scenario")
	assert.Contains(t, lines[1], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[1], "## Binding decisions")
	assert.Contains(t, lines[2], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[2], "## Left unbuilt")
	assert.Contains(t, lines[3], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[3], "## Traps")
	assert.Contains(t, lines[4], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[4], "## Open debts")

	wantStdout := baselineStdout
	for _, block := range []string{
		"\n## Scenario\n\nthe acceptance criteria\n",
		"\n## Binding decisions\n\nsome decision\n",
		"\n## Left unbuilt\n\nsomething left\n",
		"\n## Traps\n\na trap\n",
		"\n## Open debts\n\na debt\n",
	} {
		wantStdout = strings.Replace(wantStdout, block, "", 1)
	}

	assert.Equal(t, wantStdout, stdout)
}

func Test_start_says_nothing_about_a_present_but_empty_convention_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)
	tree.file(filepath.Join(featureDir, "STATE.md"), "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	_, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
}

func Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading_mem(t *testing.T) {
	// A single shared *rwfs.Mem, reused across all three run() calls: unlike
	// runStartMem's own tree.mem(), called fresh per call, "new feature" and
	// "new step" must land where "start" then reads them — tree.mem() copies
	// its own fixture on every call (rwfs.NewMem's own contract), so three
	// independent Mem instances would never see one another's writes.
	mem := newMemTree(memRoot).mem()
	var discard strings.Builder

	require.NoError(t, run(t.Context(), memRoot, []string{"new", "feature", "demo"}, nil, &discard, &discard, noBuildInfo, withRootFS(mem)))
	require.NoError(t, run(t.Context(), memRoot, []string{"new", "step", "demo"}, nil, &discard, &discard, noBuildInfo, withRootFS(mem)))

	var stdoutBuf, stderrBuf strings.Builder
	err := run(t.Context(), memRoot, []string{"start", "demo"}, nil, &stdoutBuf, &stderrBuf, noBuildInfo, withRootFS(mem))
	stdout, stderr := stdoutBuf.String(), stderrBuf.String()

	require.NoError(t, err)
	assert.NotEmpty(t, stdout)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "## Scenario")
}

func Test_start_still_refuses_a_state_file_whose_fence_is_unterminated_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.file(filepath.Join(featureDir, "STATE.md"), "```\nunterminated\n## Binding decisions\n\nsome decision\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "STATE.md"))
}

func Test_prints_the_brief_and_writes_nothing_to_stderr_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "SCENARIO-01 — 0 done, 1 open")
	assert.Contains(t, stdout, "the acceptance criteria")
	assert.Contains(t, stdout, "some decision")
}

func Test_start_says_the_feature_is_complete_when_every_step_is_done_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("done"), []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t,
		"brief start: "+filepath.Join("docs", "specifications", "demo")+
			": feature is complete, 1 of 1 steps done; run 'brief new step demo' to add the next one\n",
		stderr)
}

func Test_start_says_there_are_no_step_files_yet_for_an_empty_feature_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)
	tree.file(filepath.Join(featureDir, "specification.md"), "# demo\n\n## BDD Acceptance Progress\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t,
		"brief start: "+filepath.Join("docs", "specifications", "demo")+
			": no step files yet; run 'brief new step demo' to scaffold the first one\n",
		stderr)
}

func Test_prints_the_full_checklist_when_it_contains_a_nested_fence_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"~~~\n```\n## Not a heading\n~~~\n\n" +
		"- [ ] real task\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "real task")
}

func Test_prints_the_brief_from_a_CRLF_step_file_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\r\n\r\n" +
		"## Scenario\r\n\r\n" +
		"the acceptance criteria\r\n\r\n" +
		"## Implementation Plan\r\n\r\n" +
		"- [ ] do the thing\r\n\r\n" +
		"## Handoff\r\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	state := "## Binding decisions\r\n\r\nsome decision\r\n\r\n" +
		"## Left unbuilt\r\n\r\nsomething left\r\n\r\n" +
		"## Traps\r\n\r\na trap\r\n\r\n" +
		"## Open debts\r\n\r\na debt\r\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\r\n\r\n## BDD Acceptance Progress\r\n\r\n- [ ] SCENARIO-01\r\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "SCENARIO-01 Demo step")
	assert.Contains(t, stdout, "the acceptance criteria")
	assert.Contains(t, stdout, "do the thing")
	assert.Contains(t, stdout, "some decision")
	assert.Contains(t, stdout, "something left")
	assert.Contains(t, stdout, "a trap")
	assert.Contains(t, stdout, "a debt")
}

func Test_returns_a_usage_error_when_no_feature_is_given_to_start_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief start: no feature given; run 'brief start <feature>'\n", stderr)
}

func Test_returns_a_usage_error_when_start_is_given_too_many_arguments_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "a", "b"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief start: too many arguments; run 'brief start <feature>'\n", stderr)
}

func Test_returns_a_usage_error_for_an_unknown_start_flag_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--bogus", "demo"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr)
}

func Test_prints_the_start_usage_for_help_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--help"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "brief start reads; it never writes.")
}

func Test_returns_a_usage_error_when_help_precedes_an_undefined_flag_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--help", "--bogus"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr)
}

func Test_returns_a_usage_error_when_an_undefined_flag_precedes_help_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--bogus", "--help"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr)
}

func Test_start_still_refuses_a_feature_with_no_state_file_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)
	tree.file(filepath.Join(featureDir, "specification.md"), "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, filepath.Join("docs", "specifications", "demo", "STATE.md"))
}

func Test_returns_an_error_for_an_unknown_feature_on_start_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "ghost"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.NotEmpty(t, stderr)
}

// Test_returns_an_error_naming_the_known_feature_on_start_mem is
// enrichUnknownFeature's own control arm: the assemble.Server it builds to
// list known features (assemble.WithFS(rootFS)) must read the same Mem
// fixture as srv.Start itself, not real disk — every other unknown-feature
// _mem test above has zero other features on its own tree, so "known:
// none" there is identical to what a real, empty root would also produce
// and proves nothing about this seam specifically.
func Test_returns_an_error_naming_the_known_feature_on_start_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memConformingFeatureFiles(tree, filepath.Join(memRoot, "docs", "specifications", "alpha"))

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "ghost"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "known: alpha")
}

// memWriteStartMalformedFixture adds a conforming specification and empty
// state file for "demo" under root's default layout, plus one step file,
// SCENARIO-01.md, with no frontmatter at all — mirroring start_test.go's
// own writeStartMalformedFixture.
func memWriteStartMalformedFixture(tree *memTree, featureDir string) {
	tree.
		file(filepath.Join(featureDir, "specification.md"), "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n").
		file(filepath.Join(featureDir, "STATE.md"), "").
		file(filepath.Join(featureDir, "SCENARIO-01.md"), "no frontmatter here\n")
}

func Test_start_names_the_malformed_step_file_relative_to_the_working_directory_mem(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
		featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
		memWriteStartMalformedFixture(tree, featureDir)

		stdout, stderr, err := runStartMem(t, tree, []string{"start", "demo"})

		assert.Equal(t, 1, ExitCode(err))
		assert.Empty(t, stdout)
		assert.Equal(t,
			"brief start: "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault\n",
			stderr)
	})

	t.Run("json", func(t *testing.T) {
		tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
		featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
		memWriteStartMalformedFixture(tree, featureDir)

		stdout, stderr, err := runStartMem(t, tree, []string{"start", "--json", "demo"})

		assert.Equal(t, 1, ExitCode(err))
		assert.Empty(t, stderr)

		got := memDecodeErrorDocument(t, []byte(stdout), "start")
		require.NotNil(t, got.Path)
		assert.Equal(t, filepath.Join(featureDir, "SCENARIO-01.md"), *got.Path)
		assert.Equal(t,
			"brief start: "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault",
			got.Message)
	})

	t.Run("from a subdirectory", func(t *testing.T) {
		root := memRoot
		tree := newMemTree(root, filepath.Join(root, "docs", "specifications"), filepath.Join(root, "a"))
		tree.file(filepath.Join(root, ".brief.yaml"), "")
		featureDir := filepath.Join(root, "docs", "specifications", "demo")
		memWriteStartMalformedFixture(tree, featureDir)
		wd := filepath.Join(root, "a")

		var stdout, stderr strings.Builder
		err := run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(tree.mem()))

		assert.Equal(t, 1, ExitCode(err))
		assert.Equal(t,
			"brief start: "+filepath.Join("..", "docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault\n",
			stderr.String())
	})
}

// startJSONDocument is start's --json success document, decoded field by
// field: the common header (schema, command, ok, exit_code) precedes
// assemble.Brief's own fields, flattened by embedding — mirroring
// start_test.go's own startJSONDocument.
type startJSONDocument struct {
	assemble.Brief

	Schema   int    `json:"schema"`
	Command  string `json:"command"`
	OK       bool   `json:"ok"`
	ExitCode int    `json:"exit_code"`
}

func Test_start_json_success_document_golden_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "--json", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	want := `{"schema":1,"command":"start","ok":true,"exit_code":0,` +
		`"done":0,"open":1,` +
		`"step":{"id":"SCENARIO-01","title":"SCENARIO-01 Demo step",` +
		`"acceptance":{"heading":"## Scenario","body":"the acceptance criteria","found":true},` +
		`"checklist":{"heading":"## Implementation Plan","body":"- [ ] do the thing","found":true}},` +
		`"inherited":[` +
		`{"heading":"## Binding decisions","body":"some decision","found":true},` +
		`{"heading":"## Left unbuilt","body":"something left","found":true},` +
		`{"heading":"## Traps","body":"a trap","found":true},` +
		`{"heading":"## Open debts","body":"a debt","found":true}],` +
		`"shortfalls":null}` + "\n"

	assert.Equal(t, want, stdout)
}

func Test_start_prints_the_brief_as_json_when_asked_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "--json", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	var got startJSONDocument
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	assert.Equal(t, 1, got.Schema)
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
	assert.Equal(t, 0, got.ExitCode)

	require.NotNil(t, got.Step)
	assert.Equal(t, 0, got.Done)
	assert.Equal(t, 1, got.Open)
	assert.Equal(t, "SCENARIO-01", got.Step.ID)
	assert.Equal(t, "the acceptance criteria", got.Step.Acceptance.Body)
	assert.Equal(t, "- [ ] do the thing", got.Step.Checklist.Body)
	assert.Equal(t, "some decision", got.Inherited[0].Body)
}

func Test_start_json_emits_a_null_step_for_a_completed_feature_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("done"), []string{"start", "--json", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, `"step":null`)
	assert.Contains(t, stdout, `"done":1`)

	var got startJSONDocument
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
}

func Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)
	tree.file(filepath.Join(featureDir, "specification.md"), "# demo\n\n## BDD Acceptance Progress\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "--json", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, `"step":null`)
	assert.Contains(t, stdout, `"done":0`)
	assert.Contains(t, stdout, `"open":0`)

	var got startJSONDocument
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
}

func Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "--json", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	stdoutLines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	require.Len(t, stdoutLines, 1)

	var got assemble.Brief
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.False(t, got.Step.Acceptance.Found)
	require.Len(t, got.Shortfalls, 1)
	assert.Contains(t, got.Shortfalls[0].Path, "SCENARIO-01.md")
	assert.True(t, filepath.IsAbs(got.Shortfalls[0].Path))
}

func Test_start_json_writes_the_error_document_when_it_refuses_mem(t *testing.T) {
	t.Run("malformed feature", func(t *testing.T) {
		tree := newMemStartFixture("open")
		featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
		tree.file(filepath.Join(featureDir, "specification.md"), "# demo\n\nno progress list here\n")

		stdout, stderr, err := runStartMem(t, tree, []string{"start", "--json", "demo"})

		assert.Equal(t, 1, ExitCode(err))
		assert.Empty(t, stderr)

		got := memDecodeErrorDocument(t, []byte(stdout), "start")
		assert.Equal(t, "refusal", got.Kind)
		assert.NotContains(t, got.Message, "(no files changed)")
	})

	t.Run("unknown feature", func(t *testing.T) {
		stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--json", "ghost"})

		assert.Equal(t, 1, ExitCode(err))
		assert.Empty(t, stderr)

		got := memDecodeErrorDocument(t, []byte(stdout), "start")
		assert.Equal(t, "refusal", got.Kind)
		assert.NotEmpty(t, got.Message)
	})
}

func Test_start_json_still_reports_usage_errors_mem(t *testing.T) {
	t.Run("no feature given", func(t *testing.T) {
		stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--json"})

		require.ErrorIs(t, err, ErrUsage)
		assert.Equal(t, 2, ExitCode(err))
		assert.Empty(t, stderr)

		message := memDecodeUsageErrorDocument(t, []byte(stdout), "start", nil)
		assert.Equal(t, "brief start: no feature given; run 'brief start <feature>'", message)
	})

	t.Run("undefined flag alongside json", func(t *testing.T) {
		stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--bogus", "--json", "demo"})

		require.ErrorIs(t, err, ErrUsage)
		assert.Equal(t, 2, ExitCode(err))
		assert.Empty(t, stderr)

		message := memDecodeUsageErrorDocument(t, []byte(stdout), "start", nil)
		assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'", message)
	})
}

func Test_start_accepts_the_json_flag_after_the_feature_mem(t *testing.T) {
	beforeOut, beforeErr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "--json", "demo"})
	require.NoError(t, err)

	afterOut, afterErr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo", "--json"})

	require.NoError(t, err)
	assert.Equal(t, beforeOut, afterOut)
	assert.Equal(t, beforeErr, afterErr)
}

func Test_start_json_help_prints_usage_not_json_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemTree(memRoot), []string{"start", "--json", "--help"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "brief start reads; it never writes.")
}

func Test_start_without_json_is_byte_identical_to_the_text_brief_mem(t *testing.T) {
	stdout, stderr, err := runStartMem(t, newMemStartFixture("open"), []string{"start", "demo"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, ""+
		"SCENARIO-01 — 0 done, 1 open\n\n"+
		"# SCENARIO-01 Demo step\n\n"+
		"## Scenario\n\n"+
		"the acceptance criteria\n\n"+
		"## Implementation Plan\n\n"+
		"- [ ] do the thing\n\n"+
		"## Binding decisions\n\n"+
		"some decision\n\n"+
		"## Left unbuilt\n\n"+
		"something left\n\n"+
		"## Traps\n\n"+
		"a trap\n\n"+
		"## Open debts\n\n"+
		"a debt\n",
		stdout)
}

// Test_json_mode_renders_a_refusal_as_one_document_mem is the golden-bytes
// proof of the Gherkin row: "brief start demo --json" against a feature
// whose state file is missing renders assemble.Start's *RefusalError as
// one compact document, key order pinned, "line" and "files_changed" both
// null — mirroring json_refusal_test.go's own
// Test_json_mode_renders_a_refusal_as_one_document.
func Test_json_mode_renders_a_refusal_as_one_document_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	statePath := filepath.Join(featureDir, "STATE.md")
	delete(tree.entries, memKey(statePath))

	// wantProblem is captured from the same fixture's own rwfs.Mem adapter
	// (fstest.MapFS's own missing-file wording), rather than hardcoded —
	// see status_internal_test.go's own missingFileDetail for the same
	// capture.
	_, missingErr := tree.mem().ReadFile(memKey(statePath))
	var missingPathErr *fs.PathError
	require.ErrorAs(t, missingErr, &missingPathErr)
	wantProblem := missingPathErr.Err.Error()

	_, textStderr, textErr := runStartMem(t, tree, []string{"start", "demo"})
	assert.Equal(t, 1, ExitCode(textErr))
	wantMessage := strings.TrimRight(textStderr, "\n")

	stdout, stderr, err := runStartMem(t, tree, []string{"start", "--json", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stderr)

	want := `{"schema":1,"command":"start","ok":false,"exit_code":1,` +
		`"error":{"kind":"refusal",` +
		`"message":` + memJSONString(t, wantMessage) + `,` +
		`"path":` + memJSONString(t, statePath) + `,` +
		`"line":null,` +
		`"problem":` + memJSONString(t, wantProblem) + `,` +
		`"fix":"make it readable and re-run",` +
		`"files_changed":null}}` + "\n"

	assert.Equal(t, want, stdout)
}
