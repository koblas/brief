// Package agentfile resolves an agent by name the same way Claude Code
// identifies one: FindIn walks a ".claude/agents/" tree recursively and
// matches on frontmatter "name:", never on filename or directory layout.
// The filesystem is a port: a Tree pairs an fs.FS with the absolute OS
// directory it is rooted at, so every Definition.Path is a real path a
// caller can Lstat or rewrite; DirTree builds one over os.DirFS, and Find
// is FindIn over the project and home DirTrees. Load decodes one
// already-resolved file's own frontmatter directly from disk, LoadFS is
// its fs.FS-backed twin, and Parse decodes it from bytes already held in
// memory. ResolveBindingIn classifies a role binding's own configured
// value — "brief:<name>", another plugin's "<plugin>:<name>", or a bare
// name — against a project and a user Tree into a Binding; ResolveBinding
// is ResolveBindingIn over DirTree(root) and DirTree(home), the OS
// adapter, consistent with Find over FindIn. (Binding).LackingSkill is the
// single decision point behind "does this binding's own agent preload a
// given skill", shared by doctor's roles/roles-skill rows and setup's own
// missing-skill report: for a "brief:*" binding it reads the resolved
// file through the same Tree ResolveBindingIn found it in (LoadFS) when
// available, falling back to Load(Path) for a Binding assembled any other
// way. Find, Load, Parse and ResolveBinding only locate, decode and
// classify; none writes or edits an agent file — that is a caller's own
// concern: doctor reports what it finds, and setup's own
// "brief init --edit-agents" (internal/setup's bound_agent.go) edits a
// resolved project agent file's own "skills:" frontmatter line in place,
// deciding membership through Parse over the same bytes it edits.
//
// A Definition's own Frontmatter.Skills and Frontmatter.OmitClaudeMd
// decode loosely: a "skills:" value that is not a plain sequence of
// scalars yields a nil Skills, and an "omitClaudeMd:" value that is not a
// bare bool yields a false OmitClaudeMd, rather than failing the whole
// decode — a malformed value in either key must never drop an otherwise
// well-formed agent out of Find's own name match (Rule 5).
//
// Definition.Scope's two values, ScopeProject and ScopeUser, are the
// literal strings a JSON report renders as "scope": "project" or "scope":
// "user" — callers should not re-derive or duplicate them.
package agentfile
