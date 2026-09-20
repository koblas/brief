package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is the proof SCENARIO-22 exists for: for each of the four
// predicates internal/platform/conform shares between scaffold.Finish and
// assemble.Check, the same defect refused by one is reported by the
// other, and their <problem> text is byte-identical. internal/cli is the
// only package allowed to import both scaffold and assemble, so it is the
// only place this comparison can be written; it calls both Servers
// directly rather than through cli.Run, since the claim is about the two
// packages' own Problem strings, not about any rendering either command
// layers on top of them.

// stateWithUnterminatedFence carries every one of the default profile's
// four required headings, each followed by content, then an opened fence
// never closed — the headings all sit before the fence opens, so
// conform.MissingHeading still finds every one of them and only
// conform.UnterminatedFence's predicate fires.
const stateWithUnterminatedFence = "## Binding decisions\n\ndecision\n\n" +
	"## Left unbuilt\n\nsymbol\n\n" +
	"## Traps\n\ntrap\n\n" +
	"## Open debts\n\ndebt\n\n" +
	"```\nunterminated\n"

// stateMissingTraps carries three of the default profile's four required
// headings, "## Traps" omitted entirely — heading and section both absent,
// not merely emptied.
const stateMissingTraps = "## Binding decisions\n\ndecision\n\n" +
	"## Left unbuilt\n\nsymbol\n\n" +
	"## Open debts\n\ndebt\n"

// checklistDrift builds one feature's fixture whose only step,
// SCENARIO-01, carries status and a checklist holding one ticked item and
// one open item ("second thing", the drift assertion's own item text), and
// returns its root — callers vary only status between the two fixtures
// this predicate needs.
func checklistDrift(t *testing.T, status string) string {
	t.Helper()

	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "demo")
	writeCheckFixtureFeature(t, featureDir)
	writeCheckStep(t, featureDir, "SCENARIO-01", status, []string{"- [x] first thing", "- [ ] second thing"})

	return root
}

// writeCheckFixtureFeature writes a conforming specification and state
// file under featureDir — the drift tests' shared starting point before
// each one corrupts exactly the one input its predicate concerns.
func writeCheckFixtureFeature(t *testing.T, featureDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
}

func Test_check_drift_over_cap_handoff_matches_finishes_own_refusal(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "demo")
	writeCheckFixtureFeature(t, featureDir)
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	overCap := checkBodyOfLines(61)
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(overCap), 0o600))

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, root)
	finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte(overCap), []byte(conformingState))
	require.ErrorIs(t, finishErr, scaffold.ErrOverCap)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, root)
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "HANDOFF")
	assert.Equal(t, refusal.Problem, finding.Problem)
}

func Test_check_drift_unterminated_state_fence_matches_finishes_own_refusal(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "demo")
	writeCheckFixtureFeature(t, featureDir)
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(stateWithUnterminatedFence), 0o600))

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, root)
	finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(stateWithUnterminatedFence))
	require.ErrorIs(t, finishErr, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, root)
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "STATE.md")
	assert.Equal(t, refusal.Problem, finding.Problem)
}

func Test_check_drift_missing_state_heading_matches_finishes_own_refusal(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "demo")
	writeCheckFixtureFeature(t, featureDir)
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(stateMissingTraps), 0o600))

	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, root)
	finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(stateMissingTraps))
	require.ErrorIs(t, finishErr, scaffold.ErrMissingStateHeading)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	assembleSrv := assemble.NewServer(cfg, root)
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "STATE.md")
	assert.Equal(t, refusal.Problem, finding.Problem)
}

// Test_check_drift_open_checklist_item_matches_finishes_own_refusal is the
// one predicate that cannot pair on a single fixture: Finish refuses an
// OPEN step's unticked item, and Check only ever reports a DONE step's
// (an open step's is ordinary in-progress work). Two fixtures, differing
// only in status:, still prove one definition: the problem text depends on
// the item's own text alone, never on the step's status.
func Test_check_drift_open_checklist_item_matches_finishes_own_refusal(t *testing.T) {
	openRoot := checklistDrift(t, "open")
	cfg := config.Default()

	scaffoldSrv := scaffold.NewServer(cfg, openRoot)
	finishErr := scaffoldSrv.Finish(t.Context(), "demo", "SCENARIO-01", []byte("a fine handoff\n"), []byte(conformingState))
	require.ErrorIs(t, finishErr, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, finishErr, &refusal)

	doneRoot := checklistDrift(t, "done")
	assembleSrv := assemble.NewServer(cfg, doneRoot)
	findings, checkErr := assembleSrv.Check(t.Context(), "demo")
	require.NoError(t, checkErr)

	finding := findFinding(t, findings, "SCENARIO-01.md")
	assert.Equal(t, refusal.Problem, finding.Problem)
	assert.Contains(t, finding.Problem, "second thing")
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
