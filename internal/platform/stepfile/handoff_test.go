package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStateFile and testSpecificationFile are the reserved filenames every
// test below passes to CompileHandoff, mirroring config.Default(), except
// where the test's whole point is a collision with one of them.
const (
	testStateFile         = "STATE.md"
	testSpecificationFile = "specification.md"
)

func Test_CompileHandoff_names_the_handoff_file_from_the_step_id_and_the_suffix(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	handoff, err := stepfile.CompileHandoff(step, "-HANDOFF.md", testStateFile, testSpecificationFile)
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

	_, err = stepfile.CompileHandoff(step, "1.md", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_an_empty_suffix(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_with_a_forward_slash(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "sub/HANDOFF.md", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_with_a_backslash(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, `sub\HANDOFF.md`, testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

func Test_CompileHandoff_refuses_a_suffix_carrying_a_percent(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "-HANDOFF%s.md", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_equal_to_the_step_files_extension
// pins the other collision the architect named: a suffix identical to the
// step filename's own extension would name the step file itself, writing
// the handoff body over it.
func Test_CompileHandoff_refuses_a_suffix_equal_to_the_step_files_extension(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, ".md", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_that_renders_the_same_name_as_the_step_file_through_a_multi_dot_pattern
// pins the collision a literal suffix comparison misses: against
// "SCENARIO-%02d.step.md", Pattern.ID strips only the final ".md"
// (filepath.Ext), leaving ID(n) == "SCENARIO-01.step" rather than
// "SCENARIO-01" — so a handoff suffix of ".md" is not equal to the
// pattern's whole literal suffix (".step.md") but still renders the exact
// step filename once appended to that ID.
func Test_CompileHandoff_refuses_a_suffix_that_renders_the_same_name_as_the_step_file_through_a_multi_dot_pattern(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.step.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, ".md", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_that_differs_from_the_step_files_extension_only_by_case
// reproduces the collision on a case-insensitive filesystem: ".MD" differs
// from step pattern "SCENARIO-%02d.md"'s ".md" extension by case alone, but
// both name the same file on the filesystems this repository targets.
func Test_CompileHandoff_refuses_a_suffix_that_differs_from_the_step_files_extension_only_by_case(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, ".MD", testStateFile, testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_that_renders_the_configured_state_file_name
// pins the collision named separately from the step file's own name: a
// rendered handoff filename identical to the configured state filename
// would make finish's handoff write overwrite the state file it also
// reads.
func Test_CompileHandoff_refuses_a_suffix_that_renders_the_configured_state_file_name(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "-HANDOFF.md", "SCENARIO-01-HANDOFF.md", testSpecificationFile)

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_suffix_that_renders_the_configured_specification_file_name
// mirrors the state-file case for the specification filename.
func Test_CompileHandoff_refuses_a_suffix_that_renders_the_configured_specification_file_name(t *testing.T) {
	step, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "-HANDOFF.md", testStateFile, "SCENARIO-01-HANDOFF.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_CompileHandoff_refuses_a_step_pattern_whose_id_is_the_same_for_every_step_number
// pins the pattern this plan's rendered-name checks cannot see coming: with
// step-file-pattern ".%d", filepath.Ext(".1") is ".1" — the entire rendered
// name — so Pattern.ID renders "" for every step number, and every step's
// handoff would alias onto the one file this suffix names. The reserved
// filenames are chosen to not collide with the suffix, so a failure here
// is unambiguously the ID-varies rule, not a rendered-name collision.
func Test_CompileHandoff_refuses_a_step_pattern_whose_id_is_the_same_for_every_step_number(t *testing.T) {
	step, err := stepfile.Compile(".%d")
	require.NoError(t, err)

	_, err = stepfile.CompileHandoff(step, "-HANDOFF.md", testStateFile, testSpecificationFile)

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
	scenarioHandoff, err := stepfile.CompileHandoff(scenario, "-HANDOFF.md", testStateFile, testSpecificationFile)
	require.NoError(t, err)

	_, ok := scenario.Number(scenarioHandoff.Name(1))
	assert.False(t, ok, "n=1")
	_, ok = scenario.Number(scenarioHandoff.Name(9))
	assert.False(t, ok, "n=9")
	_, ok = scenario.Number(scenarioHandoff.Name(10))
	assert.False(t, ok, "n=10")
	_, ok = scenario.Number(scenarioHandoff.Name(99))
	assert.False(t, ok, "n=99")
	_, ok = scenario.Number(scenarioHandoff.Name(100))
	assert.False(t, ok, "n=100")

	step, err := stepfile.Compile("STEP-%d.md")
	require.NoError(t, err)
	stepHandoff, err := stepfile.CompileHandoff(step, ".handoff.md", testStateFile, testSpecificationFile)
	require.NoError(t, err)

	_, ok = step.Number(stepHandoff.Name(1))
	assert.False(t, ok, "n=1")
	_, ok = step.Number(stepHandoff.Name(9))
	assert.False(t, ok, "n=9")
	_, ok = step.Number(stepHandoff.Name(10))
	assert.False(t, ok, "n=10")
	_, ok = step.Number(stepHandoff.Name(99))
	assert.False(t, ok, "n=99")
	_, ok = step.Number(stepHandoff.Name(100))
	assert.False(t, ok, "n=100")
}
