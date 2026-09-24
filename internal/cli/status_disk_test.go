// This file's remaining scenario is real-disk-only: a symlink entry in
// the feature root must be detected without being followed, real
// containment behavior rwfs.Mem does not model the same way (see
// internal/platform/rwfs/doc.go). Every plain status scenario — content a
// fixture controls, no symlink or permission behavior — runs against
// rwfs.Mem in status_internal_test.go. The helpers below stay here rather
// than moving with them: json_refusal_test.go and help_test.go still call
// writeMalformedStatusFeature and newStatusJSONFixture
// (status_json_test.go), which in turn call these.

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
// file under featureDir — the same two files assemble.Start's own read-side
// checks require (MAJOR 1) — and creates featureDir if it does not already
// exist. Content is fixed, so calling it more than once for the same
// featureDir (writeStatusStep does, once per step) never disagrees with
// itself.
func writeConformingFeatureFiles(t *testing.T, featureDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
}

// stepTitle is the heading writeStatusStep writes for id: deliberately
// distinct from id itself, unlike an earlier fixture that wrote "# <id>" —
// a NEXT-column assertion against that fixture proved only that the id was
// echoed twice, never that the table's title column carries the step's own
// markdown.Title.
func stepTitle(id string) string {
	return "Implement " + id
}

// writeStatusStep writes one step file for feature under wd's default
// feature-directory layout, with a heading distinct from its id (stepTitle),
// after writing a conforming specification and state file for feature if
// neither already exists — MAJOR 1: status must not report a clean row for
// a feature "brief start" would itself refuse, so every status fixture
// needs the two files Start's own read-side checks require, not just step
// files.
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
// file, then one step file with no frontmatter at all, under feature's
// default layout: the row's Problem must land on the step file's own
// frontmatter fault, not on a spec/state fault this fixture does not mean
// to exercise (MAJOR 1 checks spec and state ahead of step files).
func writeMalformedStatusFeature(t *testing.T, wd, feature string) {
	t.Helper()

	featureDir := filepath.Join(wd, "docs", "specifications", feature)
	writeConformingFeatureFiles(t, featureDir)
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte("no frontmatter here\n"), 0o600))
}

// Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice
// is the symlink half of SCENARIO-10/11's own seam: a symlink entry in the
// feature root must not be swallowed by the "no features" notice, and must
// not be followed — real os.Root/os.Symlink containment behavior, kept on
// disk.
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
