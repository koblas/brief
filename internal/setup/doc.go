// Package setup owns brief's install write path: Init plans and then
// applies the ".brief.yaml" config file, the feature root, and, for
// HostClaudeCode, that host's plugin files, the brief-workflow skill,
// under WithAgents its three role-agent files, under EditAgents a
// "skills:" edit to every bare-name planner or implementer agent file, and
// the CLAUDE.md instruction block. Uninstall plans and applies removing
// everything Init installed, agent and skill files included regardless of
// any flag Init was run with. Both plan every artifact and decide every
// refusal before the first byte is written or removed, so a DryRun
// request computes exactly what a real run would, and a refusal never
// leaves a partial tree behind on its own account; a write failure after
// an earlier write landed is instead wrapped in ErrPartialWrite.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong, and
// how to fix it. Init writes the config file last, so it never appears
// before everything else has landed; Uninstall removes it last for the
// same reason, in reverse of Init's install order. The feature root and
// everything under it is never an Uninstall artifact, and setup never
// removes anything above host.PluginDir, host.WorkflowSkillDir, or the
// CLAUDE.md candidates host.InstructionFiles names.
//
// setup writes through the real filesystem via internal/platform/
// atomicfile, os.MkdirAll and os.Remove, never RemoveAll, and imports only
// internal/platform/agentfile, config, artifact, atomicfile, host, repo,
// rwfs and writable alongside the standard library — never
// internal/scaffold or internal/doctor, which own a feature's own content,
// a question setup never asks. Every read and write under a repository
// root goes through (*Server).fsRoot, production diskFS (fs.go); a
// bound-agent file (bound_agent.go's confinedAgentFile) always reads and
// writes through real disk instead, regardless of fsRoot.
package setup
