package agentfile

import (
	"io/fs"
	"path"
	"path/filepath"
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
// unbound or unresolved one. fsys and name, set only for a BindingResolved
// BindingBrief binding, are the Tree's own FS and Path's own name inside
// it — LackingSkill reads the resolved file through them rather than
// reopening Path from scratch; a Binding built any other way (the zero
// value, or one assembled by a caller directly) carries neither, and
// LackingSkill falls back to Load(Path).
type Binding struct {
	Kind  BindingKind
	State BindingState
	Path  string
	Defs  []Definition
	fsys  fs.FS
	name  string
}

// ResolveBinding classifies value (a config.RoleBindings field) against
// root and, for a bare name, home (Rule 5). It is ResolveBindingIn over
// DirTree(root) and DirTree(home) — the OS adapter, consistent with Find
// over FindIn.
func ResolveBinding(root, home, value string) Binding {
	return ResolveBindingIn(DirTree(root), DirTree(home), value)
}

// ResolveBindingIn classifies value against project and, for a bare name,
// user (Rule 5): "" is BindingUnbound. A "brief:<name>" binding is
// BindingResolved via project's own ".claude/agents/<name>.md", overriding
// project's own ".claude/skills/brief/agents/<name>.md" when both are
// regular files, BindingUnresolved when neither is (including when
// project is the zero Tree — there is nothing to check it against). Any
// other "<plugin>:<name>" is BindingUnverified. A bare "<name>" is
// BindingResolved when FindIn(project, user, name) returns at least one
// Definition, BindingUnresolved otherwise — Path and Defs then carry its
// first and every result respectively, in the scope FindIn chose (project
// agents shadow a same-named user one entirely, never mixed).
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

// resolveBriefBinding is ResolveBindingIn's own "brief:<agent>" case:
// project's own ".claude/agents/<agent>.md" overrides
// project's own ".claude/skills/brief/agents/<agent>.md" — host.PluginDir
// joined the same way, project-relative — when both are regular files,
// BindingUnresolved when neither is, including when project carries no FS
// to check either against. agent's own value is joined and path.Clean-ed
// onto each candidate directory before being checked; a value whose ".."
// climbs the cleaned result back out of that directory — landing on some
// other file the project tree happens to hold, rather than escaping the
// tree outright, which path.Join alone does not catch — is never treated
// as a match for either candidate, regardless of what sits at the escaped
// path.
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

// LackingSkill returns every Definition behind b whose own frontmatter
// "skills:" does not name skill — nil when b is not BindingResolved, or
// every resolved Definition already names it. For a BindingBare binding
// this filters b.Defs directly, in Find's own order. For a BindingBrief
// binding, which carries no Defs, it synthesizes one Definition by
// decoding the resolved file's own frontmatter: through b.fsys/b.name when
// ResolveBindingIn set them, LoadFS's own read of the same file
// ResolveBindingIn's own fileIsRegularFS check already found — or,
// for a Binding assembled any other way, Load(b.Path) directly. A decode
// failure counts as lacking, its Definition carrying a zero Frontmatter
// (the file's own omitClaudeMd is then unknown), Scope always
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

// loadBrief decodes a BindingBrief binding's own resolved file: through
// b.fsys/b.name when set, else Load(b.Path).
func (b Binding) loadBrief() (Frontmatter, error) {
	if b.fsys != nil {
		return LoadFS(b.fsys, b.name)
	}

	return Load(b.Path)
}
