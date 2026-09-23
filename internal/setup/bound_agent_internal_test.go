package setup

// White-box package: addWorkflowSkill is unexported. These tests pin its
// own shape table (Surface & Copy) directly, against hand-built frontmatter
// bytes, independent of planBoundAgents' own file-finding and membership
// decisions (bound_agent_test.go, black-box).

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

// Test_write_bound_agent_is_confined_to_resolved_root pins
// writeBoundAgent's own root confinement: rel's own leading directory is
// resolved through an os.Root opened at resolvedRoot, planning time's own
// symlink-resolved root, rather than re-resolved from a raw path at apply
// time. Simulating a symlink swapped in between planning and applying —
// the leaf's own parent directory now points outside resolvedRoot — must
// refuse the write rather than silently follow it outside.
func Test_write_bound_agent_is_confined_to_resolved_root(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "agents", "developer.md"), []byte("original"), 0o600))

	require.NoError(t, os.RemoveAll(filepath.Join(root, "agents")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "agents")))

	err := writeBoundAgent(root, filepath.Join("agents", "developer.md"), []byte("new body"), 0o600)
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(outside, "developer.md"))
	assert.True(t, os.IsNotExist(statErr), "the write must never land outside resolvedRoot")
}
