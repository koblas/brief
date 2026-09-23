package agentfile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/koblas/brief/internal/platform/host"
)

// BindingKind classifies a role binding's own configured value shape,
// independent of whether it resolved to a file: BindingBare for a plain
// name, BindingBrief for a "brief:<name>" value, BindingPlugin for any
// other "<plugin>:<name>" value.
type BindingKind int

const (
	// BindingBare marks a binding with no "<plugin>:" prefix.
	BindingBare BindingKind = iota
	// BindingBrief marks a "brief:<name>" binding.
	BindingBrief
	// BindingPlugin marks a "<plugin>:<name>" binding for a plugin other
	// than "brief".
	BindingPlugin
)

// BindingState classifies a Binding's own resolution against the
// filesystem.
type BindingState int

const (
	// BindingUnbound marks an empty binding value.
	BindingUnbound BindingState = iota
	// BindingResolved marks a binding whose own agent file was found.
	BindingResolved
	// BindingUnresolved marks a non-empty binding whose own agent file was
	// not found.
	BindingUnresolved
	// BindingUnverified marks a BindingPlugin binding: there is no file
	// layout to check it against, so it counts as bound without being
	// verified.
	BindingUnverified
)

// Binding is ResolveBinding's own result: Kind classifies the configured
// value's own shape, State classifies its resolution, Path carries the
// resolved file's own absolute path when State is BindingResolved (""
// otherwise), and Defs carries every Definition a BindingBare binding's own
// Find returned — nil for a BindingBrief or BindingPlugin binding, or an
// unbound or unresolved one.
type Binding struct {
	Kind  BindingKind
	State BindingState
	Path  string
	Defs  []Definition
}

// ResolveBinding classifies value (a config.RoleBindings field) against
// root and, for a bare name, home (Rule 5): "" is BindingUnbound. A
// "brief:<name>" binding is BindingResolved via
// "<root>/.claude/agents/<name>.md", overriding
// "<root>/.claude/skills/brief/agents/<name>.md" when both are regular
// files, BindingUnresolved when neither is. Any other "<plugin>:<name>" is
// BindingUnverified. A bare "<name>" is BindingResolved when
// Find(root, home, name) returns at least one Definition, BindingUnresolved
// otherwise — Path and Defs then carry its first and every result
// respectively, in the scope Find chose (project agents shadow a
// same-named user one entirely, never mixed).
func ResolveBinding(root, home, value string) Binding {
	if value == "" {
		return Binding{State: BindingUnbound}
	}

	if plugin, agent, ok := strings.Cut(value, ":"); ok {
		if plugin != "brief" {
			return Binding{Kind: BindingPlugin, State: BindingUnverified}
		}

		overridePath := filepath.Join(root, ".claude", "agents", agent+".md")
		pluginPath := filepath.Join(root, host.PluginDir, "agents", agent+".md")

		switch {
		case fileIsRegular(overridePath):
			return Binding{Kind: BindingBrief, State: BindingResolved, Path: overridePath}
		case fileIsRegular(pluginPath):
			return Binding{Kind: BindingBrief, State: BindingResolved, Path: pluginPath}
		default:
			return Binding{Kind: BindingBrief, State: BindingUnresolved}
		}
	}

	defs := Find(root, home, value)
	if len(defs) == 0 {
		return Binding{Kind: BindingBare, State: BindingUnresolved}
	}

	return Binding{Kind: BindingBare, State: BindingResolved, Path: defs[0].Path, Defs: defs}
}

// fileIsRegular reports whether path exists and is a regular file,
// following symlinks.
func fileIsRegular(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// LackingSkill returns every Definition behind b whose own frontmatter
// "skills:" does not name skill — nil when b is not BindingResolved, or
// every resolved Definition already names it. For a BindingBare binding
// this filters b.Defs directly, in Find's own order. For a BindingBrief
// binding, which carries no Defs, it synthesizes one Definition via Load:
// a load failure counts as lacking, its Definition carrying a zero
// Frontmatter (the file's own omitClaudeMd is then unknown), Scope always
// ScopeProject — a "brief:*" binding never resolves outside the
// repository. A BindingPlugin binding is never BindingResolved, so it
// always returns nil here without a special case.
func (b Binding) LackingSkill(skill string) []Definition {
	if b.State != BindingResolved {
		return nil
	}

	if b.Kind == BindingBare {
		var out []Definition

		for _, d := range b.Defs {
			if !hasSkill(d.Frontmatter.Skills, skill) {
				out = append(out, d)
			}
		}

		return out
	}

	fm, err := Load(b.Path)
	if err != nil {
		return []Definition{{Path: b.Path, Scope: ScopeProject}}
	}

	if hasSkill(fm.Skills, skill) {
		return nil
	}

	return []Definition{{Path: b.Path, Scope: ScopeProject, Frontmatter: fm}}
}

// hasSkill reports whether skills names skill.
func hasSkill(skills []string, skill string) bool {
	return slices.Contains(skills, skill)
}
