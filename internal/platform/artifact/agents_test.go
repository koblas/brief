package artifact_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// agentFrontmatter is one agents/<role>.md file's own frontmatter, parsed
// by parseAgentFile: name and description are always present. "tools" is
// read separately, from the raw keys map parseAgentFile also returns,
// since an unset key (the implementer, who inherits every tool) must be
// distinguishable from a present-but-empty one.
type agentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseAgentFile splits body into its YAML frontmatter (between "---"
// lines) and the markdown body following it, mirroring parseSkillFile
// (artifact_test.go). The returned keys map reports every key the
// frontmatter carries, so a test can assert an absence ("tools", "model")
// agentFrontmatter's own fixed field set cannot represent.
func parseAgentFile(t *testing.T, body []byte) (agentFrontmatter, map[string]any, string) {
	t.Helper()

	s := string(body)
	require.True(t, strings.HasPrefix(s, "---\n"), "must open with a frontmatter fence")

	after := strings.TrimPrefix(s, "---\n")
	idx := strings.Index(after, "\n---\n")
	require.GreaterOrEqual(t, idx, 0, "must close the frontmatter fence")

	frontmatter := after[:idx]
	rest := after[idx+len("\n---\n"):]

	var fm agentFrontmatter
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &fm))

	var keys map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &keys))

	return fm, keys, rest
}

// Test_agent_frontmatter_names_its_role_with_a_description_and_no_model
// pins the shape every one of the three agent renders shares: "name"
// exactly the role, a non-empty "description", and no "model" key — the
// session's own model is inherited, never pinned.
func Test_agent_frontmatter_names_its_role_with_a_description_and_no_model(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		role string
	}{
		{name: "planner", body: artifact.AgentPlanner(), role: "planner"},
		{name: "implementer", body: artifact.AgentImplementer(), role: "implementer"},
		{name: "reviewer", body: artifact.AgentReviewer(), role: "reviewer"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm, keys, _ := parseAgentFile(t, c.body)

			assert.Equal(t, c.role, fm.Name)
			assert.NotEmpty(t, fm.Description)
			assert.NotContains(t, keys, "model")
		})
	}
}

// Test_planner_and_reviewer_agents_scope_their_own_tools pins R7's
// division of capability between the two roles that do carry a "tools"
// key: the planner may Edit and Write to fill plan files, the reviewer may
// not — "calls into the tool only", never changes a file.
func Test_planner_and_reviewer_agents_scope_their_own_tools(t *testing.T) {
	cases := []struct {
		name  string
		body  []byte
		tools string
	}{
		{name: "planner", body: artifact.AgentPlanner(), tools: "Read, Grep, Glob, Bash, Edit, Write"},
		{name: "reviewer", body: artifact.AgentReviewer(), tools: "Read, Grep, Glob, Bash"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, keys, _ := parseAgentFile(t, c.body)

			assert.Equal(t, c.tools, keys["tools"])
		})
	}
}

// Test_implementer_agent_carries_no_tools_key pins the implementer's own
// position: no "tools" key at all, so it inherits every tool rather than
// being scoped to a subset the way planner and reviewer are.
func Test_implementer_agent_carries_no_tools_key(t *testing.T) {
	_, keys, _ := parseAgentFile(t, artifact.AgentImplementer())

	assert.NotContains(t, keys, "tools")
}

// rulledPlannerBody, rulledImplementerBody are the ruled one-line bodies
// specification.md's "Rewritten plugin agents" block names verbatim for the
// planner and implementer, copied byte-for-byte.
const (
	rulledPlannerBody     = "Plan the feature's next scenario: run `brief new step <feature>`, then fill its plan file. Never write production or test code."
	rulledImplementerBody = "Implement the next open step: `brief start <feature>`, work it, then `brief finish <feature> <step> --handoff <path> --state <path>`."
)

// wantPlannerBytes, wantImplementerBytes are specification.md's "Rewritten
// plugin agents" block, copied byte-for-byte (fenced code block content,
// trailing newline).
var (
	wantPlannerBytes = []byte("---\n" +
		"name: planner\n" +
		"description: Turn a feature's specification into ordered scenario plans.\n" +
		"tools: Read, Grep, Glob, Bash, Edit, Write\n" +
		"skills:\n" +
		"  - brief-workflow\n" +
		"---\n\n" +
		rulledPlannerBody + "\n")

	wantImplementerBytes = []byte("---\n" +
		"name: implementer\n" +
		"description: Implement a feature's next open step, from brief start through brief finish.\n" +
		"skills:\n" +
		"  - brief-workflow\n" +
		"---\n\n" +
		rulledImplementerBody + "\n")
)

// olderPlannerBytes, olderImplementerBytes, currentReviewerBytes are the
// pre-SCENARIO-02 renders, captured mechanically (%q dump of
// artifact.AgentPlanner/AgentImplementer/AgentReviewer) before agents.go was
// touched, and never derived from a live render call — the fixture Recognize
// itself must classify as OriginOlder (planner/implementer) or
// OriginCurrent (reviewer, unchanged).
const (
	olderPlannerBytes = "---\nname: planner\ndescription: Turn a feature's specification into ordered scenario plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's specification into ordered scenario plans: run `brief new step <feature>` for the next scenario, then fill its plan file. Never write production or test code.\n"

	olderImplementerBytes = "---\nname: implementer\ndescription: Implement a feature's next open step, from brief start through brief finish.\n---\n\nRun `brief start <feature>` and implement its next open step, working from its output rather than reading the specification or earlier steps whole. Close the step with `brief finish <feature> <step> --handoff <path> --state <path>`.\n"

	currentReviewerBytes = "---\nname: reviewer\ndescription: Report brief check's findings on a feature's open step, read-only.\ntools: Read, Grep, Glob, Bash\n---\n\nRun `brief start <feature>` to read the open step's acceptance criteria and inherited constraints (it writes nothing), then `brief check <feature>` and report what it finds. Never edit or write a file.\n"
)

// Test_planner_and_implementer_agents_render_the_ruled_bytes pins
// SCENARIO-02's Surface & Copy block byte-for-byte: AgentPlanner and
// AgentImplementer now carry a "skills:" block list naming brief-workflow
// and the ruled one-line body, in the frontmatter order name, description,
// tools (planner only), skills.
func Test_planner_and_implementer_agents_render_the_ruled_bytes(t *testing.T) {
	assert.Equal(t, wantPlannerBytes, artifact.AgentPlanner())
	assert.Equal(t, wantImplementerBytes, artifact.AgentImplementer())
}

// Test_reviewer_agent_bytes_are_unchanged pins R7's exclusion: the reviewer
// never preloads brief-workflow (R15 limits it to read-only brief start and
// brief check), so its render must stay byte-identical to the literal
// captured before agents.go changed.
func Test_reviewer_agent_bytes_are_unchanged(t *testing.T) {
	assert.Equal(t, []byte(currentReviewerBytes), artifact.AgentReviewer())
}

// Test_previous_release_agent_renders_classify_as_older pins Rule 6: the
// pre-SCENARIO-02 planner and implementer bytes move to their own
// older-digest list and Recognize reports OriginOlder for them, never
// OriginEdited; today's own render is still OriginCurrent; the reviewer
// literal — unchanged by this scenario — is still OriginCurrent, never
// OriginOlder, proving the older list holds only the two agents that
// actually changed.
func Test_previous_release_agent_renders_classify_as_older(t *testing.T) {
	assert.Equal(t, artifact.OriginOlder, artifact.Recognize(artifact.KindAgentPlanner, []byte(olderPlannerBytes)))
	assert.Equal(t, artifact.OriginOlder, artifact.Recognize(artifact.KindAgentImplementer, []byte(olderImplementerBytes)))

	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentPlanner, artifact.AgentPlanner()))
	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentImplementer, artifact.AgentImplementer()))

	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentReviewer, []byte(currentReviewerBytes)))
	assert.Equal(t, artifact.OriginEdited, artifact.Recognize(artifact.KindAgentReviewer, []byte(olderPlannerBytes)))
}

// Test_agent_bodies_name_only_their_own_brief_invocations pins R7's "thin,
// limited to calls into the tool": each body names the brief command(s)
// its own role runs — the planner "brief new step", the implementer
// "brief start" then "brief finish", the reviewer "brief start" then
// "brief check" — no review policy or persona content.
func Test_agent_bodies_name_only_their_own_brief_invocations(t *testing.T) {
	cases := []struct {
		name    string
		body    []byte
		invokes string
	}{
		{name: "planner runs new step", body: artifact.AgentPlanner(), invokes: "brief new step"},
		{name: "implementer runs start", body: artifact.AgentImplementer(), invokes: "brief start"},
		{name: "implementer runs finish", body: artifact.AgentImplementer(), invokes: "brief finish"},
		{name: "reviewer runs start", body: artifact.AgentReviewer(), invokes: "brief start"},
		{name: "reviewer runs check", body: artifact.AgentReviewer(), invokes: "brief check"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, body := parseAgentFile(t, c.body)

			assert.Contains(t, body, c.invokes)
		})
	}
}
