// Package agentfile resolves an agent by name the same way Claude Code
// identifies one: Find walks a ".claude/agents/" tree recursively and
// matches on frontmatter "name:", never on filename or directory layout;
// Load decodes one already-resolved file's own frontmatter directly. Both
// only locate and decode; neither writes or edits an agent file — that is
// a caller's own concern (doctor reports what it finds, a future setup
// edit rewrites a file's own frontmatter in place).
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
