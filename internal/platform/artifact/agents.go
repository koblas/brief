package artifact

import (
	"fmt"
	"strings"
)

// Every agents/<role>.md file this package renders is thin — its body
// names only the brief invocations its own role runs, never a review
// policy or persona (R7). renderAgent builds one such file's exact bytes:
// a YAML frontmatter block between "---" lines carrying name, description
// and, when tools is non-empty, a comma-separated "tools" list, then the
// body prose as given. No "model" key is ever written, so a plugin agent
// always inherits the session's own model.

// agentPlannerDescription is AgentPlanner's own frontmatter "description".
const agentPlannerDescription = "Turn a feature's specification into ordered scenario plans."

// agentImplementerDescription is AgentImplementer's own frontmatter
// "description".
const agentImplementerDescription = "Implement a feature's next open step, from brief start through brief finish."

// agentReviewerDescription is AgentReviewer's own frontmatter
// "description".
const agentReviewerDescription = "Report brief check's findings on a feature's open step, read-only."

// agentPlannerTools is AgentPlanner's own frontmatter "tools": the planner
// fills plan files, so it needs Edit and Write alongside the read-only
// set.
const agentPlannerTools = "Read, Grep, Glob, Bash, Edit, Write"

// agentReviewerTools is AgentReviewer's own frontmatter "tools": read-only
// — no Edit or Write — since the reviewer only ever calls into the tool
// and reports, never changes a file.
const agentReviewerTools = "Read, Grep, Glob, Bash"

// AgentPlanner renders "agents/planner.md": frontmatter naming the
// "planner" role with agentPlannerTools, and a body that turns a
// specification into steps with "brief new step <feature>", filling each
// plan, and never writes code.
func AgentPlanner() []byte {
	return renderAgent("planner", agentPlannerDescription, agentPlannerTools,
		"Turn the feature's specification into ordered scenario plans: run "+
			"`brief new step <feature>` for the next scenario, then fill its "+
			"plan file. Never write production or test code.")
}

// AgentImplementer renders "agents/implementer.md": frontmatter naming the
// "implementer" role, no "tools" key (every tool is inherited), and a body
// that runs "brief start <feature>", works from its output, and closes
// with "brief finish <feature> <step> --handoff <path> --state <path>".
func AgentImplementer() []byte {
	return renderAgent("implementer", agentImplementerDescription, "",
		"Run `brief start <feature>` and implement its next open step, "+
			"working from its output rather than reading the specification "+
			"or earlier steps whole. Close the step with `brief finish <feature> "+
			"<step> --handoff <path> --state <path>`.")
}

// AgentReviewer renders "agents/reviewer.md": frontmatter naming the
// "reviewer" role with agentReviewerTools (no Edit or Write), and a body
// that runs "brief start <feature>" read-only for the open step's own
// acceptance criteria and inherited constraints, then "brief check
// <feature>" and reports its findings — no review policy or persona
// content of its own.
func AgentReviewer() []byte {
	return renderAgent("reviewer", agentReviewerDescription, agentReviewerTools,
		"Run `brief start <feature>` to read the open step's acceptance "+
			"criteria and inherited constraints (it writes nothing), then "+
			"`brief check <feature>` and report what it finds. Never edit or "+
			"write a file.")
}

// renderAgent builds one agents/<role>.md file's exact bytes: a YAML
// frontmatter block carrying name and description, a "tools" line only
// when tools is non-empty, then body as the file's own markdown content.
func renderAgent(role, description, tools, body string) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", role)
	fmt.Fprintf(&b, "description: %s\n", description)

	if tools != "" {
		fmt.Fprintf(&b, "tools: %s\n", tools)
	}

	b.WriteString("---\n\n")
	b.WriteString(body)
	b.WriteByte('\n')

	return []byte(b.String())
}
