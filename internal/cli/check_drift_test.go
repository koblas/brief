package cli_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// For each predicate internal/platform/conform shares between
// scaffold.Finish and assemble.Check, this file proves the same defect
// refused by one is reported by the other with byte-identical text.

// driftRoot is the virtual root every fixture in this file resolves
// against — fabricated, never a real disk path.
const driftRoot = "/repo"

// stateWithUnterminatedFence carries all four required headings, then an
// opened fence never closed, so only UnterminatedFence's predicate fires.
const stateWithUnterminatedFence = "## Binding decisions\n\ndecision\n\n" +
	"## Left unbuilt\n\nsymbol\n\n" +
	"## Traps\n\ntrap\n\n" +
	"## Open debts\n\ndebt\n\n" +
	"```\nunterminated\n"

// stateMissingTraps omits "## Traps" entirely — heading and section both
// absent, not merely emptied.
const stateMissingTraps = "## Binding decisions\n\ndecision\n\n" +
	"## Left unbuilt\n\nsymbol\n\n" +
	"## Open debts\n\ndebt\n"

// driftKey turns an already-absolute path under driftRoot into the name
// an fstest.MapFS entry is keyed against: the leading separator stripped.
func driftKey(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(path), "/")
}

// driftFixtureFS returns an rwfs.Mem holding a conforming specification,
// state and step file for "demo", extended by extra (path to body).
func driftFixtureFS(status string, checklistItems []string, extra map[string]string) *rwfs.Mem {
	featureDir := filepath.Join(driftRoot, "docs", "specifications", "demo")

	var checklist strings.Builder
	for _, item := range checklistItems {
		checklist.WriteString(item + "\n")
	}

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: " + status + "\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01\n\n" +
		"## Scenario\n\nthe acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		checklist.String()

	files := fstest.MapFS{
		driftKey(driftRoot): &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		driftKey(filepath.Join(featureDir, "specification.md")): &fstest.MapFile{Data: []byte(conformingSpec), Mode: 0o600},
		driftKey(filepath.Join(featureDir, "STATE.md")):         &fstest.MapFile{Data: []byte(conformingState), Mode: 0o600},
		driftKey(filepath.Join(featureDir, "SCENARIO-01.md")):   &fstest.MapFile{Data: []byte(step), Mode: 0o600},
	}

	for name, body := range extra {
		files[driftKey(filepath.Join(driftRoot, name))] = &fstest.MapFile{Data: []byte(body), Mode: 0o600}
	}

	return rwfs.NewMem(files)
}

func Test_check_drift_over_cap_handoff_matches_finishes_own_refusal(t *testing.T) {
	overCap := checkBodyOfLines(61)
	mem := driftFixtureFS("done", []string{"- [x] do the thing"}, map[string]string{
		filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"): overCap,
	})

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, driftRoot, scaffold.WithFS(mem))
	_, finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte(overCap), []byte(conformingState))
	require.ErrorIs(t, finishErr, scaffold.ErrOverCap)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, driftRoot, assemble.WithFS(mem))
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "HANDOFF")
	assert.Equal(t, refusal.Problem, finding.Detail)
}

func Test_check_drift_unterminated_state_fence_matches_finishes_own_refusal(t *testing.T) {
	mem := driftFixtureFS("done", []string{"- [x] do the thing"}, map[string]string{
		filepath.Join("docs", "specifications", "demo", "STATE.md"): stateWithUnterminatedFence,
	})

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, driftRoot, scaffold.WithFS(mem))
	_, finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(stateWithUnterminatedFence))
	require.ErrorIs(t, finishErr, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, driftRoot, assemble.WithFS(mem))
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "STATE.md")
	assert.Equal(t, refusal.Problem, finding.Detail)
}

func Test_check_drift_missing_state_heading_matches_finishes_own_refusal(t *testing.T) {
	mem := driftFixtureFS("done", []string{"- [x] do the thing"}, map[string]string{
		filepath.Join("docs", "specifications", "demo", "STATE.md"): stateMissingTraps,
	})

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, driftRoot, scaffold.WithFS(mem))
	_, finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(stateMissingTraps))
	require.ErrorIs(t, finishErr, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, driftRoot, assemble.WithFS(mem))
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "STATE.md")
	assert.Equal(t, refusal.Problem, finding.Detail)
}

// Two fixtures, differing only in status: Finish refuses an OPEN step's
// unticked item, Check only reports a DONE step's — same problem text.
func Test_check_drift_open_checklist_item_matches_finishes_own_refusal(t *testing.T) {
	cfg := config.Default()
	items := []string{"- [x] first thing", "- [ ] second thing"}

	openMem := driftFixtureFS("open", items, nil)
	scaffoldSrv := scaffold.NewServer(cfg, driftRoot, scaffold.WithFS(openMem))
	_, finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(conformingState))
	require.ErrorIs(t, finishErr, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	doneMem := driftFixtureFS("done", items, nil)
	assembleSrv := assemble.NewServer(cfg, driftRoot, assemble.WithFS(doneMem))
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "SCENARIO-01.md")
	assert.Equal(t, refusal.Problem, finding.Detail)
	assert.Contains(t, finding.Detail, "second thing")
}

// findFinding returns the one finding in findings whose Path contains
// substr, failing the test when there is not exactly one.
func findFinding(t *testing.T, findings []assemble.Finding, substr string) assemble.Finding {
	t.Helper()

	var matches []assemble.Finding

	for _, f := range findings {
		if strings.Contains(f.Path, substr) {
			matches = append(matches, f)
		}
	}

	require.Len(t, matches, 1, "expected exactly one finding matching %q in %+v", substr, findings)

	return matches[0]
}
