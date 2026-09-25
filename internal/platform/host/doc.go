// Package host adapts brief to one agent host's protocols: reading a
// hook-event payload from that host, rendering brief's findings back into
// whatever shape that host expects on its hook's stdout, listing the files
// that host's skills-directory plugin installs (Plugin), its three
// role-agent files (Agents), the brief-workflow skill file every install
// writes outside the plugin (Skills), and naming the files it reads project
// instructions from (InstructionFiles). It carries no feature knowledge —
// a caller supplies the summary text and the path to look for, never a
// feature name or a *config.Config. Agents is separate from Plugin: Init
// only plans it under --with-agents, but Uninstall always plans their
// removal regardless of flags. Skills is planned on every claude-code
// install, with or without WithAgents, since an agent preloads it by bare
// name rather than through the plugin. None of Plugin, Agents, Skills or
// InstructionFiles performs filesystem access.
package host
