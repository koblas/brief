// finish's own feature tree can run against an rwfs.Mem the same way
// new/start/check/status's do, through withRootFS — but every finish call
// also reads its --handoff/--state flag values through readSource, which
// always calls os.ReadFile for a non-"-" path with no seam of its own
// (finish.go's own doc comment). At most one of the two flags may be "-"
// (stdin), so a finish test can never eliminate real disk entirely: at
// least one of --handoff/--state still needs a real file. This file
// demonstrates the wiring and mutation-verifies this command's own
// representative guard (unticked-checklist refusal + R11 no-op) on Mem for
// the feature tree, using a small real temp file for the flag arguments;
// finish_test.go's and finish_json_test.go's own bulk stay on disk for
// that reason, unconverted in this pass — see STATE.md's own note.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memWriteFlagFile writes contents to a fresh file under t.TempDir() and
// returns its path — mirroring finish_test.go's own writeInput. finish's
// --handoff/--state arguments always read real disk (readSource), so this
// stays real regardless of the feature tree's own seam.
func memWriteFlagFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

// newMemFinishFixture builds an rwfs.Mem holding one open step,
// "SCENARIO-01", for feature "demo" under memRoot's default layout, with
// checklistItem as its own sole checklist line — mirroring
// finish_test.go's own newFinishCLIFixture, parameterized on the one line
// that distinguishes the ticked control fixture from the unticked-item
// refusal fixture.
func newMemFinishFixture(checklistItem string) *memTree {
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

// runFinishMem runs "finish demo SCENARIO-01 --handoff <path> --state
// <path>" against tree through run()'s own withRootFS seam.
func runFinishMem(t *testing.T, tree *memTree, handoffPath, statePath string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder
	mem := tree.mem()

	err := run(t.Context(), memRoot, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	return stdout.String(), stderr.String(), err
}

// Test_finish_refuses_a_step_with_an_open_checklist_item_mem is the CLI
// slice for SCENARIO-20 against an rwfs.Mem feature tree: a step whose
// "## Implementation Plan" checklist still carries an unticked item is
// refused, naming the step file and the item's line, with the "(no files
// changed)" tail present — mirroring finish_test.go's own
// Test_finish_refuses_a_step_with_an_open_checklist_item.
func Test_finish_refuses_a_step_with_an_open_checklist_item_mem(t *testing.T) {
	tree := newMemFinishFixture("- [ ] do the thing")
	handoffPath := memWriteFlagFile(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteFlagFile(t, "state.md", "## Binding decisions\n\nsome decision\n\n"+
		"## Left unbuilt\n\nsomething left\n\n## Traps\n\na trap\n\n## Open debts\n\na debt\n")

	stdout, stderr, err := runFinishMem(t, tree, handoffPath, statePath)

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)

	want := fmt.Sprintf(
		`brief finish: %s:15: checklist item "do the thing" is not ticked; tick it with [x] once it is done, or remove it, and retry (no files changed)`+"\n",
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))
	assert.Equal(t, want, stderr)
}

// Test_finishing_an_already_finished_step_a_second_time_reports_the_no_op_and_succeeds_mem
// pins R11's user-visible contract against an rwfs.Mem feature tree: a
// second finish call with identical --handoff/--state inputs writes
// nothing, exits 0, and reports the no-op on stderr in a shape distinct
// from a writing finish's "done; wrote …" line — mirroring
// finish_test.go's own
// Test_finishing_an_already_finished_step_a_second_time_reports_the_no_op_and_succeeds.
func Test_finishing_an_already_finished_step_a_second_time_reports_the_no_op_and_succeeds_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	mem := tree.mem()
	handoffPath := memWriteFlagFile(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteFlagFile(t, "state.md", "## Binding decisions\n\nnew decision\n\n"+
		"## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	args := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}

	var firstStdout, firstStderr strings.Builder
	firstErr := run(t.Context(), memRoot, args, nil, &firstStdout, &firstStderr, noBuildInfo, withRootFS(mem))
	require.NoError(t, firstErr)

	var secondStdout, secondStderr strings.Builder
	secondErr := run(t.Context(), memRoot, args, nil, &secondStdout, &secondStderr, noBuildInfo, withRootFS(mem))

	require.NoError(t, secondErr)
	assert.Empty(t, secondStdout.String())
	assert.Equal(t, "brief finish: demo SCENARIO-01 already done with identical inputs; nothing written\n", secondStderr.String())
}
