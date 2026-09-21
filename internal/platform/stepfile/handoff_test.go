package stepfile_test

import (
	"fmt"
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

// Test_CompileHandoff_refuses collects every suffix-and-pattern combination
// CompileHandoff must reject. They share one assertion and one behaviour
// family — a handoff filename that collides with something else the feature
// directory holds, or that cannot be told apart from a step file — so the
// reason lives in each case's name and comment rather than in eleven
// function names repeating the same four lines.
//
// Every case supplies all four inputs. Three of them vary something other
// than the suffix (the step pattern, the state filename, the specification
// filename), which is why those are fields rather than constants: a case
// that had to special-case its setup would belong outside this table.
func Test_CompileHandoff_refuses(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		suffix  string
		state   string
		spec    string
	}{
		{
			// Suffix "1.md" against "STEP-%d.md" renders step 1's handoff as
			// "STEP-1" + "1.md" = "STEP-11.md", which Pattern.Number reads
			// back as step 11 — a handoff a directory scan takes for a step.
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
			// A suffix identical to the step filename's own extension names
			// the step file itself, writing the handoff body over it.
			name:    "a suffix equal to the step file's extension",
			pattern: "SCENARIO-%02d.md",
			suffix:  ".md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			// The collision a literal suffix comparison misses: against
			// "SCENARIO-%02d.step.md", Pattern.ID strips only the final
			// ".md", leaving ID(n) == "SCENARIO-01.step". A ".md" suffix is
			// therefore not equal to the pattern's literal suffix
			// (".step.md") yet still renders the exact step filename.
			name:    "a suffix rendering the step file's name through a multi-dot pattern",
			pattern: "SCENARIO-%02d.step.md",
			suffix:  ".md",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			// ".MD" differs from the pattern's ".md" by case alone, but both
			// name one file on the filesystems this repository targets.
			name:    "a suffix differing from the step file's extension only by case",
			pattern: "SCENARIO-%02d.md",
			suffix:  ".MD",
			state:   testStateFile,
			spec:    testSpecificationFile,
		},
		{
			// A rendered handoff name equal to the configured state
			// filename would make finish's handoff write land on the state
			// file it also reads.
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
			// With pattern ".%d", filepath.Ext(".1") is ".1" — the whole
			// rendered name — so Pattern.ID renders "" for every step
			// number and every step's handoff aliases onto one file. The
			// reserved filenames here are chosen not to collide with the
			// suffix, so a failure is unambiguously the ID-varies rule.
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

// Test_a_handoff_file_name_is_never_recognized_as_a_step_file pins the
// property the digit and extension refusals above exist to protect: with no
// digit in the suffix and the extension collision refused, a rendered
// handoff name can never round-trip through Pattern.Number.
//
// Two dimensions, so two loops: the accepted pattern/suffix pairs, and the
// step numbers that straddle the padding width where a round-trip is most
// likely to succeed by accident. Neither loop makes a decision — each
// combination is one subtest, named, so a failure says which pair and which
// number rather than leaving a bare "n=99" label on one of ten assertions.
func Test_a_handoff_file_name_is_never_recognized_as_a_step_file(t *testing.T) {
	pairs := []struct {
		name    string
		pattern string
		suffix  string
	}{
		{name: "zero-padded pattern", pattern: "SCENARIO-%02d.md", suffix: "-HANDOFF.md"},
		{name: "unpadded pattern", pattern: "STEP-%d.md", suffix: ".handoff.md"},
	}

	// 9/10 and 99/100 straddle the zero-padded pattern's width, where a
	// rendered handoff name is likeliest to collide with a step filename.
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
