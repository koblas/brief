// Package agentfile resolves an agent by name the same way Claude Code
// identifies one: FindIn walks a ".claude/agents/" tree recursively and
// matches on frontmatter "name:", never on filename or directory layout.
// The filesystem is a port: a Tree pairs an fs.FS with the absolute OS
// directory it is rooted at; DirTree builds one over os.DirFS, and Find is
// FindIn over the project and home DirTrees. Load, LoadFS and Parse decode
// an agent file's frontmatter from disk, an fs.FS, or bytes already in
// memory. ResolveBindingIn classifies a role binding's configured value
// ("brief:<name>", another plugin's "<plugin>:<name>", or a bare name)
// into a Binding; (Binding).LackingSkill reports whether that binding's
// agent preloads a given skill. Find, Load, Parse and ResolveBinding only
// locate, decode and classify; none writes or edits an agent file.
//
// Frontmatter.Skills and Frontmatter.OmitClaudeMd decode loosely: an
// unexpected shape yields nil or false rather than failing the whole
// decode, so a malformed value never drops an otherwise well-formed agent
// out of Find's name match (Rule 5).
package agentfile
