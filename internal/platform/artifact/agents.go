package artifact

// Every agents/<role>.md file is thin — its body names only the brief
// invocations its own role runs, never a review policy or persona (R7). No
// "model" key is ever written, so a plugin agent always inherits the
// session's own model. The planner and implementer preload
// WorkflowSkillName through frontmatter "skills:" (Rule 2); the reviewer
// does not (R15).

// WorkflowSkillName is the brief-workflow skill's own bare name, the value
// AgentPlanner and AgentImplementer list in frontmatter "skills:" (Rule 2)
// so Claude Code preloads it without either agent naming its file path.
const WorkflowSkillName = "brief-workflow"

// AgentPlanner renders "agents/planner.md": the "planner" role with
// read-write tools and skills: [WorkflowSkillName], planning the feature's
// next scenario with "brief new step <feature>" and never writing code.
func AgentPlanner() []byte {
	return mustReadFile("agents/planner.md")
}

// AgentImplementer renders "agents/implementer.md": the "implementer" role,
// no "tools" key (every tool is inherited), skills: [WorkflowSkillName],
// running "brief start <feature>" and closing with "brief finish".
func AgentImplementer() []byte {
	return mustReadFile("agents/implementer.md")
}

// AgentReviewer renders "agents/reviewer.md": the "reviewer" role with
// read-only tools and no "skills" key — R15 limits the reviewer to
// read-only "brief start" and "brief check", and the brief-workflow skill
// teaches and pre-approves "finish", which the reviewer never runs.
func AgentReviewer() []byte {
	return mustReadFile("agents/reviewer.md")
}
