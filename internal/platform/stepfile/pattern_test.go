package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/require"
)

func Test_Compile_refuses_an_empty_pattern(t *testing.T) {
	_, err := stepfile.Compile("")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

func Test_Compile_refuses_a_pattern_with_no_verb(t *testing.T) {
	_, err := stepfile.Compile("SCENARIO.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

func Test_Compile_refuses_a_pattern_with_two_verbs(t *testing.T) {
	_, err := stepfile.Compile("%d-%d.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

func Test_Compile_refuses_a_pattern_with_the_wrong_verb(t *testing.T) {
	_, err := stepfile.Compile("SCENARIO-%s.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

func Test_Compile_refuses_a_pattern_carrying_a_percent_escape(t *testing.T) {
	_, err := stepfile.Compile("100%%-%d.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

func Test_Compile_refuses_a_pattern_with_a_path_separator(t *testing.T) {
	_, err := stepfile.Compile("steps/%d.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

// Test_Compile_refuses_an_unpadded_width_verb reproduces the reviewer's
// finding: "%3d" renders Name(1) as "SCENARIO-  1.md" (space-padded), but
// Number can never read a space back out of the verb's place, since it
// only accepts digits there. A pattern Compile accepts but Number can
// never recognize its own Name output for makes a feature's second step
// unaddressable.
func Test_Compile_refuses_an_unpadded_width_verb(t *testing.T) {
	_, err := stepfile.Compile("SCENARIO-%3d.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

// Test_Compile_refuses_a_left_justified_verb is the "%-4d" half of the
// same finding: the "-" flag left-justifies with spaces on the right,
// which Number's digits-only scan can never read back either.
func Test_Compile_refuses_a_left_justified_verb(t *testing.T) {
	_, err := stepfile.Compile("SCENARIO-%-4d.md")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

// Test_Compile_accepts_a_zero_padded_width_verb is the positive control
// for the two refusals above: "%02d" pads with zeros, which Number reads
// back as ordinary digits, so Compile must keep accepting it.
func Test_Compile_accepts_a_zero_padded_width_verb(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")

	require.NoError(t, err)
	require.Equal(t, "SCENARIO-03.md", p.Name(3))
}

// Test_Compile_accepts_a_zero_width_zero_padded_verb reproduces the
// reviewer's finding directly: "%0d" renders and round-trips identically
// to plain "%d", so it must be accepted the same as "%00d" — verbRe's
// group used to require at least one digit after the leading "0",
// rejecting "%0d" while accepting "%00d" for no behavioral reason.
func Test_Compile_accepts_a_zero_width_zero_padded_verb(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%0d.md")

	require.NoError(t, err)
	require.Equal(t, "SCENARIO-3.md", p.Name(3))
}

func Test_Name_renders_the_step_number_through_the_pattern(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	require.Equal(t, "SCENARIO-03.md", p.Name(3))
}

func Test_ID_strips_the_rendered_names_extension(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	require.Equal(t, "SCENARIO-03", p.ID(3))
}

func Test_Number_matches_a_filename_that_round_trips_exactly(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	n, ok := p.Number("SCENARIO-03.md")

	require.True(t, ok)
	require.Equal(t, 3, n)
}

func Test_Number_round_trips_a_number_wider_than_the_verbs_padding(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	n, ok := p.Number("SCENARIO-100.md")

	require.True(t, ok)
	require.Equal(t, 100, n)
}

func Test_Number_refuses_an_unpadded_near_miss(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, ok := p.Number("SCENARIO-7.md")

	require.False(t, ok)
}

func Test_Number_refuses_non_numeric_text_in_the_verbs_place(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, ok := p.Number("SCENARIO-XX.md")

	require.False(t, ok)
}

func Test_Number_refuses_a_filename_with_the_wrong_extension(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, ok := p.Number("SCENARIO-03.markdown")

	require.False(t, ok)
}

func Test_Number_refuses_a_filename_that_does_not_match_the_pattern_at_all(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")
	require.NoError(t, err)

	_, ok := p.Number("STATE.md")

	require.False(t, ok)
}

func Test_Name_and_ID_on_a_non_default_pattern(t *testing.T) {
	p, err := stepfile.Compile("step-%d.txt")
	require.NoError(t, err)

	require.Equal(t, "step-3.txt", p.Name(3))
	require.Equal(t, "step-3", p.ID(3))
}
