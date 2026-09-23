package setup

// White-box package: addWorkflowSkill and removeWorkflowSkill are
// unexported, pinned here directly against hand-built frontmatter bytes,
// independent of planBoundAgents' own file-finding and membership decisions
// (bound_agent_test.go, black-box). planBoundAgent, planBoundAgentRemoval,
// confinedAgentFile (.read, .write) and verifyBoundAgentUnchanged are also
// called or constructed directly here rather than through
// srv.Init/srv.Uninstall, for inputs their real flow can never construct — a
// dangling symlink, unparseable frontmatter, or a symlink re-pointed after
// boundAgentTargets already selected a path (a directory component
// swapped for one pointing outside resolvedRoot, or a leaf re-pointed at a
// decoy): agentfile.Find filters every candidate through a successful
// decode before it is ever a target, so each of these is reachable in
// production only through the same kind of TOCTOU window between Find's own
// scan (or planning) and this package's own next read or write, never
// through the exported entry points' own real flow.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_add_workflow_skill_edits_only_the_skills_line pins addWorkflowSkill's
// full shape matrix: alreadyListed is caller-supplied (the same loose
// decode LackingSkill uses), never decided by the transform itself, so
// every "already listed" row below passes it true regardless of the line's
// own text shape. Every other row passes false and lets the transform
// classify the "skills:" key's own shape from body's own bytes.
func Test_add_workflow_skill_edits_only_the_skills_line(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		alreadyListed bool
		wantBody      string
		wantLine      string
		wantShape     skillsShape
	}{
		{
			name:      "no top-level key, LF file: insert before the closing delimiter",
			body:      "---\nname: developer\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeNoKey,
		},
		{
			name:      "no top-level key, CRLF file: insert keeps the CR terminator",
			body:      "---\r\nname: developer\r\n---\r\n\r\nbody\r\n",
			wantBody:  "---\r\nname: developer\r\nskills: [brief-workflow]\r\n---\r\n\r\nbody\r\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeNoKey,
		},
		{
			name:      "a nested skills: under another key is not top-level: insert",
			body:      "---\nname: developer\nmetadata:\n  skills: [x]\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nmetadata:\n  skills: [x]\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeNoKey,
		},
		{
			name:      "skills: text inside a description block scalar is not top-level: insert",
			body:      "---\nname: developer\ndescription: |\n  something about skills: here\n---\n\nbody\n",
			wantBody:  "---\nname: developer\ndescription: |\n  something about skills: here\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeNoKey,
		},
		{
			name:      "block list, 0-space indent",
			body:      "---\nname: developer\nskills:\n- other-skill\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n- other-skill\n- brief-workflow\n---\n\nbody\n",
			wantLine:  "- brief-workflow",
			wantShape: shapeBlockList,
		},
		{
			name:      "block list, 2-space indent",
			body:      "---\nname: developer\nskills:\n  - other-skill\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n  - other-skill\n  - brief-workflow\n---\n\nbody\n",
			wantLine:  "  - brief-workflow",
			wantShape: shapeBlockList,
		},
		{
			name:      "block list, 4-space indent",
			body:      "---\nname: developer\nskills:\n    - other-skill\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n    - other-skill\n    - brief-workflow\n---\n\nbody\n",
			wantLine:  "    - brief-workflow",
			wantShape: shapeBlockList,
		},
		{
			name:      "block list, CRLF file",
			body:      "---\r\nname: developer\r\nskills:\r\n  - other-skill\r\n---\r\n\r\nbody\r\n",
			wantBody:  "---\r\nname: developer\r\nskills:\r\n  - other-skill\r\n  - brief-workflow\r\n---\r\n\r\nbody\r\n",
			wantLine:  "  - brief-workflow",
			wantShape: shapeBlockList,
		},
		{
			name:      "flow list, one-line [a]",
			body:      "---\nname: developer\nskills: [other-skill]\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [other-skill, brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [other-skill, brief-workflow]",
			wantShape: shapeFlowList,
		},
		{
			name:      "flow list, empty []",
			body:      "---\nname: developer\nskills: []\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeFlowList,
		},
		{
			name:      "flow list, whitespace only [ ]: treated as empty",
			body:      "---\nname: developer\nskills: [ ]\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeFlowList,
		},
		{
			name:      "flow list, trailing comma: the stray comma is not carried into the result",
			body:      "---\nname: developer\nskills: [other-skill,]\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [other-skill, brief-workflow]\n---\n\nbody\n",
			wantLine:  "skills: [other-skill, brief-workflow]",
			wantShape: shapeFlowList,
		},
		{
			name:          "already listed: block list, membership supplied by the caller",
			body:          "---\nname: developer\nskills:\n  - brief-workflow\n---\n\nbody\n",
			alreadyListed: true,
			wantBody:      "---\nname: developer\nskills:\n  - brief-workflow\n---\n\nbody\n",
			wantLine:      "",
			wantShape:     shapeAlreadyListed,
		},
		{
			name:          "already listed: flow list, membership supplied by the caller",
			body:          "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			alreadyListed: true,
			wantBody:      "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantLine:      "",
			wantShape:     shapeAlreadyListed,
		},
		{
			name:          `already listed: quoted "brief-workflow", membership supplied by the caller`,
			body:          "---\nname: developer\nskills:\n  - \"brief-workflow\"\n---\n\nbody\n",
			alreadyListed: true,
			wantBody:      "---\nname: developer\nskills:\n  - \"brief-workflow\"\n---\n\nbody\n",
			wantLine:      "",
			wantShape:     shapeAlreadyListed,
		},
		{
			name:      "other shape: scalar",
			body:      "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: empty skills:",
			body:      "---\nname: developer\nskills:\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: null",
			body:      "---\nname: developer\nskills: null\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: null\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: multi-line flow",
			body:      "---\nname: developer\nskills: [other,\n  existing]\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [other,\n  existing]\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: flow with a trailing comment",
			body:      "---\nname: developer\nskills: [other-skill] # comment\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills: [other-skill] # comment\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: mapping",
			body:      "---\nname: developer\nskills:\n  brief-workflow: true\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n  brief-workflow: true\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "other shape: non-scalar item",
			body:      "---\nname: developer\nskills:\n  - other-skill\n  - key: val\n---\n\nbody\n",
			wantBody:  "---\nname: developer\nskills:\n  - other-skill\n  - key: val\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "a skills: line after the closing --- is untouched",
			body:      "---\nname: developer\n---\n\nskills: something\n",
			wantBody:  "---\nname: developer\nskills: [brief-workflow]\n---\n\nskills: something\n",
			wantLine:  "skills: [brief-workflow]",
			wantShape: shapeNoKey,
		},
		{
			name:      "opening delimiter with no closing one: unrecognized, never fabricated",
			body:      "---\n",
			wantBody:  "---\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "no frontmatter at all: unrecognized, left untouched",
			body:      "just body text\n",
			wantBody:  "just body text\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
		{
			name:      "no opening delimiter, but a later line matches the closing scan: unrecognized",
			body:      "name: developer\n---\n\nbody\n",
			wantBody:  "name: developer\n---\n\nbody\n",
			wantLine:  "",
			wantShape: shapeOther,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			edited, line, shape := addWorkflowSkill([]byte(c.body), c.alreadyListed)

			assert.Equal(t, c.wantBody, string(edited))
			assert.Equal(t, c.wantLine, line)
			assert.Equal(t, c.wantShape, shape)
		})
	}
}

// Test_remove_workflow_skill_edits_only_the_skills_line pins
// removeWorkflowSkill's full shape matrix, addWorkflowSkill's own inverse
// (Surface & Copy, Rule 8): every body already carries a top-level
// "skills:" key naming "brief-workflow" in some form — membership itself is
// planBoundAgentRemoval's own concern (agentfile.Parse), never this
// function's; what this table pins is only whether, and how, the surgical
// text edit can remove it.
func Test_remove_workflow_skill_edits_only_the_skills_line(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantBody string
		wantOK   bool
	}{
		{
			name:     "single-item flow list: the whole skills: line is dropped",
			body:     "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantBody: "---\nname: developer\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "single-item flow list, CRLF file: the whole skills: line is dropped, CR preserved elsewhere",
			body:     "---\r\nname: developer\r\nskills: [brief-workflow]\r\n---\r\n\r\nbody\r\n",
			wantBody: "---\r\nname: developer\r\n---\r\n\r\nbody\r\n",
			wantOK:   true,
		},
		{
			name:     "flow list, brief-workflow last: [a, brief-workflow] -> [a]",
			body:     "---\nname: developer\nskills: [a, brief-workflow]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [a]\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "flow list, brief-workflow first: [brief-workflow, a] -> [a]",
			body:     "---\nname: developer\nskills: [brief-workflow, a]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [a]\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "flow list, brief-workflow in the middle: [a, brief-workflow, b] -> [a, b]",
			body:     "---\nname: developer\nskills: [a, brief-workflow, b]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [a, b]\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "flow list, trailing comma, only item: the whole skills: line is dropped, not skills: []",
			body:     "---\nname: developer\nskills: [brief-workflow,]\n---\n\nbody\n",
			wantBody: "---\nname: developer\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "flow list, trailing comma, another item remains",
			body:     "---\nname: developer\nskills: [a, brief-workflow,]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [a]\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "block list, item removed, others remain",
			body:     "---\nname: developer\nskills:\n  - a\n  - brief-workflow\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills:\n  - a\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "block list, only item: the skills: key line is dropped too",
			body:     "---\nname: developer\nskills:\n  - brief-workflow\n---\n\nbody\n",
			wantBody: "---\nname: developer\n---\n\nbody\n",
			wantOK:   true,
		},
		{
			name:     "block list, only item, CRLF file: the skills: key line is dropped too",
			body:     "---\r\nname: developer\r\nskills:\r\n  - brief-workflow\r\n---\r\n\r\nbody\r\n",
			wantBody: "---\r\nname: developer\r\n---\r\n\r\nbody\r\n",
			wantOK:   true,
		},
		{
			name:     "a skills: line after the closing --- is untouched",
			body:     "---\nname: developer\nskills: [brief-workflow]\n---\n\nskills: something\n",
			wantBody: "---\nname: developer\n---\n\nskills: something\n",
			wantOK:   true,
		},
		{
			name:     "quoted flow item: unremovable",
			body:     "---\nname: developer\nskills: [\"brief-workflow\"]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [\"brief-workflow\"]\n---\n\nbody\n",
			wantOK:   false,
		},
		{
			name:     "quoted block item: unremovable",
			body:     "---\nname: developer\nskills:\n  - \"brief-workflow\"\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills:\n  - \"brief-workflow\"\n---\n\nbody\n",
			wantOK:   false,
		},
		{
			name:     "flow list with a trailing comment: unremovable",
			body:     "---\nname: developer\nskills: [brief-workflow] # comment\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [brief-workflow] # comment\n---\n\nbody\n",
			wantOK:   false,
		},
		{
			name:     "multi-line flow list: unremovable",
			body:     "---\nname: developer\nskills: [other,\n  brief-workflow]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [other,\n  brief-workflow]\n---\n\nbody\n",
			wantOK:   false,
		},
		{
			name:     "opening delimiter with no closing one: unremovable, never fabricated",
			body:     "---\n",
			wantBody: "---\n",
			wantOK:   false,
		},
		{
			name:     "no frontmatter at all: unremovable",
			body:     "just body text\n",
			wantBody: "just body text\n",
			wantOK:   false,
		},
		{
			name:     "skills: present but no closing delimiter: unremovable, never fabricated",
			body:     "---\nname: developer\nskills: [brief-workflow]\n",
			wantBody: "---\nname: developer\nskills: [brief-workflow]\n",
			wantOK:   false,
		},
		{
			name:     "no opening delimiter, but a later line matches the closing scan: unremovable",
			body:     "name: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantBody: "name: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantOK:   false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			edited, ok := removeWorkflowSkill([]byte(c.body))

			assert.Equal(t, c.wantOK, ok)
			assert.Equal(t, c.wantBody, string(edited))
		})
	}
}

// Test_plan_bound_agent_dangling_symlink and
// Test_plan_bound_agent_removal_dangling_symlink call planBoundAgent and
// planBoundAgentRemoval directly, unexported, since a dangling symlink
// never reaches either through Init's or Uninstall's own real flow:
// agentfile.Find reads a candidate's bytes to decode its "name:" before it
// is ever a target, and a dangling symlink's own os.ReadFile always fails,
// so it is filtered out before boundAgentTargets ever sees it. Both pin
// os.Lstat, not os.Stat, deciding "not a regular file": Stat follows a
// symlink and would error on one with no target, which os.Lstat never
// does.
func Test_plan_bound_agent_dangling_symlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "developer.md")
	require.NoError(t, os.Symlink(filepath.Join(root, "missing-target.md"), path))

	art, ok, err := planBoundAgent(path, root)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ActionKept, art.Action)
	assert.Equal(t, "not a regular file", art.Detail)
}

func Test_plan_bound_agent_removal_dangling_symlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "developer.md")
	require.NoError(t, os.Symlink(filepath.Join(root, "missing-target.md"), path))

	art, ok, err := planBoundAgentRemoval(path, root)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, boundAgentArtifact{}, art)
}

// Test_plan_bound_agent_unparseable_frontmatter_is_never_merged pins
// planBoundAgent's own parseErr != nil branch: a frontmatter that fails to
// decode is kept, with boundAgentUneditableDetail, and never handed to
// addWorkflowSkill at all — body below (an opening delimiter with no
// closing one at all) is also the shape addWorkflowSkill's own openLen/
// found guard rejects as shapeOther (Test_add_workflow_skill_edits_
// only_the_skills_line's own "opening delimiter with no closing one" case),
// so this test pins planBoundAgent's own short-circuit rather than relying
// on that guard alone. agentfile.Find would never hand a file like this to
// boundAgentTargets in the first place, since it filters on decode success,
// so — like the dangling symlink case above — this is reachable only
// through a direct call, standing in for a TOCTOU window between Find's own
// scan and this function's own read.
func Test_plan_bound_agent_unparseable_frontmatter_is_never_merged(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	path := filepath.Join(root, "developer.md")
	body := []byte("---\n")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	art, ok, err := planBoundAgent(path, root)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ActionKept, art.Action)
	assert.Equal(t, boundAgentUneditableDetail, art.Detail)
	assert.Nil(t, art.edited)
}

// Test_write_bound_agent_is_confined_to_resolved_root pins
// confinedAgentFile.write's own root confinement: rel's own leading
// directory is resolved through an os.Root opened at resolvedRoot, planning
// time's own symlink-resolved root, rather than re-resolved from a raw path
// at apply time. Simulating a symlink swapped in between planning and
// applying — the leaf's own parent directory now points outside
// resolvedRoot — must refuse the write rather than silently follow it
// outside. displayPath is deliberately a different string from
// resolvedRoot/rel, so the assertion below can tell "the error names
// displayPath" apart from "the error happens to name the same path either
// way" — this only exercises the OpenRoot(dir) error site, one of
// confinedAgentFile.write's several displayPath call sites.
func Test_write_bound_agent_is_confined_to_resolved_root(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "agents", "developer.md"), []byte("original"), 0o600))

	require.NoError(t, os.RemoveAll(filepath.Join(root, "agents")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "agents")))

	resolvedPath := filepath.Join(root, "agents", "developer.md")
	displayPath := filepath.Join(root, "linked-agents", "developer.md")

	c := confinedAgentFile{resolvedRoot: root, rel: filepath.Join("agents", "developer.md"), displayPath: displayPath}
	err := c.write([]byte("new body"), 0o600)

	require.Error(t, err)
	assert.Contains(t, err.Error(), displayPath)
	assert.NotContains(t, err.Error(), resolvedPath, "the error must name displayPath, not resolvedRoot/rel")

	_, statErr := os.Stat(filepath.Join(outside, "developer.md"))
	assert.True(t, os.IsNotExist(statErr), "the write must never land outside resolvedRoot")
}

// rePointSymlink removes the symlink at path and recreates it pointing at
// target.
func rePointSymlink(t *testing.T, path, target string) {
	t.Helper()

	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Symlink(target, path))
}

// tamperFile overwrites path's own bytes with body, simulating a hand edit
// or a concurrent brief invocation landing between planning and applying.
func tamperFile(t *testing.T, path string, body []byte) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, body, 0o600))
}

// Test_verify_bound_agent_unchanged_reads_through_resolved_root pins
// verifyBoundAgentUnchanged's own re-read target: resolvedRoot/rel, the
// same location the following ba.agentFile().write call is about to
// overwrite — not ba.Path's own symlink, followed fresh. developerPath
// (ba.Path) is
// re-pointed, between planning and this call, at a decoy crafted to hold
// the exact bytes planning read — a check against ba.Path directly would
// wrongly pass, which the control assertion below proves — while
// resolvedRoot/rel's own real target (targetA) is independently tampered
// with in the same window. verifyBoundAgentUnchanged must still refuse: the
// file about to be overwritten did change, regardless of what ba.Path
// currently resolves to.
func Test_verify_bound_agent_unchanged_reads_through_resolved_root(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))

	targetA := filepath.Join(root, "agents", "target-a.md")
	require.NoError(t, os.WriteFile(targetA, []byte("original"), 0o600))

	developerPath := filepath.Join(root, "agents", "developer.md")
	require.NoError(t, os.Symlink(targetA, developerPath))

	ba := boundAgentArtifact{
		Artifact:     Artifact{Path: developerPath},
		existing:     []byte("original"),
		resolvedRoot: root,
		rel:          filepath.Join("agents", "target-a.md"),
	}

	targetB := filepath.Join(root, "agents", "target-b.md")
	tamperFile(t, targetB, []byte("original"))
	rePointSymlink(t, developerPath, targetB)
	tamperFile(t, targetA, []byte("tampered"))

	viaPath, pathErr := os.ReadFile(ba.Path)
	require.NoError(t, pathErr)
	require.Equal(t, ba.existing, viaPath, "control: the decoy must still match planning's own bytes")

	verifyErr := verifyBoundAgentUnchanged(ba, "brief init --edit-agents")

	assert.ErrorIs(t, verifyErr, ErrConcurrentEdit)

	var refusal *RefusalError
	require.ErrorAs(t, verifyErr, &refusal)
	assert.Equal(t, developerPath, refusal.Path, "the reported path is still ba.Path — what the adopter typed")
}

// Test_confined_agent_file_read_refuses_a_directory_symlink_escape pins
// confinedAgentFile.read's own os.Root confinement on the read side — the
// write side is pinned separately by
// Test_write_bound_agent_is_confined_to_resolved_root above. A directory
// component of rel (the "agents" segment) is swapped, between planning and
// this call, for a symlink pointing outside resolvedRoot, at a file holding
// byte-identical content to the original — so a naive
// os.ReadFile(filepath.Join(resolvedRoot, rel)) read would wrongly treat
// the file as unchanged; read must refuse to follow the escape instead.
func Test_confined_agent_file_read_refuses_a_directory_symlink_escape(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	outside, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "agents", "developer.md"), []byte("original"), 0o600))

	c := confinedAgentFile{
		resolvedRoot: root,
		rel:          filepath.Join("agents", "developer.md"),
		displayPath:  filepath.Join(root, "agents", "developer.md"),
	}

	require.NoError(t, os.WriteFile(filepath.Join(outside, "developer.md"), []byte("original"), 0o600))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "agents")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "agents")))

	_, readErr := c.read()

	require.Error(t, readErr)
	assert.False(t, os.IsNotExist(readErr), "a confinement refusal is not a not-exist error")
}

// Test_verify_bound_agent_unchanged_classifies_an_escape_as_concurrent_edit
// pins verifyBoundAgentUnchanged's own classification of a non-NotExist
// read error: when ba.Path no longer resolves to ba.resolvedRoot/ba.rel at
// all — a directory component re-pointed outside resolvedRoot between
// planning and this call, refused by confinedAgentFile.read's own
// confinement — the failure is reported as the same *RefusalError wrapping
// ErrConcurrentEdit a byte mismatch would produce, never a bare wrapped
// read error. displayPath (ba.Path) is deliberately an in-root symlinked
// leaf distinct from resolvedRoot/rel, so refusal.Path can be told apart
// from the location actually read; outside/developer.md — where the escape
// lands — holds byte-identical content to the original, so a read that
// bypassed confinedAgentFile's own os.Root confinement would wrongly treat
// the file as unchanged rather than escaped.
func Test_verify_bound_agent_unchanged_classifies_an_escape_as_concurrent_edit(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	outside, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "agents", "developer.md"), []byte("original"), 0o600))

	displayPath := filepath.Join(root, "developer.md")
	require.NoError(t, os.Symlink(filepath.Join(root, "agents", "developer.md"), displayPath))

	ba := boundAgentArtifact{
		Artifact:     Artifact{Path: displayPath},
		existing:     []byte("original"),
		resolvedRoot: root,
		rel:          filepath.Join("agents", "developer.md"),
	}

	tamperFile(t, filepath.Join(outside, "developer.md"), []byte("original"))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "agents")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "agents")))

	verifyErr := verifyBoundAgentUnchanged(ba, "brief init --edit-agents")

	assert.ErrorIs(t, verifyErr, ErrConcurrentEdit)

	var refusal *RefusalError
	require.ErrorAs(t, verifyErr, &refusal)
	assert.Equal(t, displayPath, refusal.Path)
	assert.NotEqual(t, filepath.Join(root, "agents", "developer.md"), refusal.Path)
}

// Test_verify_bound_agent_unchanged_does_not_misclassify_a_permission_error
// pins boundAgentPathEscaped's own narrower half: a read failure whose
// displayPath still resolves to resolvedRoot/rel — a directory made
// unreadable, rather than re-pointed — must not be reported as
// ErrConcurrentEdit, since rerunning would not fix it.
func Test_verify_bound_agent_unchanged_does_not_misclassify_a_permission_error(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))
	displayPath := filepath.Join(root, "agents", "developer.md")
	require.NoError(t, os.WriteFile(displayPath, []byte("original"), 0o600))

	ba := boundAgentArtifact{
		Artifact:     Artifact{Path: displayPath},
		existing:     []byte("original"),
		resolvedRoot: root,
		rel:          filepath.Join("agents", "developer.md"),
	}

	require.NoError(t, os.Chmod(filepath.Join(root, "agents"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "agents"), 0o755) })

	verifyErr := verifyBoundAgentUnchanged(ba, "brief init --edit-agents")

	require.Error(t, verifyErr)
	assert.NotErrorIs(t, verifyErr, ErrConcurrentEdit)
}
