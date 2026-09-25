package setup

// These tests call planBoundAgent, planBoundAgentRemoval, confinedAgentFile
// and the addWorkflowSkill/removeWorkflowSkill helpers directly: they cover
// real symlinks and TOCTOU windows that srv.Init/srv.Uninstall's own flow,
// and an rwfs.Mem, cannot reach or reproduce.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// alreadyListed is caller-supplied, not decided by the transform; only the
// "already listed" cases below pass it true.
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

// A dangling symlink never reaches planBoundAgent/planBoundAgentRemoval
// through Init's or Uninstall's real flow, so both call them directly.
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

// agentfile.Find would never hand a file like this to boundAgentTargets,
// since it filters on decode success; this calls planBoundAgent directly.
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

// displayPath is deliberately different from resolvedRoot/rel, so the
// assertion can tell which one the error names.
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

// developerPath (ba.Path) is re-pointed at a decoy holding planning's own
// bytes (the control read below proves this); resolvedRoot/rel's real
// target is tampered independently in the same window.
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

// The escape target holds byte-identical content to the original, so a
// naive read would wrongly treat the file as unchanged; read must refuse.
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

// displayPath (ba.Path) is an in-root symlink distinct from resolvedRoot/
// rel, so refusal.Path can be told apart from the location actually read.
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

// The agents directory is made unreadable, not re-pointed, so displayPath
// still resolves to resolvedRoot/rel.
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
