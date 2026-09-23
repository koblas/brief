package setup

import (
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
)

// MissingSkillAgent is one bare-name planner or implementer binding whose
// resolved agent file does not preload the brief-workflow skill —
// Result.AgentsMissingSkill's own element: Role and Agent name the
// binding's own position and configured value, Path is the resolved
// file's own absolute path, Scope names which root it came from
// (agentfile.Scope), ScopeRelPath carries a ScopeUser Definition's own
// path relative to home, slash-separated ("" for a ScopeProject
// Definition — the project row renders relative to wd instead, a caller's
// own concern), and Escaped is true for a ScopeProject Definition whose
// own resolved path, symlinks followed, lands outside root (a ".claude"
// symlinked elsewhere) — always false for ScopeUser, which is already
// outside the repository by definition and carries its own annotation.
type MissingSkillAgent struct {
	Role         string
	Agent        string
	Path         string
	Scope        agentfile.Scope
	ScopeRelPath string
	Escaped      bool
}

// agentsMissingSkill walks planner then implementer in roles, keeping only
// bindings agentfile.ResolveBinding classifies BindingBare (never a
// "brief:*" or other-plugin binding — Rule 3, Rule 4's own target set),
// and emits one MissingSkillAgent per Definition its own
// LackingSkill(artifact.WorkflowSkillName) returns, in that same order.
// The result is never nil.
func agentsMissingSkill(root, home string, roles config.RoleBindings) []MissingSkillAgent {
	out := []MissingSkillAgent{}
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)

	for _, r := range []struct{ role, value string }{
		{"planner", roles.Planner},
		{"implementer", roles.Implementer},
	} {
		b := agentfile.ResolveBinding(root, home, r.value)
		if b.Kind != agentfile.BindingBare {
			continue
		}

		for _, d := range b.LackingSkill(artifact.WorkflowSkillName) {
			out = append(out, MissingSkillAgent{
				Role:         r.role,
				Agent:        r.value,
				Path:         d.Path,
				Scope:        d.Scope,
				ScopeRelPath: scopeRelPath(home, d),
				Escaped:      d.Scope == agentfile.ScopeProject && pathEscapesRoot(resolvedRoot, rootErr, d.Path),
			})
		}
	}

	return out
}

// pathEscapesRoot reports whether path, symlinks resolved, lands outside
// resolvedRoot — the same escape check planBoundAgent's own resolvedRoot
// pair applies, mirrored here for report-only use: rootErr non-nil (root
// itself unresolvable) or path's own resolution failing is never treated
// as an escape, since neither proves anything about path's relation to
// root.
func pathEscapesRoot(resolvedRoot string, rootErr error, path string) bool {
	if rootErr != nil {
		return false
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(resolvedRoot, resolvedPath)

	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// scopeRelPath renders d's own path relative to home, slash-separated, for
// a ScopeUser Definition — "" for a ScopeProject one, or when the relative
// path cannot be computed (d.Path itself, unlikely once Find has already
// resolved it under home).
func scopeRelPath(home string, d agentfile.Definition) string {
	if d.Scope != agentfile.ScopeUser {
		return ""
	}

	rel, err := filepath.Rel(home, d.Path)
	if err != nil {
		return d.Path
	}

	return filepath.ToSlash(rel)
}
