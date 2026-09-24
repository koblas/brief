// status's plain scenarios — every one that only reads feature content a
// fixture controls, no real containment or permission behavior — run
// against an rwfs.Mem through withRootFS, reaching run() directly since
// that seam is unexported. status_test.go's own symlink case (real
// containment: a symlink entry in the feature root, never followed) stays
// on disk; its own helpers (writeConformingFeatureFiles, stepTitle,
// writeStatusStep, writeMalformedStatusFeature) and status_json_test.go's
// own newStatusJSONFixture stay defined there too — json_refusal_test.go
// and help_test.go, both out of this pass's scope, still call them. This
// file builds its own mem* equivalents rather than reusing those: a
// package cli_test symbol is not visible from this package cli file.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStepTitle mirrors status_test.go's own stepTitle: the heading a step
// fixture carries, deliberately distinct from its id.
func memStepTitle(id string) string {
	return "Implement " + id
}

// memConformingSpec and memConformingState mirror check_test.go's own
// conformingSpec/conformingState — duplicated here since a package
// cli_test constant is not visible from this package cli file.
// memStateWithUnterminatedFence mirrors check_drift_test.go's own
// stateWithUnterminatedFence the same way.
const (
	memConformingSpec  = "# demo\n\n## BDD Acceptance Progress\n\n- [x] SCENARIO-01\n"
	memConformingState = "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	memStateWithUnterminatedFence = "## Binding decisions\n\ndecision\n\n" +
		"## Left unbuilt\n\nsymbol\n\n" +
		"## Traps\n\ntrap\n\n" +
		"## Open debts\n\ndebt\n\n" +
		"```\nunterminated\n"
)

// memConformingFeatureFiles adds featureDir's specification and state file
// to tree, conforming to every read-side check assemble.Start applies —
// mirroring status_test.go's own writeConformingFeatureFiles.
func memConformingFeatureFiles(tree *memTree, featureDir string) {
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
}

// memStepBody renders one step file's body: id echoed as both frontmatter
// id and title (memStepTitle), status and depends-on as given — mirroring
// status_test.go's own writeStatusStep body shape.
func memStepBody(id, status string, dependsOn []string) string {
	deps := "depends-on: []\n"
	if len(dependsOn) > 0 {
		var sb strings.Builder

		sb.WriteString("depends-on:\n")
		for _, dep := range dependsOn {
			sb.WriteString("  - " + dep + "\n")
		}

		deps = sb.String()
	}

	return "---\n" +
		"id: " + id + "\n" +
		"status: " + status + "\n" +
		deps +
		"---\n\n" +
		"# " + memStepTitle(id) + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
}

// memStatusStep adds one step file for feature, under root's default
// feature-directory layout, after seeding a conforming specification and
// state file for feature if neither is already present — mirroring
// status_test.go's own writeStatusStep, MAJOR 1's own requirement that a
// status fixture carry the same two files assemble.Start's read-side
// checks require.
func memStatusStep(tree *memTree, root, feature, name, id, status string, dependsOn []string) {
	featureDir := filepath.Join(root, "docs", "specifications", feature)
	memConformingFeatureFiles(tree, featureDir)

	tree.file(filepath.Join(featureDir, name), memStepBody(id, status, dependsOn))
}

// newMemStatusFixture builds an rwfs.Mem holding three features under
// memRoot's default layout — mirroring status_test.go's own
// newStatusFixture: "alpha" (1/3 done, next SCENARIO-02, nothing
// blocked), "beta" (3/3 done, complete) and "gamma" (1/4 done, next
// SCENARIO-02, one step blocked on gamma's own unfinished SCENARIO-02).
func newMemStatusFixture() *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	memStatusStep(tree, memRoot, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-03.md", "SCENARIO-03", "open", nil)

	memStatusStep(tree, memRoot, "beta", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "beta", "SCENARIO-02.md", "SCENARIO-02", "done", nil)
	memStatusStep(tree, memRoot, "beta", "SCENARIO-03.md", "SCENARIO-03", "done", nil)

	memStatusStep(tree, memRoot, "gamma", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "gamma", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	memStatusStep(tree, memRoot, "gamma", "SCENARIO-03.md", "SCENARIO-03", "open", []string{"SCENARIO-02"})
	memStatusStep(tree, memRoot, "gamma", "SCENARIO-04.md", "SCENARIO-04", "open", nil)

	return tree
}

// memMalformedStatusFeature adds a conforming specification and state
// file, then one step file with no frontmatter at all, under feature's
// default layout — mirroring status_test.go's own
// writeMalformedStatusFeature: the row's Problem must land on the step
// file's own frontmatter fault, not on a spec/state fault this fixture
// does not mean to exercise.
func memMalformedStatusFeature(tree *memTree, root, feature string) {
	featureDir := filepath.Join(root, "docs", "specifications", feature)
	memConformingFeatureFiles(tree, featureDir)

	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), "no frontmatter here\n")
}

// runStatusMem runs "status" (or the args given) against tree through
// run()'s own withRootFS seam.
func runStatusMem(t *testing.T, tree *memTree, args []string) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	err := run(t.Context(), memRoot, args, nil, &stdout, &stderr, noBuildInfo, withRootFS(tree.mem()))

	return stdout.String(), stderr.String(), err
}

func Test_status_prints_the_table_and_nothing_else_mem(t *testing.T) {
	tree := newMemStatusFixture()

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"gamma    1/4   1        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n",
		stdout)
	assert.Equal(t, "brief status: 3 features: 2 in progress, 1 complete, 0 malformed\n", stderr)
}

func Test_status_prints_a_table_for_a_single_feature_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/2   0        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n",
		stdout)
	assert.Equal(t, "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n", stderr)
}

func Test_status_shows_the_complete_marker_for_a_completed_feature_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    2/2   0        (complete)\n",
		stdout)
	assert.Equal(t, "brief status: 1 feature: 0 in progress, 1 complete, 0 malformed\n", stderr)
}

func Test_status_says_no_features_were_found_when_the_feature_root_is_absent_mem(t *testing.T) {
	tree := newMemTree(memRoot)

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr)
}

func Test_status_says_no_features_were_found_when_the_feature_root_is_empty_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr)
}

// Test_status_reports_the_other_features_unchanged_when_one_is_malformed_mem
// is the control arm for SCENARIO-11's core claim: two runs differing in
// exactly one variable — the malformed feature present or absent — both
// asserted against literal expected stdout strings.
func Test_status_reports_the_other_features_unchanged_when_one_is_malformed_mem(t *testing.T) {
	stdoutWithout, _, errWithout := runStatusMem(t, newMemStatusFixture(), []string{"status"})

	require.NoError(t, errWithout)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"gamma    1/4   1        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n",
		stdoutWithout)

	treeWith := newMemStatusFixture()
	memMalformedStatusFeature(treeWith, memRoot, "delta")
	stdoutWith, _, errWith := runStatusMem(t, treeWith, []string{"status"})

	require.NoError(t, errWith)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"delta    -     -        (malformed, see below)\n"+
		"gamma    1/4   1        SCENARIO-02  "+memStepTitle("SCENARIO-02")+"\n",
		stdoutWith)
}

// Test_status_names_the_reason_for_a_malformed_feature_on_stderr_mem is
// the representative mutation-guard case for this command's own WithFS and
// resolveRoot seams — see the mutation notes in this package's mem_internal_test.go
// commit history for how each was verified.
func Test_status_names_the_reason_for_a_malformed_feature_on_stderr_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memMalformedStatusFeature(tree, memRoot, "delta")

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})
	_ = stdout

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"brief status: delta: "+filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")+
		": frontmatter does not parse: no frontmatter found; run 'brief check delta' to list every fault\n"+
		"brief status: 1 feature: 0 in progress, 0 complete, 1 malformed\n",
		stderr)
	assert.NotContains(t, stderr, "(no files changed)")
}

func Test_status_names_the_line_of_a_state_file_s_unclosed_fence_on_stderr_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memStateWithUnterminatedFence).
		file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))

	_, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	wantPath := filepath.Join("docs", "specifications", "demo", "STATE.md")
	assert.Contains(t, stderr, "brief status: demo: "+wantPath+":17: ")
}

func Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memMalformedStatusFeature(tree, memRoot, "delta")

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"delta    -     -        (malformed, see below)\n",
		stdout)
	assert.NotContains(t, stderr, "no features found")
	assert.Equal(t, 2, strings.Count(stderr, "\n"))
}

func Test_status_on_a_repository_whose_only_entry_is_a_regular_file_still_says_no_features_found_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	tree.file(filepath.Join(memRoot, "docs", "specifications", "README.md"), "not a feature\n")

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr)
}

func Test_status_writes_one_stderr_line_per_malformed_feature_then_the_summary_mem(t *testing.T) {
	tree := newMemStatusFixture()
	memMalformedStatusFeature(tree, memRoot, "delta")
	memMalformedStatusFeature(tree, memRoot, "epsilon")

	_, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, 3, strings.Count(stderr, "\n"))

	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md"))
	assert.Contains(t, lines[1], filepath.Join("docs", "specifications", "epsilon", "SCENARIO-01.md"))
	assert.Equal(t, "brief status: 5 features: 2 in progress, 1 complete, 2 malformed", lines[2])
}

// Test_status_writes_the_table_before_the_malformed_lines_and_the_summary_mem
// passes ONE shared buffer as both stdout and stderr: only a shared buffer
// pins that the table is fully written before any stderr byte.
func Test_status_writes_the_table_before_the_malformed_lines_and_the_summary_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-01.md", "SCENARIO-01", "open", nil)
	memMalformedStatusFeature(tree, memRoot, "delta")

	var shared bytes.Buffer

	err := run(t.Context(), memRoot, []string{"status"}, nil, &shared, &shared, noBuildInfo, withRootFS(tree.mem()))

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    0/1   0        SCENARIO-01  "+memStepTitle("SCENARIO-01")+"\n"+
		"delta    -     -        (malformed, see below)\n"+
		"brief status: delta: "+filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")+
		": frontmatter does not parse: no frontmatter found; run 'brief check delta' to list every fault\n"+
		"brief status: 2 features: 1 in progress, 0 complete, 1 malformed\n",
		shared.String())
}

func Test_status_summary_mem(t *testing.T) {
	cases := []struct {
		name     string
		features map[string]string
		want     string
	}{
		{
			name:     "one feature is singular",
			features: map[string]string{"alpha": "open"},
			want:     "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n",
		},
		{
			name:     "all features complete leaves in-progress and malformed at zero",
			features: map[string]string{"alpha": "done"},
			want:     "brief status: 1 feature: 0 in progress, 1 complete, 0 malformed\n",
		},
		{
			name:     "a zero-step feature counts as in progress, not complete",
			features: map[string]string{"alpha": ""},
			want:     "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

			for feature, status := range c.features {
				featureDir := filepath.Join(memRoot, "docs", "specifications", feature)
				memConformingFeatureFiles(tree, featureDir)

				if status != "" {
					memStatusStep(tree, memRoot, feature, "SCENARIO-01.md", "SCENARIO-01", status, nil)
				}
			}

			_, stderr, err := runStatusMem(t, tree, []string{"status"})

			require.NoError(t, err)
			assert.Equal(t, c.want, stderr)
		})
	}
}

func Test_returns_a_usage_error_when_status_is_given_an_argument_mem(t *testing.T) {
	stdout, stderr, err := runStatusMem(t, newMemTree(memRoot), []string{"status", "alpha"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "brief status")
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
}

func Test_returns_a_usage_error_when_status_is_given_an_undefined_flag_mem(t *testing.T) {
	stdout, stderr, err := runStatusMem(t, newMemTree(memRoot), []string{"status", "--bogus"})

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "brief status")
	assert.Equal(t, 1, strings.Count(stderr, "\n"))
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_status_mem(t *testing.T) {
	stdout, stderr, err := runStatusMem(t, newMemTree(memRoot), []string{"status", "--help"})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "malformed")
	assert.Contains(t, stdout, "exits 0")
}

func Test_status_text_marks_a_feature_missing_its_specification_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "STATE.md"), memConformingState).
		file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"demo     -     -        (malformed, see below)\n",
		stdout)
	assert.Contains(t, stderr,
		"brief status: demo: "+filepath.Join("docs", "specifications", "demo", "specification.md")+
			`: specification.md not found; write a specification.md with a "## BDD Acceptance Progress" heading and re-run`)
	assert.Contains(t, stderr, "brief status: 1 feature: 0 in progress, 0 complete, 1 malformed")
}

func Test_status_text_marks_a_feature_whose_specification_has_no_progress_heading_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), "# demo\n\nno progress list here\n").
		file(filepath.Join(featureDir, "STATE.md"), memConformingState).
		file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"demo     -     -        (malformed, see below)\n",
		stdout)
	assert.Contains(t, stderr,
		"brief status: demo: "+filepath.Join("docs", "specifications", "demo", "specification.md")+
			`: no "## BDD Acceptance Progress" heading found; add a "## BDD Acceptance Progress" heading to the specification`)
	assert.Contains(t, stderr, "brief status: 1 feature: 0 in progress, 0 complete, 1 malformed")
}

func Test_status_text_marks_a_feature_missing_its_state_file_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))

	stdout, stderr, err := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"demo     -     -        (malformed, see below)\n",
		stdout)
	assert.Contains(t, stderr, "brief status: demo: "+filepath.Join("docs", "specifications", "demo", "STATE.md")+": ")
	assert.Contains(t, stderr, "; make it readable and re-run")
	assert.Contains(t, stderr, "brief status: 1 feature: 0 in progress, 0 complete, 1 malformed")
}

func Test_status_json_marks_a_feature_for_each_of_major_1s_three_conditions_mem(t *testing.T) {
	// missingFileDetail is fstest.MapFS's own missing-file wording — "file
	// does not exist", fs.ErrNotExist's own Error() text — reached through
	// rwfs.Mem's ReadFile wrap and newProblem's own *fs.PathError unwrap;
	// deliberately not the real OS's "no such file or directory", since
	// this fixture never touches real disk.
	missingFileDetail := "file does not exist"

	cases := []struct {
		name         string
		setup        func(tree *memTree, featureDir string)
		wantDetailIn string
	}{
		{
			name: "missing specification",
			setup: func(tree *memTree, featureDir string) {
				tree.
					file(filepath.Join(featureDir, "STATE.md"), memConformingState).
					file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))
			},
			wantDetailIn: "specification.md not found",
		},
		{
			name: "specification has no progress heading",
			setup: func(tree *memTree, featureDir string) {
				tree.
					file(filepath.Join(featureDir, "specification.md"), "# demo\n\nno progress list here\n").
					file(filepath.Join(featureDir, "STATE.md"), memConformingState).
					file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))
			},
			wantDetailIn: `no "## BDD Acceptance Progress" heading found`,
		},
		{
			name: "missing state file",
			setup: func(tree *memTree, featureDir string) {
				tree.
					file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
					file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))
			},
			wantDetailIn: missingFileDetail,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
			c.setup(tree, filepath.Join(memRoot, "docs", "specifications", "demo"))

			stdout, stderr, err := runStatusMem(t, tree, []string{"status", "--json"})

			require.NoError(t, err)
			assert.Empty(t, stderr)

			var doc struct {
				Features []map[string]json.RawMessage `json:"features"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
			require.Len(t, doc.Features, 1)

			row := doc.Features[0]
			assert.Equal(t, "null", string(row["done"]))
			assert.Equal(t, "null", string(row["total"]))
			assert.Equal(t, "null", string(row["blocked"]))
			assert.Equal(t, "null", string(row["next"]))
			assert.Equal(t, "false", string(row["complete"]))

			require.NotEqual(t, "null", string(row["problem"]))

			var problem struct {
				Path   string `json:"path"`
				Detail string `json:"detail"`
			}
			require.NoError(t, json.Unmarshal(row["problem"], &problem))
			assert.Equal(t, c.wantDetailIn, problem.Detail)
		})
	}
}

func Test_status_json_leaves_a_conforming_feature_s_counts_non_null_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memStatusStep(tree, memRoot, "demo", "SCENARIO-01.md", "SCENARIO-01", "open", nil)

	stdout, stderr, err := runStatusMem(t, tree, []string{"status", "--json"})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	var doc struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	require.Len(t, doc.Features, 1)

	row := doc.Features[0]
	assert.Equal(t, "0", string(row["done"]))
	assert.Equal(t, "1", string(row["total"]))
	assert.Equal(t, "0", string(row["blocked"]))
	assert.Equal(t, "null", string(row["problem"]))
}

func Test_status_json_reports_the_line_of_a_state_file_s_unclosed_fence_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memStateWithUnterminatedFence).
		file(filepath.Join(featureDir, "SCENARIO-01.md"), memStepBody("SCENARIO-01", "open", nil))

	stdout, _, err := runStatusMem(t, tree, []string{"status", "--json"})

	require.NoError(t, err)

	var doc struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	require.Len(t, doc.Features, 1)

	var problem struct {
		Line *int `json:"line"`
	}
	require.NoError(t, json.Unmarshal(doc.Features[0]["problem"], &problem))
	require.NotNil(t, problem.Line)
	assert.Equal(t, 17, *problem.Line)
}

// newMemStatusJSONFixture mirrors status_json_test.go's own
// newStatusJSONFixture: four features named so fs.ReadDir's byte order is
// also the golden order — "alpha" (1/4 done, next SCENARIO-02, one step
// blocked on its own unfinished SCENARIO-02), "beta" (2/2 done, complete),
// "delta" (malformed — no frontmatter) and "epsilon" (a bare feature
// directory with no step files at all).
func newMemStatusJSONFixture() *memTree {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))

	memStatusStep(tree, memRoot, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-03.md", "SCENARIO-03", "open", []string{"SCENARIO-02"})
	memStatusStep(tree, memRoot, "alpha", "SCENARIO-04.md", "SCENARIO-04", "open", nil)

	memStatusStep(tree, memRoot, "beta", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	memStatusStep(tree, memRoot, "beta", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	memMalformedStatusFeature(tree, memRoot, "delta")

	memConformingFeatureFiles(tree, filepath.Join(memRoot, "docs", "specifications", "epsilon"))

	return tree
}

// Test_status_json_document_golden_mem is the exact-bytes golden pinning
// statusDocument's key order: an in-progress feature with a blocked step
// and a non-null next, a complete feature (next null), a malformed feature
// (counts null, a problem object, line null) and a well-formed zero-step
// feature (0/0/0, complete false, next null).
func Test_status_json_document_golden_mem(t *testing.T) {
	mem := newMemStatusJSONFixture().mem()

	var textStdout, textStderr bytes.Buffer
	textErr := run(t.Context(), memRoot, []string{"status"}, nil, &textStdout, &textStderr, noBuildInfo, withRootFS(mem))
	require.NoError(t, textErr)

	deltaRelStep := filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")
	prefix := "brief status: delta: " + deltaRelStep + ": "
	line, _, _ := strings.Cut(textStderr.String(), "\n")
	require.True(t, strings.HasPrefix(line, prefix), "line %q missing prefix %q", line, prefix)
	rest := strings.TrimPrefix(line, prefix)
	parts := strings.SplitN(rest, "; ", 2)
	require.Len(t, parts, 2)
	detail, fix := parts[0], parts[1]

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), memRoot, []string{"status", "--json"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Empty(t, stderr.String())

	featureDirAlpha := filepath.Join(memRoot, "docs", "specifications", "alpha")
	featureDirBeta := filepath.Join(memRoot, "docs", "specifications", "beta")
	featureDirDelta := filepath.Join(memRoot, "docs", "specifications", "delta")
	featureDirEpsilon := filepath.Join(memRoot, "docs", "specifications", "epsilon")
	nextPathAlpha := filepath.Join(featureDirAlpha, "SCENARIO-02.md")
	problemPathDelta := filepath.Join(featureDirDelta, "SCENARIO-01.md")

	want := `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[` +
		`{"name":"alpha","path":` + memJSONString(t, featureDirAlpha) + `,"done":1,"total":4,"blocked":1,"complete":false,` +
		`"next":{"id":"SCENARIO-02","title":` + memJSONString(t, memStepTitle("SCENARIO-02")) + `,"path":` + memJSONString(t, nextPathAlpha) + `},"problem":null},` +
		`{"name":"beta","path":` + memJSONString(t, featureDirBeta) + `,"done":2,"total":2,"blocked":0,"complete":true,"next":null,"problem":null},` +
		`{"name":"delta","path":` + memJSONString(t, featureDirDelta) + `,"done":null,"total":null,"blocked":null,"complete":false,"next":null,` +
		`"problem":{"path":` + memJSONString(t, problemPathDelta) + `,"line":null,"detail":` + memJSONString(t, detail) + `,"fix":` + memJSONString(t, fix) + `}},` +
		`{"name":"epsilon","path":` + memJSONString(t, featureDirEpsilon) + `,"done":0,"total":0,"blocked":0,"complete":false,"next":null,"problem":null}` +
		`]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_status_json_with_no_features_mem covers R9's empty discriminator in
// JSON form for both zero-row causes: the feature root missing entirely and
// the feature root present but empty — both render "features":[], never
// null, with zero stderr bytes. The control arm proves the empty stderr is
// the --json branch, not the zero-rows notice going missing for some other
// reason: the same fixture's text-mode run still writes the notice.
func Test_status_json_with_no_features_mem(t *testing.T) {
	cases := []struct {
		name string
		tree *memTree
	}{
		{name: "feature root absent", tree: newMemTree(memRoot)},
		{name: "feature root empty", tree: newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, err := runStatusMem(t, c.tree, []string{"status", "--json"})

			require.NoError(t, err)
			assert.Equal(t, 0, ExitCode(err))

			want := `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[]}` + "\n"
			assert.Equal(t, want, stdout)
			assert.Empty(t, stderr)
		})
	}

	_, textStderr, textErr := runStatusMem(t, newMemTree(memRoot), []string{"status"})

	require.NoError(t, textErr)
	assert.NotEmpty(t, textStderr)
}

// Test_status_json_keeps_the_next_title_raw_mem pins that --json's
// next.title carries the step heading exactly as markdown.Title returns
// it, interior tab included, while the text-mode NEXT column flattens that
// same tab to a space — the control arm proving the two render
// differently for the identical fixture, only --json differing.
func Test_status_json_keeps_the_next_title_raw_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	rawTitle := "Implement\tSCENARIO-01"
	featureDir := filepath.Join(memRoot, "docs", "specifications", "alpha")
	memConformingFeatureFiles(tree, featureDir)
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# " + rawTitle + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
	tree.file(filepath.Join(featureDir, "SCENARIO-01.md"), step)

	textStdout, _, textErr := runStatusMem(t, tree, []string{"status"})

	require.NoError(t, textErr)
	assert.Contains(t, textStdout, "Implement SCENARIO-01")
	assert.NotContains(t, textStdout, rawTitle)

	stdout, _, err := runStatusMem(t, tree, []string{"status", "--json"})

	require.NoError(t, err)
	var doc struct {
		Features []struct {
			Next *struct {
				Title string `json:"title"`
			} `json:"next"`
		} `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	require.Len(t, doc.Features, 1)
	require.NotNil(t, doc.Features[0].Next)
	assert.Equal(t, rawTitle, doc.Features[0].Next.Title)
}

// Test_status_json_counts_are_null_only_on_a_malformed_row_mem decodes
// each row into a map so "null" is distinguishable from the number 0: a
// malformed row's done/total/blocked are literally null and its path is
// still the absolute feature directory; a well-formed zero-step row's are
// literally 0, the one variable — malformed or not — the two rows differ
// on.
func Test_status_json_counts_are_null_only_on_a_malformed_row_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memMalformedStatusFeature(tree, memRoot, "delta")
	memConformingFeatureFiles(tree, filepath.Join(memRoot, "docs", "specifications", "epsilon"))

	stdout, _, err := runStatusMem(t, tree, []string{"status", "--json"})

	require.NoError(t, err)
	var doc struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	require.Len(t, doc.Features, 2)

	delta := doc.Features[0]
	assert.Equal(t, "null", string(delta["done"]))
	assert.Equal(t, "null", string(delta["total"]))
	assert.Equal(t, "null", string(delta["blocked"]))
	assert.Equal(t, memJSONString(t, filepath.Join(memRoot, "docs", "specifications", "delta")), string(delta["path"]))

	epsilon := doc.Features[1]
	assert.Equal(t, "0", string(epsilon["done"]))
	assert.Equal(t, "0", string(epsilon["total"]))
	assert.Equal(t, "0", string(epsilon["blocked"]))
}
