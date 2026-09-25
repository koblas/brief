package setup

import (
	"fmt"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
)

// MissingSkillReach classifies whether "--edit-agents" could still reach a
// MissingSkillAgent's file, the same verdict planBoundAgent would itself
// reach.
type MissingSkillReach string

const (
	// ReachNone is the zero value, carried only by a ScopeUser row.
	ReachNone MissingSkillReach = ""
	// ReachFixable marks a row "--edit-agents" can still merge the skill into.
	ReachFixable MissingSkillReach = "fixable"
	// ReachNotRegular marks a row whose leaf is not a regular file.
	ReachNotRegular MissingSkillReach = "not-regular"
	// ReachUneditable marks a row whose "skills:" shape or frontmatter can't be edited or parsed.
	ReachUneditable MissingSkillReach = "uneditable"
	// ReachEscaped marks a row whose resolved path lands outside the repository.
	ReachEscaped MissingSkillReach = "escaped"
)

// MissingSkillAgent is one bare-name planner or implementer binding whose
// resolved agent file does not preload the brief-workflow skill. Role and
// Agent name the binding's position and configured value; ScopeRelPath is
// "" for a ScopeProject Definition; Reach is ReachNone for a ScopeUser one.
type MissingSkillAgent struct {
	Role         string
	Agent        string
	Path         string
	Scope        agentfile.Scope
	ScopeRelPath string
	Reach        MissingSkillReach
}

// agentsMissingSkill walks planner then implementer in roles, keeping only
// bare-name bindings, and emits one MissingSkillAgent per Definition
// lacking the workflow skill. Never nil.
func agentsMissingSkill(resolveRoot func(string) (string, error), root, home string, roles config.RoleBindings) ([]MissingSkillAgent, error) {
	resolvedRoot, err := resolveRoot(root)
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
			reach, keep, reachErr := missingSkillReach(d, resolvedRoot)
			if reachErr != nil {
				return nil, fmt.Errorf("setup: check %s for %s: %w", d.Path, artifact.WorkflowSkillName, reachErr)
			}

			if !keep {
				continue
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

// missingSkillReach classifies d via planBoundAgent's verdict. keep is
// false when a fresh read shows the skill already listed (the report is
// stale) or planBoundAgent itself fails.
func missingSkillReach(d agentfile.Definition, resolvedRoot string) (MissingSkillReach, bool, error) {
	if d.Scope != agentfile.ScopeProject {
		return ReachNone, true, nil
	}

	art, ok, err := planBoundAgent(d.Path, resolvedRoot)
	if err != nil {
		return "", false, err
	}

	if !ok {
		return ReachEscaped, true, nil
	}

	switch art.Action {
	case ActionUnchanged:
		return "", false, nil
	case ActionMerged:
		return ReachFixable, true, nil
	case ActionCreated, ActionRemoved, ActionKept:
	}

	if art.Detail == boundAgentNotRegularDetail {
		return ReachNotRegular, true, nil
	}

	return ReachUneditable, true, nil
}

// scopeRelPath renders d's path relative to home, slash-separated, for a
// ScopeUser Definition; "" for a ScopeProject one.
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
