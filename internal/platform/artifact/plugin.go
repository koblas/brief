package artifact

// PluginManifest renders ".claude-plugin/plugin.json": {"name": "brief"},
// two-space indented, trailing newline. It carries no "version" key —
// code.claude.com/docs/en/plugins-reference confirms version is optional
// for a skills-directory plugin — so its bytes never change across a
// release unless the plugin's own name does.
func PluginManifest() []byte {
	return mustReadFile("plugin.json")
}

// SkillStart renders "skills/start/SKILL.md": an explicit slash command
// (disable-model-invocation: true) whose allowed-tools is scoped to
// "Bash(brief start *)", running "brief start $ARGUMENTS" and working from
// its output.
func SkillStart() []byte {
	return mustReadFile("skills/start/SKILL.md")
}

// SkillFinish renders "skills/finish/SKILL.md": an explicit slash command
// (disable-model-invocation: true) whose allowed-tools is scoped to
// "Bash(brief finish *)", running "brief finish $ARGUMENTS" and working
// from its output.
func SkillFinish() []byte {
	return mustReadFile("skills/finish/SKILL.md")
}

// SkillWorkflow renders ".claude/skills/brief-workflow/SKILL.md": the
// step-protocol skill any agent — brief's own or one the repository already
// has — preloads via its frontmatter "skills:" to learn brief start /
// tick-by-hand / brief finish / brief new step. Unlike SkillStart and
// SkillFinish, it is not user-invocable and carries no argument-hint: it is
// never run as a slash command, only preloaded.
func SkillWorkflow() []byte {
	return mustReadFile("skills/brief-workflow/SKILL.md")
}

// ClaudeHooks renders "hooks/hooks.json": one PostToolUse entry matching
// "Edit|Write|MultiEdit" that runs "brief check --hook claude-code" as a
// "command" hook (R12).
func ClaudeHooks() []byte {
	return mustReadFile("hooks.json")
}

// Render returns kind's own current bytes — written bytes and recognized
// bytes are always one value, since digestsFor's own compiled-in digests
// are computed from these same render functions — or nil for a Kind this
// package does not render.
func Render(kind Kind) []byte {
	switch kind {
	case KindConfig:
		return ConfigFile()
	case KindPluginManifest:
		return PluginManifest()
	case KindSkillStart:
		return SkillStart()
	case KindSkillFinish:
		return SkillFinish()
	case KindClaudeHooks:
		return ClaudeHooks()
	case KindAgentPlanner:
		return AgentPlanner()
	case KindAgentImplementer:
		return AgentImplementer()
	case KindAgentReviewer:
		return AgentReviewer()
	case KindSkillWorkflow:
		return SkillWorkflow()
	case KindSnippet:
		// Deliberately excluded: SnippetBlock is the snippet's own render
		// function, taking the feature directory Render's signature has no
		// room for (see doc.go).
		return nil
	default:
		return nil
	}
}
