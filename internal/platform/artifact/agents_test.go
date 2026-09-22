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
