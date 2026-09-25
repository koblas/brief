package artifact_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// agentFrontmatter is one agents/<role>.md file's frontmatter: name and
// description. "tools" is read separately from parseAgentFile's raw keys
// map, since an unset key must be distinguishable from an empty one.
type agentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseAgentFile splits body into its YAML frontmatter and markdown body.
// The returned keys map reports every frontmatter key, so a test can
// assert an absence agentFrontmatter's fixed field set cannot represent.
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

func Test_implementer_agent_carries_no_tools_key(t *testing.T) {
	_, keys, _ := parseAgentFile(t, artifact.AgentImplementer())

	assert.NotContains(t, keys, "tools")
}

// rulledPlannerBody, rulledImplementerBody are the ruled one-line agent bodies.
const (
	rulledPlannerBody     = "Plan the feature's next scenario: run `brief new step <feature>`, then fill its plan file. Never write production or test code."
	rulledImplementerBody = "Implement the next open step: `brief start <feature>`, work it, then `brief finish <feature> <step> --handoff <path> --state <path>`."
)

// wantPlannerBytes, wantImplementerBytes are the ruled planner and
// implementer renders, copied byte-for-byte from the source of truth.
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

// olderPlannerBytes, olderImplementerBytes, currentReviewerBytes are
// earlier-release renders, captured mechanically and never derived from a
// live render call.
const (
	olderPlannerBytes = "---\nname: planner\ndescription: Turn a feature's specification into ordered scenario plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's specification into ordered scenario plans: run `brief new step <feature>` for the next scenario, then fill its plan file. Never write production or test code.\n"

	olderImplementerBytes = "---\nname: implementer\ndescription: Implement a feature's next open step, from brief start through brief finish.\n---\n\nRun `brief start <feature>` and implement its next open step, working from its output rather than reading the specification or earlier steps whole. Close the step with `brief finish <feature> <step> --handoff <path> --state <path>`.\n"

	currentReviewerBytes = "---\nname: reviewer\ndescription: Report brief check's findings on a feature's open step, read-only.\ntools: Read, Grep, Glob, Bash\n---\n\nRun `brief start <feature>` to read the open step's acceptance criteria and inherited constraints (it writes nothing), then `brief check <feature>` and report what it finds. Never edit or write a file.\n"
)

func Test_planner_and_implementer_agents_render_the_ruled_bytes(t *testing.T) {
	assert.Equal(t, wantPlannerBytes, artifact.AgentPlanner())
	assert.Equal(t, wantImplementerBytes, artifact.AgentImplementer())
}

func Test_reviewer_agent_bytes_are_unchanged(t *testing.T) {
	assert.Equal(t, []byte(currentReviewerBytes), artifact.AgentReviewer())
}

func Test_previous_release_agent_renders_classify_as_older(t *testing.T) {
	assert.Equal(t, artifact.OriginOlder, artifact.Recognize(artifact.KindAgentPlanner, []byte(olderPlannerBytes)))
	assert.Equal(t, artifact.OriginOlder, artifact.Recognize(artifact.KindAgentImplementer, []byte(olderImplementerBytes)))

	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentPlanner, artifact.AgentPlanner()))
	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentImplementer, artifact.AgentImplementer()))

	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindAgentReviewer, []byte(currentReviewerBytes)))
	assert.Equal(t, artifact.OriginEdited, artifact.Recognize(artifact.KindAgentReviewer, []byte(olderPlannerBytes)))
}

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
