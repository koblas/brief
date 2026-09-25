package artifact

// Every agents/<role>.md file is thin — its body names only the brief
// invocations its own role runs, never a review policy or persona. No
// "model" key is ever written, so a plugin agent always inherits the
// session's model. The planner and implementer preload WorkflowSkillName
// through frontmatter "skills:"; the reviewer does not.

// WorkflowSkillName is the brief-workflow skill's bare name, the value
// AgentPlanner and AgentImplementer list in frontmatter "skills:"
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
// read-only tools and no "skills" key — limited to read-only "brief start"
// and "brief check"; the reviewer never runs "finish".
func AgentReviewer() []byte {
	return mustReadFile("agents/reviewer.md")
}
