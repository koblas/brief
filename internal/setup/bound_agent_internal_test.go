package setup

// White-box package: addWorkflowSkill is unexported. These tests pin its
// own shape table (Surface & Copy) directly, against hand-built frontmatter
// bytes, independent of planBoundAgents' own file-finding and membership
// decisions (bound_agent_test.go, black-box).

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
