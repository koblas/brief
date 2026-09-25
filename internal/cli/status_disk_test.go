// This file's remaining scenario is real-disk-only: a symlink entry in
// the feature root must be detected without being followed. The helpers
// below stay here since other test files still call them.

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConformingFeatureFiles writes a conforming specification and state
// file under featureDir, creating it if needed; safe to call more than once.
func writeConformingFeatureFiles(t *testing.T, featureDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
}

// stepTitle is the heading writeStatusStep writes for id, deliberately
// distinct from id so a NEXT-column assertion can't pass on echo alone.
func stepTitle(id string) string {
	return "Implement " + id
}

// writeStatusStep writes one step file for feature, after writing a
// conforming specification and state file for it if neither exists yet.
func writeStatusStep(t *testing.T, wd, feature, name, id, status string, dependsOn []string) {
	t.Helper()

	featureDir := filepath.Join(wd, "docs", "specifications", feature)
	writeConformingFeatureFiles(t, featureDir)

	deps := "depends-on: []\n"
	if len(dependsOn) > 0 {
		var sb strings.Builder

		sb.WriteString("depends-on:\n")
		for _, dep := range dependsOn {
			sb.WriteString("  - " + dep + "\n")
		}

		deps = sb.String()
	}

	step := "---\n" +
		"id: " + id + "\n" +
		"status: " + status + "\n" +
		deps +
		"---\n\n" +
		"# " + stepTitle(id) + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(step), 0o600))
}

// writeMalformedStatusFeature writes a conforming specification and state
// file, then one step file with no frontmatter at all.
func writeMalformedStatusFeature(t *testing.T, wd, feature string) {
	t.Helper()

	featureDir := filepath.Join(wd, "docs", "specifications", feature)
	writeConformingFeatureFiles(t, featureDir)
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte("no frontmatter here\n"), 0o600))
}

func Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice(t *testing.T) {
	wd := t.TempDir()
	realDir := filepath.Join(wd, "real-delta")
	require.NoError(t, os.MkdirAll(realDir, 0o755))

	specs := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(specs, 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(specs, "delta")))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"delta    -     -        (malformed, see below)\n",
		stdout.String())
	assert.NotContains(t, stderr.String(), "no features found")
}
