// Package host adapts brief to one agent host's own protocols: reading a
// hook-event payload from that host, rendering brief's own findings back
// into whatever shape that host expects on its hook's stdout, listing the
// files that host's skills-directory plugin installs (Plugin) and its
// three role-agent files (Agents, R7), and naming the files it reads
// project instructions from (InstructionFiles, R5's CLAUDE.md candidates).
// It carries no feature knowledge — a caller supplies the summary text and
// the path to look for, never a feature name or a *config.Config; a
// plugin File names only a relative path and an
// internal/platform/artifact.Kind, never a body or an install root, both of
// which the caller supplies. Agents is separate from Plugin: Init only
// plans it under --with-agents, but Uninstall always plans their removal,
// independent of any flag Init was run with. InstructionFiles performs no
// filesystem access of its own, the same way Plugin's and Agents' own File
// lists do not.
package host
