package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/require"
)

// Test_Compile_refuses collects the patterns Compile must reject. They
// share one assertion and one behaviour family: a step-file-pattern that
// cannot round-trip — either it does not name exactly one step number, or
// it renders a name Pattern.Number can never read back.
func Test_Compile_refuses(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
	}{
		{name: "an empty pattern", pattern: ""},
		{name: "a pattern with no verb", pattern: "SCENARIO.md"},
		{name: "a pattern with two verbs", pattern: "%d-%d.md"},
		{name: "a pattern with the wrong verb", pattern: "SCENARIO-%s.md"},
		{name: "a pattern carrying a percent escape", pattern: "100%%-%d.md"},
		{name: "a pattern with a path separator", pattern: "steps/%d.md"},
		{
			// "%3d" renders Name(1) as "SCENARIO-  1.md", space-padded, and
			// Number only accepts digits in the verb's place — so it could
			// never read back its own Name output, making a feature's
			// second step unaddressable.
			name:    "an unpadded width verb",
			pattern: "SCENARIO-%3d.md",
		},
		{
			// The "-" flag left-justifies with spaces on the right, which
			// Number's digits-only scan cannot read back either.
			name:    "a left-justified verb",
			pattern: "SCENARIO-%-4d.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := stepfile.Compile(c.pattern)

			require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
		})
	}
}

// Test_Compile_accepts_a_zero_padded_width_verb is the positive control for
// the width refusals above: "%02d" pads with zeros, which Number reads back
// as ordinary digits, so Compile must keep accepting it. It is not in the
// table above because it asserts a different tuple — no error AND a
// rendered name.
func Test_Compile_accepts_a_zero_padded_width_verb(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")

	require.NoError(t, err)
	require.Equal(t, "SCENARIO-03.md", p.Name(3))
}

// Test_Compile_accepts_a_zero_width_zero_padded_verb pins that "%0d"
// renders and round-trips identically to plain "%d": verbRe's group once
// required at least one digit after the leading "0", rejecting "%0d" while
// accepting "%00d" for no behavioural reason.
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

// Test_Number_refuses collects the filenames Number must not recognize as a
// step of the pattern "SCENARIO-%02d.md". One assertion, one family: a name
// that does not round-trip through this pattern. The positive cases above
// stay separate because each asserts a number as well as a bool.
func Test_Number_refuses(t *testing.T) {
	cases := []struct {
		name     string
		filename string
	}{
		{name: "an unpadded near miss", filename: "SCENARIO-7.md"},
		{name: "non-numeric text in the verb's place", filename: "SCENARIO-XX.md"},
		{
			// The one filename only the digits-only scan rejects. Atoi
			// parses "-5" happily, and Name(-5) renders "SCENARIO--5.md"
			// right back, so the round-trip check at the end of Number
			// agrees too — without the scan, a negative step number would
			// be a recognized step.
			name:     "a negative number in the verb's place",
			filename: "SCENARIO--5.md",
		},
		{name: "the wrong extension", filename: "SCENARIO-03.markdown"},
		{name: "a filename that does not match the pattern at all", filename: "STATE.md"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := stepfile.Compile("SCENARIO-%02d.md")
			require.NoError(t, err)

			_, ok := p.Number(c.filename)

			require.False(t, ok)
		})
	}
}

func Test_Name_and_ID_on_a_non_default_pattern(t *testing.T) {
	p, err := stepfile.Compile("step-%d.txt")
	require.NoError(t, err)

	require.Equal(t, "step-3.txt", p.Name(3))
	require.Equal(t, "step-3", p.ID(3))
}
