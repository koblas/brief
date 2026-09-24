// Every plain check scenario runs against rwfs.Mem in check_internal_test.go.
// This file keeps its own symlink case (real containment: a symlink entry
// in the feature root, never followed) plus every helper check_hook_test.go
// and json_refusal_test.go still call.

package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkBodyOfLines returns a body of exactly n distinct lines, with a
// trailing newline.
func checkBodyOfLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return strings.Join(lines, "\n") + "\n"
}

// conformingState is a state body carrying the default profile's four
// required headings, each with content.
const conformingState = "## Binding decisions\n\nsome decision\n\n" +
	"## Left unbuilt\n\nsomething left\n\n" +
	"## Traps\n\na trap\n\n" +
	"## Open debts\n\na debt\n"

// conformingSpec is a specification body carrying the default profile's
// progress heading.
const conformingSpec = "# demo\n\n## BDD Acceptance Progress\n\n- [x] SCENARIO-01\n"

// overCapStateLines is one line past config.Default's state-cap-lines (80),
// the smallest state body overCapState needs to trip the state-cap rule.
const overCapStateLines = 81

// overCapState returns a state body of exactly n lines, carrying the
// default profile's four required headings, padded with filler past
// config.Default's state-cap-lines so conform.OverCap's rule fires —
// mirrors internal/assemble's own checkStateOfLines test helper.
func overCapState(n int) string {
	headings := []string{"## Binding decisions", "## Left unbuilt", "## Traps", "## Open debts"}

	var lines []string
	for _, h := range headings {
		lines = append(lines, h, "", "content")
	}

	for i := 0; len(lines) < n; i++ {
		lines = append(lines, fmt.Sprintf("filler line %d", i))
	}

	return strings.Join(lines[:n], "\n") + "\n"
}

// writeCheckStep writes one step file under featureDir, using the default
// step-file-pattern's naming.
func writeCheckStep(t *testing.T, featureDir, id, status string, checklistItems []string) {
	t.Helper()

	var checklist strings.Builder
	for _, item := range checklistItems {
		checklist.WriteString(item + "\n")
	}

	step := "---\n" +
		"id: " + id + "\n" +
		"status: " + status + "\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# " + id + "\n\n" +
		"## Scenario\n\nthe acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		checklist.String()
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+".md"), []byte(step), 0o600))
}

// Test_check_labels_a_symlinked_feature_directory_under_its_own_name pins
// the feature-level producer's own group: a symlink where a feature
// directory is expected groups under its own link name, always
// "(in flight)" — InFlight is hard-coded true for a feature-level finding
// regardless of what the link's target would have measured.
func Test_check_labels_a_symlinked_feature_directory_under_its_own_name(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

	realDir := filepath.Join(wd, "real-gamma")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(wd, "docs", "specifications", "gamma")))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, ""+
		"gamma  (in flight)\n"+
		"  ERROR  "+filepath.Join("docs", "specifications", "gamma")+"  is a symbolic link, not read as a feature directory\n",
		stdout.String())
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 feature-symlink); ERRORs block finish on in-flight features\n", stderr.String())
}
