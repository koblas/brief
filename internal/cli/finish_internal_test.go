// White-box: finish's scenarios run against rwfs.Mem, reaching run()
// directly since withRootFS is unexported. This file never touches real disk.

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memFinishInputDir is the virtual directory every finish _mem test's
// --handoff/--state argument lives under, distinct from the feature tree.
var memFinishInputDir = filepath.Join(memRoot, "in")

// memWriteInput adds path (a fresh file under memFinishInputDir) to tree
// holding contents, and returns its absolute path.
func memWriteInput(tree *memTree, name, contents string) string {
	path := filepath.Join(memFinishInputDir, name)
	tree.file(path, contents)

	return path
}

// newMemFinishFixture builds a memTree holding one open step,
// "SCENARIO-01", for feature "demo", with checklistItem as its sole line.
func newMemFinishFixture(checklistItem string) *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
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
		checklistItem + "\n"
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

// memFinishStep names one step file newMemFinishFixtureWithSteps writes:
// id, status ("open" or "done"), and dependsOn rendered verbatim.
type memFinishStep struct {
	id, status, dependsOn string
}

// newMemFinishFixtureWithSteps builds a memTree holding one step file per
// spec, each fully ticked, with a progress entry for every one.
func newMemFinishFixtureWithSteps(specs ...memFinishStep) *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	var progress strings.Builder

	for _, s := range specs {
		step := "---\n" +
			"id: " + s.id + "\n" +
			"status: " + s.status + "\n" +
			"depends-on: " + s.dependsOn + "\n" +
			"---\n\n" +
			"# " + s.id + " Demo step\n\n" +
			"## Scenario\n\n" +
			"the acceptance criteria\n\n" +
			"## Implementation Plan\n\n" +
			"- [x] do the thing\n"
		tree.file(filepath.Join(featureDir, s.id+".md"), step)

		mark := " "
		if s.status == "done" {
			mark = "x"
		}

		fmt.Fprintf(&progress, "- [%s] %s\n", mark, s.id)
	}

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n" + progress.String()
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	return tree
}

// memWantFinishCompleteLine renders the "wrote …, ticked …; <feature> is
// complete" success line newMemFinishFixture's single-step tree produces.
func memWantFinishCompleteLine(feature, step string) string {
	return fmt.Sprintf(
		"brief finish: %s %s done; wrote %s, replaced %s, ticked %s; %s is complete\n",
		feature, step,
		filepath.Join("docs", "specifications", feature, step+"-HANDOFF.md"),
		filepath.Join("docs", "specifications", feature, "STATE.md"),
		filepath.Join("docs", "specifications", feature, "specification.md"),
		feature)
}

// memOverCapBody returns a handoff/state body of exactly n lines, with a
// trailing newline.
func memOverCapBody(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}

	return strings.Join(lines, "\n") + "\n"
}

// runFinishArgsMem runs args against mem through run()'s own withRootFS
// seam, stdin empty.
func runFinishArgsMem(t *testing.T, mem *rwfs.Mem, args []string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	return stdout.String(), stderr.String(), err
}

// runFinishStdinMem runs args against mem with stdin supplying the piped
// body, through run()'s own withRootFS seam.
func runFinishStdinMem(t *testing.T, mem *rwfs.Mem, args []string, stdin string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder

	err := run(t.Context(), memRoot, args, strings.NewReader(stdin), &stdout, &stderr, noBuildInfo, withRootFS(mem))

	return stdout.String(), stderr.String(), err
}

// runFinishMem runs "finish demo SCENARIO-01 --handoff <path> --state
// <path>" against tree through run()'s own withRootFS seam — the
// single-call shape most tests below need.
func runFinishMem(t *testing.T, tree *memTree, handoffPath, statePath string) (string, string, error) {
	t.Helper()

	return runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})
}

func Test_finishes_the_step_and_prints_nothing_to_stdout_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, memWantFinishCompleteLine("demo", "SCENARIO-01"), stderr)
}

func Test_finishes_the_step_when_the_flags_precede_the_feature_and_step_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "--handoff", handoffPath, "--state", statePath, "demo", "SCENARIO-01"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, memWantFinishCompleteLine("demo", "SCENARIO-01"), stderr)
}

func Test_finishing_an_already_finished_step_a_second_time_reports_the_no_op_and_succeeds_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	mem := tree.mem()
	args := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}

	_, _, firstErr := runFinishArgsMem(t, mem, args)
	require.NoError(t, firstErr)

	secondStdout, secondStderr, secondErr := runFinishArgsMem(t, mem, args)

	require.NoError(t, secondErr)
	assert.Empty(t, secondStdout)
	assert.Equal(t, "brief finish: demo SCENARIO-01 already done with identical inputs; nothing written\n", secondStderr)
}

func Test_finish_names_the_next_open_step_and_its_start_command_mem(t *testing.T) {
	tree := newMemFinishFixtureWithSteps(
		memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
		memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
	)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	want := fmt.Sprintf(
		"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s, ticked %s; next: SCENARIO-02 — run 'brief start demo'\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"),
		filepath.Join("docs", "specifications", "demo", "STATE.md"),
		filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Equal(t, want, stderr)
}

func Test_finish_names_a_blocked_step_as_next_mem(t *testing.T) {
	tree := newMemFinishFixtureWithSteps(
		memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
		memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[SCENARIO-03]"},
		memFinishStep{id: "SCENARIO-03", status: "open", dependsOn: "[]"},
	)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	want := fmt.Sprintf(
		"brief finish: demo SCENARIO-01 done; wrote %s, replaced %s, ticked %s; next: SCENARIO-02 — run 'brief start demo'\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"),
		filepath.Join("docs", "specifications", "demo", "STATE.md"),
		filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Equal(t, want, stderr)
}

func Test_finish_names_a_lower_numbered_open_step_as_next_mem(t *testing.T) {
	tree := newMemFinishFixtureWithSteps(
		memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
		memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
	)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-02", "--handoff", handoffPath, "--state", statePath})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	want := fmt.Sprintf(
		"brief finish: demo SCENARIO-02 done; wrote %s, replaced %s, ticked %s; next: SCENARIO-01 — run 'brief start demo'\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-02-HANDOFF.md"),
		filepath.Join("docs", "specifications", "demo", "STATE.md"),
		filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Equal(t, want, stderr)
}

func Test_finish_leaves_a_legacy_handoff_section_in_the_step_file_untouched_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n\n" +
		"## Handoff\n\n" +
		"LEGACY HANDOFF PROSE, PLEASE KEEP\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", state)
	mem := tree.mem()

	_, _, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})
	require.NoError(t, err)

	got, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, "SCENARIO-01.md")))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "## Handoff\n\nLEGACY HANDOFF PROSE, PLEASE KEEP\n")

	handoffGot, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")))
	require.NoError(t, readErr)
	assert.Equal(t, "NEW-HANDOFF\n", string(handoffGot))
}

func Test_replaces_the_state_file_on_disk_when_finishing_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n"
	statePath := memWriteInput(tree, "state.md", newState)
	mem := tree.mem()

	_, _, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})
	require.NoError(t, err)

	got, readErr := mem.ReadFile(memKey(filepath.Join(memRoot, "docs", "specifications", "demo", "STATE.md")))
	require.NoError(t, readErr)
	assert.Equal(t, newState, string(got))
}

func Test_reads_the_handoff_body_from_stdin_when_the_path_is_a_dash_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	mem := tree.mem()

	_, _, err := runFinishStdinMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", statePath}, "HANDOFF-FROM-STDIN\n")
	require.NoError(t, err)

	got, readErr := mem.ReadFile(memKey(filepath.Join(memRoot, "docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "HANDOFF-FROM-STDIN")
}

func Test_writes_the_handoff_file_beside_the_step_file_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	mem := tree.mem()

	_, _, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})
	require.NoError(t, err)

	got, readErr := mem.ReadFile(memKey(filepath.Join(memRoot, "docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")))
	require.NoError(t, readErr)
	assert.Equal(t, "NEW-HANDOFF\n", string(got))
}

func Test_reads_the_state_body_from_stdin_when_the_path_is_a_dash_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\nSTATE-FROM-STDIN\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	mem := tree.mem()

	_, _, err := runFinishStdinMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-"}, newState)
	require.NoError(t, err)

	got, readErr := mem.ReadFile(memKey(filepath.Join(memRoot, "docs", "specifications", "demo", "STATE.md")))
	require.NoError(t, readErr)
	assert.Equal(t, newState, string(got))
}

func Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	mem := tree.mem()
	argv := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}

	_, _, firstErr := runFinishArgsMem(t, mem, argv)
	require.NoError(t, firstErr)

	names := []string{"SCENARIO-01.md", "STATE.md", "specification.md", "SCENARIO-01-HANDOFF.md"}
	before := make(map[string][]byte, len(names))

	for _, name := range names {
		data, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, name)))
		require.NoError(t, readErr)
		before[name] = data
	}

	// Written directly into mem, not tree: a second tree.mem() snapshot
	// would silently lose the first finish call's own writes.
	differentHandoffPath := filepath.Join(memFinishInputDir, "different-handoff.md")
	require.NoError(t, mem.WriteFile(memKey(differentHandoffPath), []byte("DIFFERENT-HANDOFF\n"), 0o600))

	stdout, stderr, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", differentHandoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	handoffFile := filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")
	want := fmt.Sprintf(
		"brief finish: %s: step \"SCENARIO-01\" is already done and the given handoff differs "+
			"from the one recorded here; diff the handoff you passed against it, then edit this "+
			"file directly if the new handoff is correct (no files changed)\n",
		handoffFile)
	assert.Equal(t, want, stderr)

	for _, name := range names {
		data, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, name)))
		require.NoError(t, readErr)
		assert.Equal(t, before[name], data, "%s must be byte-identical after a refused re-finish", name)
	}
}

func Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", memOverCapBody(61))
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	relHandoffPath, relErr := filepath.Rel(memRoot, handoffPath)
	require.NoError(t, relErr)
	want := fmt.Sprintf(
		"brief finish: %s: handoff is 61 lines, over the cap of 60; cut the handoff to 60 lines or "+
			"fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files changed)\n",
		relHandoffPath)
	assert.Equal(t, want, stderr)
}

func Test_names_stdin_when_the_piped_handoff_is_over_the_cap_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runFinishStdinMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", statePath}, memOverCapBody(61))

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	want := "brief finish: <stdin>: handoff is 61 lines, over the cap of 60; cut the handoff to 60 lines or " +
		"fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files changed)\n"
	assert.Equal(t, want, stderr)
}

func Test_refuses_a_state_body_over_the_cap_and_names_the_state_path_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", memOverCapBody(81))

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	relStatePath, relErr := filepath.Rel(memRoot, statePath)
	require.NoError(t, relErr)
	want := fmt.Sprintf(
		"brief finish: %s: state is 81 lines, over the cap of 80; cut the state to 80 lines or "+
			"fewer, or raise state-cap-lines in .brief.yaml, and retry (no files changed)\n",
		relStatePath)
	assert.Equal(t, want, stderr)
}

func Test_refuses_a_state_body_missing_a_heading_and_names_the_state_path_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Open debts\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	relStatePath, relErr := filepath.Rel(memRoot, statePath)
	require.NoError(t, relErr)
	want := fmt.Sprintf(
		`brief finish: %s: state is missing the "## Traps" section; add a "## Traps" heading to the state body — an empty section is valid — and retry (no files changed)`+"\n",
		relStatePath)
	assert.Equal(t, want, stderr)
}

func Test_returns_a_usage_error_when_no_feature_is_given_to_finish_mem(t *testing.T) {
	stdout, stderr, err := runFinishArgsMem(t, newMemTree(memRoot).mem(), []string{"finish"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: no feature given; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_no_step_is_given_to_finish_mem(t *testing.T) {
	stdout, stderr, err := runFinishArgsMem(t, newMemTree(memRoot).mem(), []string{"finish", "demo"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: no step given; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_finish_is_given_too_many_arguments_mem(t *testing.T) {
	stdout, stderr, err := runFinishArgsMem(t, newMemTree(memRoot).mem(), []string{"finish", "demo", "SCENARIO-01", "extra"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: too many arguments; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_handoff_is_omitted_mem(t *testing.T) {
	tree := newMemTree(memRoot, memFinishInputDir)
	statePath := memWriteInput(tree, "state.md", "")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--state", statePath})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: --handoff is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_state_is_omitted_mem(t *testing.T) {
	tree := newMemTree(memRoot, memFinishInputDir)
	handoffPath := memWriteInput(tree, "handoff.md", "")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: --state is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_when_dash_is_given_for_both_handoff_and_state_mem(t *testing.T) {
	stdout, stderr, err := runFinishArgsMem(t, newMemTree(memRoot).mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", "-"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: - may be given for at most one of --handoff and --state", memOneLine(t, stderr))
}

func Test_returns_a_usage_error_for_an_unknown_finish_flag_mem(t *testing.T) {
	stdout, stderr, err := runFinishArgsMem(t, newMemTree(memRoot).mem(), []string{"finish", "demo", "SCENARIO-01", "--bogus"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Empty(t, stdout)
	assert.Equal(t, "brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'", memOneLine(t, stderr))
}

func Test_names_stdin_when_the_piped_state_s_fence_is_unterminated_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")

	stdout, stderr, err := runFinishStdinMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-"}, "Repro:\n\n```bash\ngo test ./...\n")

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	line := memOneLine(t, stderr)
	assert.Contains(t, line, "<stdin>:3")
}

func Test_names_the_state_path_when_its_fence_is_unterminated_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n```\nunterminated\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	relStatePath, relErr := filepath.Rel(memRoot, statePath)
	require.NoError(t, relErr)
	line := memOneLine(t, stderr)
	assert.Contains(t, line, relStatePath+":3")
}

func Test_returns_an_error_when_the_handoff_path_is_unreadable_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	statePath := memWriteInput(tree, "state.md", "")
	missing := filepath.Join(memFinishInputDir, "does-not-exist.md")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", missing, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	line := memOneLine(t, stderr)
	assert.Contains(t, line, missing)
	assert.False(t, strings.HasSuffix(line, "(no files changed)"), "line %q must not carry the write-refusal tail", line)
}

func Test_preserves_a_CRLF_step_body_when_marking_it_done_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\r\n\r\n" +
		"## Implementation Plan\r\n\r\n" +
		"- [x] do the thing\r\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	state := "## Binding decisions\n\nsome decision\n\n## Left unbuilt\n\nsomething left\n\n## Traps\n\na trap\n\n## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", state)
	mem := tree.mem()

	stdout, stderr, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, memWantFinishCompleteLine("demo", "SCENARIO-01"), stderr)

	got, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, "SCENARIO-01.md")))
	require.NoError(t, readErr)
	want := strings.Replace(step, "status: open\n", "status: done\n", 1)
	assert.Equal(t, want, string(got))
}

func Test_finish_refuses_a_step_with_an_open_checklist_item_mem(t *testing.T) {
	tree := newMemFinishFixture("- [ ] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nsome decision\n\n"+
		"## Left unbuilt\n\nsomething left\n\n## Traps\n\na trap\n\n## Open debts\n\na debt\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	want := fmt.Sprintf(
		`brief finish: %s:15: checklist item "do the thing" is not ticked; tick it with [x] once it is done, or remove it, and retry (no files changed)`+"\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))
	assert.Equal(t, want, stderr)
}

func Test_finish_refuses_a_step_whose_dependency_is_unfinished_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")

	step01 := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step01)

	step02 := "---\n" +
		"id: SCENARIO-02\n" +
		"status: open\n" +
		"depends-on: [SCENARIO-01]\n" +
		"---\n\n" +
		"# SCENARIO-02 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-02.md"), step02)

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	tree.file(filepath.Join(featureDir, "STATE.md"), state)

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n- [ ] SCENARIO-02\n"
	tree.file(filepath.Join(featureDir, "specification.md"), spec)

	names := []string{"SCENARIO-01.md", "SCENARIO-02.md", "STATE.md", "specification.md"}
	before := make(map[string][]byte, len(names))
	preMem := tree.mem()

	for _, name := range names {
		data, readErr := preMem.ReadFile(memKey(filepath.Join(featureDir, name)))
		require.NoError(t, readErr)
		before[name] = data
	}

	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", state)
	mem := tree.mem()

	stdout, stderr, err := runFinishArgsMem(t, mem, []string{"finish", "demo", "SCENARIO-02", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	want := fmt.Sprintf(
		`brief finish: %s: step "SCENARIO-02" depends on "SCENARIO-01", which is not finished; finish SCENARIO-01 first, or remove it from this step's depends-on, and retry (no files changed)`+"\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-02.md"))
	assert.Equal(t, want, stderr)

	for _, name := range names {
		data, readErr := mem.ReadFile(memKey(filepath.Join(featureDir, name)))
		require.NoError(t, readErr)
		assert.Equal(t, before[name], data, "%s must be byte-identical after a refused finish", name)
	}
}

func Test_returns_an_error_for_an_unknown_feature_on_finish_mem(t *testing.T) {
	tree := newMemTree(memRoot, memFinishInputDir)
	handoffPath := memWriteInput(tree, "handoff.md", "h")
	statePath := memWriteInput(tree, "state.md", "s")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "ghost", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	line := memOneLine(t, stderr)
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}

func Test_finish_names_the_known_steps_for_an_unknown_step_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-99", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, `brief finish: no step "SCENARIO-99" in demo; known: SCENARIO-01 (no files changed)`+"\n", stderr)
}

func Test_finish_on_an_unknown_step_with_no_step_files_suggests_creating_one_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"), memFinishInputDir)
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), "# demo\n\n## BDD Acceptance Progress\n").
		file(filepath.Join(featureDir, "STATE.md"), "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, `brief finish: no step "SCENARIO-01" in demo; known: none; run 'brief new step demo' to create one (no files changed)`+"\n", stderr)
}

func Test_finish_json_is_one_exact_document_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	stateFilePath := filepath.Join(featureDir, "STATE.md")
	stepFilePath := filepath.Join(featureDir, "SCENARIO-01.md")
	specFilePath := filepath.Join(featureDir, "specification.md")
	want := `{"schema":1,"command":"finish","ok":true,"exit_code":0,"feature":"demo","step":"SCENARIO-01","changed":true,"handoff_path":` +
		memJSONString(t, filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")) + `,"state_path":` +
		memJSONString(t, stateFilePath) + `,"next":null,"modified":[` +
		memJSONString(t, stateFilePath) + `,` + memJSONString(t, stepFilePath) + `,` + memJSONString(t, specFilePath) + `],"dropped_entries":[]}` + "\n"

	assert.Equal(t, want, stdout)
}

func Test_finish_json_next_is_the_id_title_path_object_when_another_step_is_open_mem(t *testing.T) {
	tree := newMemFinishFixtureWithSteps(
		memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
		memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
	)
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, _, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"})

	require.NoError(t, err)

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))

	stepPath := filepath.Join(memRoot, "docs", "specifications", "demo", "SCENARIO-02.md")
	want := `{"id":"SCENARIO-02","title":"SCENARIO-02 Demo step","path":` + memJSONString(t, stepPath) + `}`
	assert.JSONEq(t, want, string(doc["next"]))
}

func Test_finish_json_decodes_next_and_changed_correctly_mem(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) (mem *rwfs.Mem, args []string)
		key   string
		want  string
	}{
		{
			name: "next is null when nothing else is open",
			setup: func(t *testing.T) (*rwfs.Mem, []string) {
				t.Helper()

				tree := newMemFinishFixture("- [x] do the thing")
				handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
				statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

				return tree.mem(), []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "next",
			want: `null`,
		},
		{
			name: "changed is false when re-finishing with identical inputs",
			setup: func(t *testing.T) (*rwfs.Mem, []string) {
				t.Helper()

				tree := newMemFinishFixture("- [x] do the thing")
				handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
				statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
				mem := tree.mem()
				firstRun := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
				_, _, firstErr := runFinishArgsMem(t, mem, firstRun)
				require.NoError(t, firstErr)

				return mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "changed",
			want: `false`,
		},
		{
			name: "modified is empty when re-finishing with identical inputs",
			setup: func(t *testing.T) (*rwfs.Mem, []string) {
				t.Helper()

				tree := newMemFinishFixture("- [x] do the thing")
				handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
				statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
				mem := tree.mem()
				firstRun := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
				_, _, firstErr := runFinishArgsMem(t, mem, firstRun)
				require.NoError(t, firstErr)

				return mem, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
			},
			key:  "modified",
			want: `[]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mem, args := tc.setup(t)

			stdout, _, err := runFinishArgsMem(t, mem, args)

			require.NoError(t, err)

			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
			assert.JSONEq(t, tc.want, string(doc[tc.key]))
		})
	}
}

func Test_finish_json_refusal_is_unchanged_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

	stdout, stderr, err := runFinishArgsMem(t, tree.mem(), []string{"finish", "demo", "SCENARIO-99", "--handoff", handoffPath, "--state", statePath, "--json"})

	require.Error(t, err)
	assert.Empty(t, stderr)

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "error"}, memJSONKeys(doc))
}

// Agreement pin between scaffold.Finish's next and assemble.Start's own
// next-open-step rule, plus assemble.Status's own.
func Test_finish_next_agrees_with_start_mem(t *testing.T) {
	tests := []struct {
		name          string
		build         func() *memTree
		finishFeature string
		finishStep    string
	}{
		{
			name: "a blocked step is still named next",
			build: func() *memTree {
				return newMemFinishFixtureWithSteps(
					memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
					memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[SCENARIO-03]"},
					memFinishStep{id: "SCENARIO-03", status: "open", dependsOn: "[]"},
				)
			},
			finishFeature: "demo",
			finishStep:    "SCENARIO-01",
		},
		{
			name: "a lower-numbered open step is named next",
			build: func() *memTree {
				return newMemFinishFixtureWithSteps(
					memFinishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
					memFinishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
				)
			},
			finishFeature: "demo",
			finishStep:    "SCENARIO-02",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := tc.build()
			handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
			statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
			mem := tree.mem()

			finishStdout, _, finishErr := runFinishArgsMem(t, mem,
				[]string{"finish", tc.finishFeature, tc.finishStep, "--handoff", handoffPath, "--state", statePath, "--json"})
			require.NoError(t, finishErr)

			var finishDoc struct {
				Next *struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Path  string `json:"path"`
				} `json:"next"`
			}
			require.NoError(t, json.Unmarshal([]byte(finishStdout), &finishDoc))
			require.NotNil(t, finishDoc.Next, "the fixture always leaves another step open")

			startStdout, _, startErr := runFinishArgsMem(t, mem, []string{"start", tc.finishFeature, "--json"})
			require.NoError(t, startErr)

			var startDoc struct {
				Step *struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"step"`
			}
			require.NoError(t, json.Unmarshal([]byte(startStdout), &startDoc))
			require.NotNil(t, startDoc.Step, "the fixture always leaves another step open")

			assert.Equal(t, startDoc.Step.ID, finishDoc.Next.ID)
			assert.Equal(t, startDoc.Step.Title, finishDoc.Next.Title)

			statusStdout, _, statusErr := runFinishArgsMem(t, mem, []string{"status", "--json"})
			require.NoError(t, statusErr)

			var statusDoc struct {
				Features []struct {
					Name string `json:"name"`
					Next *struct {
						Path string `json:"path"`
					} `json:"next"`
				} `json:"features"`
			}
			require.NoError(t, json.Unmarshal([]byte(statusStdout), &statusDoc))

			var statusFeature *struct {
				Name string `json:"name"`
				Next *struct {
					Path string `json:"path"`
				} `json:"next"`
			}

			for i, f := range statusDoc.Features {
				if f.Name == tc.finishFeature {
					statusFeature = &statusDoc.Features[i]

					break
				}
			}

			require.NotNil(t, statusFeature, "status must report the same feature finish just closed a step in")
			require.NotNil(t, statusFeature.Next, "the fixture always leaves another step open")
			assert.Equal(t, statusFeature.Next.Path, finishDoc.Next.Path)
		})
	}
}
