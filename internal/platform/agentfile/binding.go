package agentfile

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/host"
)

// BindingKind classifies a role binding's configured value shape,
// independent of whether it resolved to a file: BindingBare for a plain
// name, BindingBrief for a "brief:<name>" value, BindingPlugin for any
// other "<plugin>:<name>" value.
type BindingKind int

const (
	// BindingBare marks a binding with no "<plugin>:" prefix.
	BindingBare BindingKind = iota
	// BindingBrief marks a "brief:<name>" binding.
	BindingBrief
	// BindingPlugin marks a "<plugin>:<name>" binding for another plugin.
	BindingPlugin
)

// BindingState classifies a Binding's resolution against the filesystem.
type BindingState int

const (
	// BindingUnbound marks an empty binding value.
	BindingUnbound BindingState = iota
	// BindingResolved marks a binding whose agent file was found.
	BindingResolved
	// BindingUnresolved marks a non-empty binding whose agent file was not found.
	BindingUnresolved
	// BindingUnverified marks a BindingPlugin binding: no file layout
	// exists to check it against, so it counts as bound without verification.
	BindingUnverified
)

// Binding is ResolveBinding's result: Kind classifies the configured
// value's shape, State classifies its resolution, Path carries the
// resolved file's absolute path when State is BindingResolved, and Defs
// carries every Definition a BindingBare binding's Find returned. fsys and
// name, set only for a resolved BindingBrief, let LackingSkill read the
// resolved file directly instead of reopening Path.
type Binding struct {
	Kind  BindingKind
	State BindingState
	Path  string
	Defs  []Definition
	fsys  fs.FS
	name  string
}

// ResolveBinding classifies value (a config.RoleBindings field) against
// root and, for a bare name, home: ResolveBindingIn over
// DirTree(root) and DirTree(home).
func ResolveBinding(root, home, value string) Binding {
	return ResolveBindingIn(DirTree(root), DirTree(home), value)
}

// ResolveBindingIn classifies value against project and, for a bare name,
// user: "" is BindingUnbound. A "brief:<name>" binding resolves
// against project's ".claude/agents/<name>.md", falling back to
// ".claude/skills/brief/agents/<name>.md". Any other "<plugin>:<name>" is
// BindingUnverified. A bare "<name>" is BindingResolved when
// FindIn(project, user, name) returns at least one Definition.
func ResolveBindingIn(project, user Tree, value string) Binding {
	if value == "" {
		return Binding{State: BindingUnbound}
	}

	if plugin, agent, ok := strings.Cut(value, ":"); ok {
		if plugin != "brief" {
			return Binding{Kind: BindingPlugin, State: BindingUnverified}
		}

		return resolveBriefBinding(project, agent)
	}

	defs := FindIn(project, user, value)
	if len(defs) == 0 {
		return Binding{Kind: BindingBare, State: BindingUnresolved}
	}

	return Binding{Kind: BindingBare, State: BindingResolved, Path: defs[0].Path, Defs: defs}
}

// resolveBriefBinding is ResolveBindingIn's "brief:<agent>" case:
// ".claude/agents/<agent>.md" overrides ".claude/skills/brief/agents/<agent>.md"
// when both are regular files, BindingUnresolved otherwise. agent is
// path.Join-cleaned onto each candidate directory first, so a value whose
// ".." climbs back out of that directory never matches.
func resolveBriefBinding(project Tree, agent string) Binding {
	if project.FS == nil {
		return Binding{Kind: BindingBrief, State: BindingUnresolved}
	}

	const overrideDir = ".claude/agents"
	pluginDir := host.PluginDir + "/agents"

	overrideName := path.Join(".claude", "agents", agent+".md")
	pluginName := path.Join(host.PluginDir, "agents", agent+".md")

	switch {
	case withinDir(overrideName, overrideDir) && fileIsRegularFS(project.FS, overrideName):
		return Binding{
			Kind: BindingBrief, State: BindingResolved,
			Path: filepath.Join(project.Dir, filepath.FromSlash(overrideName)),
			fsys: project.FS, name: overrideName,
		}
	case withinDir(pluginName, pluginDir) && fileIsRegularFS(project.FS, pluginName):
		return Binding{
			Kind: BindingBrief, State: BindingResolved,
			Path: filepath.Join(project.Dir, filepath.FromSlash(pluginName)),
			fsys: project.FS, name: pluginName,
		}
	default:
		return Binding{Kind: BindingBrief, State: BindingUnresolved}
	}
}

// withinDir reports whether name, already path.Join-cleaned, still sits
// inside dir rather than having climbed back out of it through a ".."
// element in the value that produced it.
func withinDir(name, dir string) bool {
	return strings.HasPrefix(name, dir+"/")
}

// fileIsRegularFS reports whether name exists in fsys and is a regular
// file, following a symlink fsys itself follows (fs.Stat, not fs.Lstat).
func fileIsRegularFS(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)

	return err == nil && info.Mode().IsRegular()
}

// LackingSkill returns every Definition behind b whose frontmatter
// "skills:" does not name skill — nil when b is not BindingResolved, or
// every resolved Definition already names it. A BindingBrief binding
// carries no Defs, so it decodes the resolved file itself; a decode
// failure counts as lacking, with a zero Frontmatter.
func (b Binding) LackingSkill(skill string) []Definition {
	if b.State != BindingResolved {
		return nil
	}

	if b.Kind == BindingBare {
		var out []Definition

		for _, d := range b.Defs {
			if !d.Frontmatter.HasSkill(skill) {
				out = append(out, d)
			}
		}

		return out
	}

	fm, err := b.loadBrief()
	if err != nil {
		return []Definition{{Path: b.Path, Scope: ScopeProject}}
	}

	if fm.HasSkill(skill) {
		return nil
	}

	return []Definition{{Path: b.Path, Scope: ScopeProject, Frontmatter: fm}}
}

// loadBrief decodes a BindingBrief binding's resolved file: through
// b.fsys/b.name when set, else Load(b.Path).
func (b Binding) loadBrief() (Frontmatter, error) {
	if b.fsys != nil {
		return LoadFS(b.fsys, b.name)
	}

	return Load(b.Path)
}
