// check's plain scenarios — every one, except the one symlink case — run
// against an rwfs.Mem through withRootFS, reaching run() directly since
// that seam is unexported. check_disk_test.go keeps its own symlink case
// plus every helper this file and check_hook_disk_test.go/json_refusal_test.go
// still call. check --hook itself stays entirely on disk
// (check_hook_disk_test.go): runCheckHook's own FeatureContaining resolves
// through real os.Lstat/filepath.EvalSymlinks with no seam.

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memCheckBodyOfLines mirrors check_test.go's own checkBodyOfLines.
func memCheckBodyOfLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return strings.Join(lines, "\n") + "\n"
}

// memOverCapStateLines mirrors check_test.go's own overCapStateLines: one
// line past config.Default's state-cap-lines (80).
const memOverCapStateLines = 81

// memOverCapState mirrors check_test.go's own overCapState.
func memOverCapState(n int) string {
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

// memCheckStep writes one step file under featureDir into tree, using the
// default step-file-pattern's naming — mirroring check_test.go's own
// writeCheckStep.
func memCheckStep(tree *memTree, featureDir, id, status string, checklistItems []string) {
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
	tree.file(filepath.Join(featureDir, id+".md"), step)
}

// runCheckMem runs "check" (or the args given) against tree through
// run()'s own withRootFS seam.
func runCheckMem(t *testing.T, tree *memTree, args []string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, &stdout, &stderr, noBuildInfo, withRootFS(tree.mem()))

	return stdout.String(), stderr.String(), err
}

func Test_check_prints_findings_on_stdout_and_exits_1_for_an_error_finding_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})
	tree.file(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	stdout, stderr, err := runCheckMem(t, tree, []string{"check"})

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, ""+
		"demo  (in flight)\n"+
		"  ERROR  "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout)
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 handoff-cap); ERRORs block finish on in-flight features\n", stderr)
}

func Test_check_groups_two_features_with_a_blank_line_and_counts_rules_by_count_then_id_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	alphaDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	tree.
		file(filepath.Join(alphaDir, "specification.md"), memConformingSpec).
		file(filepath.Join(alphaDir, "STATE.md"), memOverCapState(memOverCapStateLines))
	memCheckStep(tree, alphaDir, "SCENARIO-01", "done", []string{"- [x] first thing", "- [ ] second thing"})

	betaDir := filepath.Join(memRoot, "docs", "specifications", "beta")
	tree.
		file(filepath.Join(betaDir, "specification.md"), memConformingSpec).
		file(filepath.Join(betaDir, "STATE.md"), memOverCapState(memOverCapStateLines))
	memCheckStep(tree, betaDir, "SCENARIO-01", "done", []string{"- [x] first thing"})
	tree.file(filepath.Join(betaDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	stdout, stderr, err := runCheckMem(t, tree, []string{"check"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"alpha  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "alpha", "STATE.md")+":81  state is 81 lines, over the cap of 80\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "alpha", "SCENARIO-01.md")+":16  checklist item \"second thing\" is not ticked\n"+
		"\n"+
		"beta  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "beta", "STATE.md")+":81  state is 81 lines, over the cap of 80\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "beta", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout)
	assert.Equal(t,
		"brief check: 0 ERROR, 4 WARN in 2 features (2 state-cap, 1 checklist, 1 handoff-cap); 'brief check <feature>' narrows to one\n",
		stderr)
}

func Test_check_omits_the_line_suffix_for_a_whole_file_finding_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.file(filepath.Join(featureDir, "specification.md"), memConformingSpec)

	stdout, _, err := runCheckMem(t, tree, []string{"check"})

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout, "\n  ERROR  "+filepath.Join("docs", "specifications", "demo", "STATE.md")+"  ")
	assert.NotContains(t, stdout, "STATE.md:0")
}

func Test_check_summary_drops_the_ERRORs_clause_when_every_finding_is_WARN_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	tree.file(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	stdout, stderr, err := runCheckMem(t, tree, []string{"check"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"demo  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout)
	assert.Equal(t, "brief check: 0 ERROR, 1 WARN in 1 feature (1 handoff-cap)\n", stderr)
}

func Test_check_summary_drops_the_narrowing_clause_for_a_named_feature_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})
	tree.file(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	_, stderr, err := runCheckMem(t, tree, []string{"check", "alpha"})

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 handoff-cap); ERRORs block finish on in-flight features\n", stderr)
}

func Test_check_reports_no_findings_and_exits_0_for_a_conforming_repository_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	stdout, stderr, err := runCheckMem(t, tree, []string{"check"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
}

func Test_check_says_no_findings_and_exits_0_for_a_repository_with_no_features_mem(t *testing.T) {
	stdout, stderr, err := runCheckMem(t, newMemTree(memRoot), []string{"check"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
}

func Test_check_refuses_an_unknown_feature_with_no_files_changed_tail_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	stdout, stderr, err := runCheckMem(t, tree, []string{"check", "ghost"})

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
	assert.NotContains(t, stderr, "(no files changed)")
}

func Test_check_names_the_feature_in_the_no_findings_notice_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	stdout, stderr, err := runCheckMem(t, tree, []string{"check", "alpha"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, "brief check: alpha: no findings\n", stderr)
}

func Test_check_scopes_to_the_named_feature_only_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	goodDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	tree.
		file(filepath.Join(goodDir, "specification.md"), memConformingSpec).
		file(filepath.Join(goodDir, "STATE.md"), memConformingState)
	memCheckStep(tree, goodDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	badDir := filepath.Join(memRoot, "docs", "specifications", "beta")
	tree.
		file(filepath.Join(badDir, "specification.md"), "# beta\n\nno progress list\n").
		file(filepath.Join(badDir, "STATE.md"), memConformingState)

	stdout, _, err := runCheckMem(t, tree, []string{"check", "alpha"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stdout)
}

func Test_returns_a_usage_error_when_check_is_given_two_arguments_mem(t *testing.T) {
	stdout, stderr, err := runCheckMem(t, newMemTree(memRoot), []string{"check", "alpha", "beta"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
}

func Test_returns_a_usage_error_when_check_is_given_an_undefined_flag_mem(t *testing.T) {
	stdout, _, err := runCheckMem(t, newMemTree(memRoot), []string{"check", "--bogus"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout)
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_check_mem(t *testing.T) {
	stdout, stderr, err := runCheckMem(t, newMemTree(memRoot), []string{"check", "--help"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "brief check")
}

// newMemCheckJSONFixture mirrors check_json_test.go's own
// newCheckJSONFixture: two features named so fs.ReadDir's byte order is
// also Check's and the golden's own feature order — "alpha" still in
// flight (an open step) with an over-cap state file, "beta" fully done
// with no handoff or cap issue.
func newMemCheckJSONFixture() *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	alphaDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	tree.
		file(filepath.Join(alphaDir, "specification.md"), memConformingSpec).
		file(filepath.Join(alphaDir, "STATE.md"), memOverCapState(memOverCapStateLines))
	memCheckStep(tree, alphaDir, "SCENARIO-01", "open", nil)

	betaDir := filepath.Join(memRoot, "docs", "specifications", "beta")
	tree.file(filepath.Join(betaDir, "specification.md"), memConformingSpec)
	memCheckStep(tree, betaDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	return tree
}

// Test_check_json_document_golden_mem is the exact-bytes golden pinning
// checkDocument's key order: an in-flight feature's ERROR finding
// carrying a line ("alpha") and a done feature's WARN, whole-file finding
// ("beta", "line":null). detail and path are captured from
// assemble.NewServer(...).Check against the same fixture, never a
// production literal — mirroring check_json_test.go's own
// Test_check_json_document_golden.
func Test_check_json_document_golden_mem(t *testing.T) {
	tree := newMemCheckJSONFixture()
	mem := tree.mem()

	findings, checkErr := assemble.NewServer(config.Default(), memRoot, assemble.WithFS(mem)).Check(t.Context(), "")
	require.NoError(t, checkErr)
	alphaFinding := memFindFinding(t, findings, "alpha")
	betaFinding := memFindFinding(t, findings, "beta")

	var stdout, stderr strings.Builder
	err := run(t.Context(), memRoot, []string{"check", "--json"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stderr.String())

	alphaDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	betaDir := filepath.Join(memRoot, "docs", "specifications", "beta")

	want := `{"schema":1,"command":"check","ok":false,"exit_code":1,` +
		`"counts":{"error":1,"warn":1},` +
		`"features":[` +
		`{"name":"alpha","path":` + memJSONString(t, alphaDir) + `,"in_flight":true,"findings":[` +
		`{"severity":"ERROR","rule":"state-cap","path":` + memJSONString(t, alphaFinding.Path) + `,"line":81,"detail":` + memJSONString(t, alphaFinding.Detail) + `}]},` +
		`{"name":"beta","path":` + memJSONString(t, betaDir) + `,"in_flight":false,"findings":[` +
		`{"severity":"WARN","rule":"state-missing","path":` + memJSONString(t, betaFinding.Path) + `,"line":null,"detail":` + memJSONString(t, betaFinding.Detail) + `}]}` +
		`]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

func Test_check_json_with_no_findings_mem(t *testing.T) {
	cases := []struct {
		name string
		tree *memTree
	}{
		{
			name: "conforming repository",
			tree: func() *memTree {
				tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
				featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
				tree.
					file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
					file(filepath.Join(featureDir, "STATE.md"), memConformingState)
				memCheckStep(tree, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

				return tree
			}(),
		},
		{name: "no features", tree: newMemTree(memRoot)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, err := runCheckMem(t, c.tree, []string{"check", "--json"})

			require.NoError(t, err)
			assert.Equal(t, 0, ExitCode(err))

			want := `{"schema":1,"command":"check","ok":true,"exit_code":0,"counts":{"error":0,"warn":0},"features":[]}` + "\n"
			assert.Equal(t, want, stdout)
			assert.Empty(t, stderr)
		})
	}

	_, textStderr, textErr := runCheckMem(t, newMemTree(memRoot), []string{"check"})

	require.NoError(t, textErr)
	assert.Equal(t, "brief check: no findings\n", textStderr)
}

func Test_check_json_warn_only_is_ok_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	tree.file(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	stdout, stderr, err := runCheckMem(t, tree, []string{"check", "--json"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stderr)

	var doc struct {
		OK       bool `json:"ok"`
		ExitCode int  `json:"exit_code"`
		Counts   struct {
			Error int `json:"error"`
			Warn  int `json:"warn"`
		} `json:"counts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	assert.True(t, doc.OK)
	assert.Equal(t, 0, doc.ExitCode)
	assert.Equal(t, 0, doc.Counts.Error)
	assert.Positive(t, doc.Counts.Warn)
}

func Test_check_json_scopes_to_the_named_feature_mem(t *testing.T) {
	stdout, stderr, err := runCheckMem(t, newMemCheckJSONFixture(), []string{"check", "alpha", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stderr)

	var doc struct {
		Counts struct {
			Error int `json:"error"`
			Warn  int `json:"warn"`
		} `json:"counts"`
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	assert.Equal(t, 1, doc.Counts.Error)
	assert.Equal(t, 0, doc.Counts.Warn)
	require.Len(t, doc.Features, 1)
	assert.Equal(t, "alpha", doc.Features[0].Name)
}

// memFindFinding returns the one finding in findings whose Path contains
// substr, failing the test when there is not exactly one — mirroring
// check_drift_test.go's own findFinding, duplicated since that is a
// package cli_test symbol.
func memFindFinding(t *testing.T, findings []assemble.Finding, substr string) assemble.Finding {
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
