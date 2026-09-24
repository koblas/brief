// Every finish scenario except finish_json_disk_test.go's own two os.Mkdir
// cases runs against rwfs.Mem in finish_internal_test.go. newFinishCLIFixture
// and writeInput stay here rather than moving with them: flag_error_test.go,
// help_test.go, json_refusal_test.go and invalid_config_test.go still call
// them.

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newFinishCLIFixture writes one open step, "SCENARIO-01", for feature
// "demo" under the default profile's layout, with a fully ticked checklist
// and a bare handoff anchor, and returns the working directory Run should
// be called with.
func newFinishCLIFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	return wd
}

// writeInput writes contents to a fresh file under t.TempDir() and returns
// its path.
func writeInput(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}
