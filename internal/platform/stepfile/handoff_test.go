package stepfile_test

import (
	"fmt"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStateFile and testSpecificationFile mirror config.Default(); tests
// override them only when the point is a collision with one of them.
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

func Test_CompileHandoff_refuses(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		suffix  string
		state   string
		spec    string
	}{
		{
			// Against "STEP-%d.md", "1.md" renders step 1's handoff as "STEP-11.md", read back as step 11.
			name:    "a suffix containing a digit",
			pattern: "STEP-%d.md",
			suffix:  "1.md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "an empty suffix",
			pattern: "SCENARIO-%02d.md",
			suffix:  "",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix with a forward slash",
			pattern: "SCENARIO-%02d.md",
			suffix:  "sub/HANDOFF.md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix with a backslash",
			pattern: "SCENARIO-%02d.md",
			suffix:  `sub\HANDOFF.md`,
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix carrying a percent",
			pattern: "SCENARIO-%02d.md",
			suffix:  "-HANDOFF%s.md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix equal to the step file's extension",
			pattern: "SCENARIO-%02d.md",
			suffix:  ".md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			// Against "SCENARIO-%02d.step.md", Pattern.ID strips only the final ".md", so ".md" still collides.
			name:    "a suffix rendering the step file's name through a multi-dot pattern",
			pattern: "SCENARIO-%02d.step.md",
			suffix:  ".md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix differing from the step file's extension only by case",
			pattern: "SCENARIO-%02d.md",
			suffix:  ".MD",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix rendering the configured state file's name",
			pattern: "SCENARIO-%02d.md",
			suffix:  "-HANDOFF.md",
			state:   "SCENARIO-01-HANDOFF.md",
			spec:    testSpecificationFile,
		},
		{
			name:    "a suffix rendering the configured specification file's name",
			pattern: "SCENARIO-%02d.md",
			suffix:  "-HANDOFF.md",
			state:   testStateFile,
			spec:    "SCENARIO-01-HANDOFF.md",
		},
		{
			// filepath.Ext(".1") is ".1", the whole name, so Pattern.ID renders "" for every step number.
			name:    "a step pattern whose id is the same for every step number",
			pattern: ".%d",
			suffix:  "-HANDOFF.md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			step, err := stepfile.Compile(c.pattern)
			require.NoError(t, err)

			_, err = stepfile.CompileHandoff(step, c.suffix, c.state, c.spec)

			require.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
		})
	}
}

// Pins that an accepted handoff name never round-trips through Pattern.Number.
func Test_a_handoff_file_name_is_never_recognized_as_a_step_file(t *testing.T) {
	pairs := []struct {
		name    string
		pattern string
		suffix  string
	}{
		{name: "zero-padded pattern", pattern: "SCENARIO-%02d.md", suffix: "-HANDOFF.md"},
		{name: "unpadded pattern", pattern: "STEP-%d.md", suffix: ".handoff.md"},
	}

	// 9/10 and 99/100 straddle the zero-padded pattern's width.
	stepNumbers := []int{1, 9, 10, 99, 100}

	for _, p := range pairs {
		for _, n := range stepNumbers {
			t.Run(fmt.Sprintf("%s/n=%d", p.name, n), func(t *testing.T) {
				step, err := stepfile.Compile(p.pattern)
				require.NoError(t, err)

				handoff, err := stepfile.CompileHandoff(step, p.suffix, testStateFile, testSpecificationFile)
				require.NoError(t, err)

				_, ok := step.Number(handoff.Name(n))

				assert.False(t, ok)
			})
		}
	}
}
