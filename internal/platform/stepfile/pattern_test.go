package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/require"
)

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
			// "%3d" renders Name(1) as "SCENARIO-  1.md", space-padded, which Number cannot read back.
			name:    "an unpadded width verb",
			pattern: "SCENARIO-%3d.md",
		},
		{
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

func Test_Compile_accepts_a_zero_padded_width_verb(t *testing.T) {
	p, err := stepfile.Compile("SCENARIO-%02d.md")

	require.NoError(t, err)
	require.Equal(t, "SCENARIO-03.md", p.Name(3))
}

// Pins that "%0d" renders and round-trips identically to plain "%d".
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

func Test_Number_refuses(t *testing.T) {
	cases := []struct {
		name     string
		filename string
	}{
		{name: "an unpadded near miss", filename: "SCENARIO-7.md"},
		{name: "non-numeric text in the verb's place", filename: "SCENARIO-XX.md"},
		{
			// Only the digits-only scan rejects this: Atoi parses "-5", and Name(-5) renders it right back.
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
