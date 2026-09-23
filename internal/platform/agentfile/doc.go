// Package agentfile resolves an agent by name the same way Claude Code
// identifies one: Find walks a ".claude/agents/" tree recursively and
// matches on frontmatter "name:", never on filename or directory layout.
// It only locates and decodes; it never writes or edits an agent file —
// that is a caller's own concern (doctor reports what it finds, a future
// setup edit rewrites a file's own frontmatter in place).
//
// Definition.Scope's two values, ScopeProject and ScopeUser, are the
// literal strings a JSON report renders as "scope": "project" or "scope":
// "user" — callers should not re-derive or duplicate them.
package agentfile
