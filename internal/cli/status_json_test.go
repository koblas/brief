// Every plain status --json scenario runs against rwfs.Mem in
// status_internal_test.go. newStatusJSONFixture stays here rather than
// moving with them: help_test.go still calls it.

package cli_test

import (
	"path/filepath"
	"testing"
)

// newStatusJSONFixture writes four features named so fs.ReadDir's byte
// order is also the golden order.
func newStatusJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()

	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-03.md", "SCENARIO-03", "open", []string{"SCENARIO-02"})
	writeStatusStep(t, wd, "alpha", "SCENARIO-04.md", "SCENARIO-04", "open", nil)

	writeStatusStep(t, wd, "beta", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "beta", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	writeMalformedStatusFeature(t, wd, "delta")

	writeConformingFeatureFiles(t, filepath.Join(wd, "docs", "specifications", "epsilon"))

	return wd
}
