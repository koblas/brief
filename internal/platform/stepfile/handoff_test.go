package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/require"
)

func Test_CompileHandoff_names_the_handoff_file_from_the_step_id_and_the_suffix(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	handoff, err := stepfile.CompileHandoff(step, "-HANDOFF.md")
	require.NoError(t, err)

	require.Equal(t, "SCENARIO-01-HANDOFF.md", handoff.Name(1))
}

// Test_CompileHandoff_refuses_a_suffix_containing_a_digit reproduces the
// architect's collision directly: suffix "1.md" against step pattern
// "STEP-%d.md" would render step 1's handoff as "STEP-1" + "1.md" =
// "STEP-11.md", which Pattern.Number recognizes as step 11 — a handoff
// file a directory scan would read as a step.
func Test_CompileHandoff_refuses_a_suffix_containing_a_digit(t *testing.T) {
	step, err := stepfile.Compile("STEP-%d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "1.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_an_empty_suffix(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_with_a_forward_slash(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "sub/HANDOFF.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_with_a_backslash(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, `sub\HANDOFF.md`)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_carrying_a_percent(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "-HANDOFF%s.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_equal_to_the_step_files_extension
// pins the other collision the architect named: a suffix identical to the
// step filename's own extension would name the step file itself, writing
// the handoff body over it.
func Test_CompileHandoff_refuses_a_suffix_equal_to_the_step_files_extension(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, ".md")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_a_handoff_file_name_is_never_recognized_as_a_step_file pins the
// property Test_CompileHandoff_refuses_a_suffix_containing_a_digit and the
// extension-collision refusal protect: with no digit in the suffix and the
// extension case refused, the rendered handoff name can never round-trip
// through Pattern.Number, at every step number this scans.
func Test_a_handoff_file_name_is_never_recognized_as_a_step_file(t *testing.T) {
	scenario, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)
	scenarioHandoff, err := stepfile.CompileHandoff(scenario, "-HANDOFF.md")
	require.NoError(t, err)

	for _, n := range []int{1, 9, 10, 99, 100} {
		_, ok := scenario.Number(scenarioHandoff.Name(n))
		require.False(t, ok, "n=%d", n)
	}

	step, err := stepfile.Compile("STEP-%d.md")
	require.NoError(t, err)
	stepHandoff, err := stepfile.CompileHandoff(step, ".handoff.md")
	require.NoError(t, err)

	for _, n := range []int{1, 9, 10, 99, 100} {
		_, ok := step.Number(stepHandoff.Name(n))
		require.False(t, ok, "n=%d", n)
	}
}
