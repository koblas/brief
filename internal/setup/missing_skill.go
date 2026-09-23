package setup

import (
	"fmt"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
)

// MissingSkillReach classifies whether "--edit-agents" could still reach a
// MissingSkillAgent's own file — setup's own planBoundAgent verdict,
// computed once here so a caller (cli's own missing-skill report) need only
// map it to display text rather than re-derive it: ReachFixable when
// planBoundAgent would still merge the skill in; ReachNotRegular and
// ReachUneditable each name a reason planBoundAgent itself would leave a
// row it read ActionKept — a non-regular leaf, or a "skills:" shape (or
// unparseable frontmatter) it cannot edit; ReachEscaped when the resolved
// path lands outside the repository, the one case planBoundAgent
// contributes no row for at all. ReachNone is the zero value, carried only
// by a ScopeUser row — "--edit-agents" never targets user-scope agents in
// the first place (Rule 3), so none of the other four ever apply to one.
type MissingSkillReach string

const (
	// ReachNone is the zero value, carried only by a ScopeUser row —
	// "--edit-agents" never targets user-scope agents (Rule 3), so no
	// other MissingSkillReach value ever applies to one.
	ReachNone MissingSkillReach = ""
	// ReachFixable marks a ScopeProject row "--edit-agents" can still
	// merge the skill into.
	ReachFixable MissingSkillReach = "fixable"
	// ReachNotRegular marks a ScopeProject row whose own leaf is not a
	// regular file (a symlink whose own target still resolves inside the
	// repository).
	ReachNotRegular MissingSkillReach = "not-regular"
	// ReachUneditable marks a ScopeProject row whose "skills:" frontmatter
	// shape planBoundAgent cannot edit, or whose frontmatter does not
	// parse at all.
	ReachUneditable MissingSkillReach = "uneditable"
	// ReachEscaped marks a ScopeProject row whose own resolved path lands
	// outside the repository (a ".claude" symlinked elsewhere).
	ReachEscaped MissingSkillReach = "escaped"
)

// MissingSkillAgent is one bare-name planner or implementer binding whose
// resolved agent file does not preload the brief-workflow skill —
// Result.AgentsMissingSkill's own element: Role and Agent name the
// binding's own position and configured value, Path is the resolved
// file's own absolute path, Scope names which root it came from
// (agentfile.Scope), ScopeRelPath carries a ScopeUser Definition's own
// path relative to home, slash-separated ("" for a ScopeProject
// Definition — the project row renders relative to wd instead, a caller's
// own concern), and Reach classifies a ScopeProject Definition's own
// setup.planBoundAgent verdict (MissingSkillReach) — ReachNone for a
// ScopeUser Definition, which "--edit-agents" never targets.
type MissingSkillAgent struct {
	Role         string
	Agent        string
	Path         string
	Scope        agentfile.Scope
	ScopeRelPath string
	Reach        MissingSkillReach
}

// agentsMissingSkill walks planner then implementer in roles, keeping only
// bindings agentfile.ResolveBinding classifies BindingBare (never a
// "brief:*" or other-plugin binding — Rule 3, Rule 4's own target set),
// and emits one MissingSkillAgent per Definition its own
// LackingSkill(artifact.WorkflowSkillName) returns, in that same order.
// root's own symlink resolution failing is returned as an error, the same
// treatment boundAgentTargets gives it — nothing meaningful can be
// classified against an unresolvable root. The result is never nil.
func agentsMissingSkill(root, home string, roles config.RoleBindings) ([]MissingSkillAgent, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("setup: resolve %s: %w", root, err)
	}

	out := []MissingSkillAgent{}

	for _, r := range []struct{ role, value string }{
		{"planner", roles.Planner},
		{"implementer", roles.Implementer},
	} {
		b := agentfile.ResolveBinding(root, home, r.value)
		if b.Kind != agentfile.BindingBare {
			continue
		}

		for _, d := range b.LackingSkill(artifact.WorkflowSkillName) {
			reach, reachErr := missingSkillReach(d, resolvedRoot)
			if reachErr != nil {
				return nil, reachErr
			}

			out = append(out, MissingSkillAgent{
				Role:         r.role,
				Agent:        r.value,
				Path:         d.Path,
				Scope:        d.Scope,
				ScopeRelPath: scopeRelPath(home, d),
				Reach:        reach,
			})
		}
	}

	return out, nil
}

// missingSkillReach classifies d — one MissingSkillAgent's own source
// Definition — via planBoundAgent, the same verdict "--edit-agents" would
// itself reach, called here in the same plan-only shape planBoundAgents
// itself calls it in (no write ever happens from this path): ReachNone for
// a ScopeUser Definition, which planBoundAgent never targets; ReachEscaped
// when planBoundAgent contributes no row at all (ok false — the resolved
// path escapes resolvedRoot); ReachFixable when it returns a row with
// Action other than ActionKept — the same "still reachable" condition
// planBoundAgents' own apply step relies on; otherwise ReachNotRegular or
// ReachUneditable, read off the row's own Detail. A planBoundAgent error
// propagates unchanged — d.Path having already been read once to build the
// report in the first place, a fresh Lstat/EvalSymlinks/ReadFile failure
// here means it changed underneath this run.
func missingSkillReach(d agentfile.Definition, resolvedRoot string) (MissingSkillReach, error) {
	if d.Scope != agentfile.ScopeProject {
		return ReachNone, nil
	}

	art, ok, err := planBoundAgent(d.Path, resolvedRoot)
	if err != nil {
		return "", err
	}

	if !ok {
		return ReachEscaped, nil
	}

	if art.Action != ActionKept {
		return ReachFixable, nil
	}

	if art.Detail == boundAgentNotRegularDetail {
		return ReachNotRegular, nil
	}

	return ReachUneditable, nil
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
