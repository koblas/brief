package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/host"
)

// UninstallRequest is Uninstall's own input: Host selects which agent-host
// integration's own artifacts to plan for removal (Hosts), DryRun computes
// the same plan without removing anything, and Force removes an edited
// artifact instead of keeping it. Unlike InitRequest, UninstallRequest
// carries no WithAgents of its own: for HostClaudeCode, the three
// role-agent files (host.Host.Agents) are always planned for removal,
// whether or not the install that put them there — or this Uninstall call
// itself — ever named --with-agents.
type UninstallRequest struct {
	Host   string
	DryRun bool
	Force  bool
}

// Uninstall plans then, unless req.DryRun, applies the removal of
// everything Init installed: for req.Host == HostClaudeCode, the CLAUDE.md
// block, then, unless the skill is itself being kept (Rule 8's own gate,
// below), one KindBoundAgent row per planBoundAgentRemovals target, that
// host's own agent files (host.Host.Agents, always planned) and plugin
// files, then the config file; for HostNone, the config file alone.
// Recognition is digest-only (artifact.Recognize) — Uninstall never decodes
// the config, or a plugin or agent file, the way Init does, so it has no
// refusal class of its own; an invalid, unparseable, or locally edited file
// is simply "edited locally", the same as any other byte mismatch, kept
// unless --force. An OriginOlder file (Rule 6) is not an edit: it is
// removed the same as a current one, without --force, since its bytes are
// still brief's own, just an earlier release's. No config found anywhere
// (config.LocateInRepo's own bounded
// walk-up, R3 — an ancestor config above the nearest enclosing git
// repository is treated as though it did not exist, the same rule Init
// applies to its own install root) and no plugin or agent file found either
// means zero artifacts, reported by cli as "nothing installed".
//
// Artifacts lists, for HostClaudeCode, the CLAUDE.md block first
// (planSnippetRemoval), then Rule 8's own bound-agent rows, then the three
// agent files reversed (reviewer, implementer, planner), then a
// claude-code host's own plugin files — hook wiring, finish skill, start
// skill, manifest, the reverse of the order Init installs them in — ahead
// of the config file, always last: the config, this repository's opt-in
// marker, is always removed last, so a failure partway through never
// removes it while something else still is. A plugin or agent file Lstat
// finds missing (never installed, or a --no-hook init's own hook file)
// plans no row and no error; a CLAUDE.md candidate found but carrying no
// recognized block plans no row either.
//
// Rule 8's own gate: the bound-agent rows are planned only when the
// brief-workflow skill's own row is not itself ActionKept — an edited
// SKILL.md without --force, or one that is not a regular file even with
// --force. Roles come from config.Inspect(nearest): nearest == "" or any
// read or decode failure means no bound rows at all, never a refusal —
// Uninstall's own config removal stays digest-only, the same as everywhere
// else in this file. home is s.homeDir, "" on failure.
//
// Apply strips or deletes the CLAUDE.md block first, then rewrites every
// bound-agent target in place (never deleting one), then removes every
// other ActionRemoved artifact in that same order, then, for
// HostClaudeCode, prunes the plugin's own now-empty directories
// deepest-first ("agents/" included), stopping at host.PluginDir — never
// above it, and never touching a directory still holding a file brief did
// not write. A failure after at least one artifact was already removed or
// rewritten is wrapped in ErrPartialWrite, distinguishing a partial
// uninstall from one that changed nothing.
func (s *Server) Uninstall(_ context.Context, wd string, req UninstallRequest) (Result, error) {
	if !validHost(req.Host) {
		return Result{}, fmt.Errorf("%q: %w", req.Host, ErrUnknownHost)
	}

	nearest, _, err := config.LocateInRepo(wd)
	if err != nil {
		return Result{}, err
	}

	root := wd
	if nearest != "" {
		root = filepath.Dir(nearest)
	}

	res := Result{
		Host:               req.Host,
		DryRun:             req.DryRun,
		Root:               root,
		Artifacts:          []Artifact{},
		Created:            []string{},
		Modified:           []string{},
		Removed:            []string{},
		AgentsMissingSkill: []MissingSkillAgent{},
	}

	var (
		snippetArt     snippetArtifact
		hasSnippet     bool
		boundAgentArts []boundAgentArtifact
	)

	if req.Host == HostClaudeCode {
		h, _ := host.Lookup(host.ClaudeCode)

		var present bool

		snippetArt, present, err = planSnippetRemoval(root, h, req.Force)
		if err != nil {
			return Result{}, err
		}

		if present {
			res.Artifacts = append(res.Artifacts, snippetArt.Artifact)
			hasSnippet = true
		}

		var (
			skillArts []Artifact
			skillKept bool
		)

		for _, f := range h.Skills() {
			path := filepath.Join(root, filepath.FromSlash(f.RelPath))

			art, present, err := planPluginRemoval(path, KindSkill, f.Kind, req.Force)
			if err != nil {
				return Result{}, err
			}

			if present {
				skillArts = append(skillArts, art)

				if art.Action == ActionKept {
					skillKept = true
				}
			}
		}

		if !skillKept && nearest != "" {
			home, homeErr := s.homeDir()
			if homeErr != nil {
				home = ""
			}

			if cfg, _, inspectErr := config.Inspect(nearest); inspectErr == nil {
				boundAgentArts, err = planBoundAgentRemovals(root, home, cfg.Roles)
				if err != nil {
					return Result{}, err
				}
			}
		}

		for _, ba := range boundAgentArts {
			res.Artifacts = append(res.Artifacts, ba.Artifact)
		}

		for _, f := range slices.Backward(h.Agents()) {
			path := filepath.Join(root, filepath.FromSlash(f.RelPath))

			art, present, err := planPluginRemoval(path, KindAgent, f.Kind, req.Force)
			if err != nil {
				return Result{}, err
			}

			if present {
				res.Artifacts = append(res.Artifacts, art)
			}
		}

		res.Artifacts = append(res.Artifacts, skillArts...)

		files := h.Plugin(true)

		for _, f := range slices.Backward(files) {
			kind := KindPlugin
			if f.Hook {
				kind = KindHook
			}

			path := filepath.Join(root, filepath.FromSlash(f.RelPath))

			art, present, err := planPluginRemoval(path, kind, f.Kind, req.Force)
			if err != nil {
				return Result{}, err
			}

			if present {
				res.Artifacts = append(res.Artifacts, art)
			}
		}
	}

	configArt, present, err := planConfigRemoval(nearest, req.Force)
	if err != nil {
		return Result{}, err
	}

	if present {
		res.Artifacts = append(res.Artifacts, configArt)
	}

	if req.DryRun {
		return res, nil
	}

	return applyUninstall(res, root, req.Host, snippetArt, hasSnippet, boundAgentArts)
}

// planPluginRemoval decides one plugin file's own removal Artifact,
// mirroring planConfigRemoval: a missing path reports present=false, no
// row at all. A path that exists but is not a regular file (os.Lstat — a
// directory, a symlink) reports ActionKept, detail "not a regular file",
// regardless of force — never followed. A regular file whose bytes are
// artifact.Recognize's OriginCurrent or OriginOlder for renderKind is
// always ActionRemoved, no detail — Rule 6: an earlier release's own render
// is not a local edit, so uninstall removes it the same as today's own,
// without needing --force. Any other bytes (OriginEdited) report detail
// "edited locally": unless force is set, ActionKept with ForceRemovable
// true; with force, ActionRemoved with ForceRemovable false — the field is
// true only while --force could still act on the artifact, never once it
// already has.
func planPluginRemoval(path string, kind Kind, renderKind artifact.Kind, force bool) (Artifact, bool, error) {
	info, err := os.Lstat(path)

	switch {
	case os.IsNotExist(err):
		return Artifact{}, false, nil
	case err != nil:
		return Artifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "not a regular file"}, true, nil
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	if origin := artifact.Recognize(renderKind, body); origin == artifact.OriginCurrent || origin == artifact.OriginOlder {
		return Artifact{Kind: kind, Path: path, Action: ActionRemoved}, true, nil
	}

	if force {
		return Artifact{Kind: kind, Path: path, Action: ActionRemoved, Detail: "edited locally"}, true, nil
	}

	return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "edited locally", ForceRemovable: true}, true, nil
}

// pluginPruneDirs lists every directory applyUninstall may remove once
// empty, deepest first: every directory under host.PluginDir, then
// host.PluginDir itself, then host.WorkflowSkillDir — R6's ownership
// boundary: brief owns the plugin directory and the workflow skill's own
// directory, never ".claude/skills/" or ".claude/" above either one, both
// of which may hold a host's or an adopter's own files.
var pluginPruneDirs = []string{
	filepath.Join(host.PluginDir, "skills", "start"),
	filepath.Join(host.PluginDir, "skills", "finish"),
	filepath.Join(host.PluginDir, "skills"),
	filepath.Join(host.PluginDir, "hooks"),
	filepath.Join(host.PluginDir, ".claude-plugin"),
	filepath.Join(host.PluginDir, "agents"),
	host.PluginDir,
	host.WorkflowSkillDir,
}

// pruneEmptyPluginDirs removes every pluginPruneDirs entry under root that
// exists and is empty, in list order (deepest first), via os.Remove — a
// directory still holding any entry (a file brief did not write, or a
// sibling not yet pruned) is left in place, and a directory that never
// existed is skipped without error.
func pruneEmptyPluginDirs(root string) error {
	for _, rel := range pluginPruneDirs {
		dir := filepath.Join(root, rel)

		entries, err := os.ReadDir(dir)

		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			return fmt.Errorf("setup: read %s: %w", dir, err)
		case len(entries) > 0:
			continue
		}

		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("setup: remove %s: %w", dir, err)
		}
	}

	return nil
}

// planConfigRemoval decides the config file's own removal Artifact: "" (no
// config found anywhere) reports present=false, no row at all. A path that
// exists but is not a regular file (os.Lstat — a directory, a symlink)
// reports ActionKept, detail "not a regular file", regardless of Force:
// Uninstall never calls RemoveAll and never follows a symlink to decide
// what it points at. A regular file whose bytes are the digest-recognized
// current render (artifact.Recognize's OriginCurrent) is always
// ActionRemoved, no detail. Any other bytes — edited, invalid, or
// unparseable; Uninstall never decodes to tell those apart — report
// detail "edited locally": unless Force is set, ActionKept with
// ForceRemovable true; with Force, ActionRemoved with ForceRemovable
// false — the field is true only while --force could still act on the
// artifact, never once it already has.
func planConfigRemoval(path string, force bool) (Artifact, bool, error) {
	if path == "" {
		return Artifact{}, false, nil
	}

	info, err := os.Lstat(path)

	switch {
	case os.IsNotExist(err):
		return Artifact{}, false, nil
	case err != nil:
		return Artifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: KindConfig, Path: path, Action: ActionKept, Detail: "not a regular file"}, true, nil
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	if artifact.Recognize(artifact.KindConfig, body) == artifact.OriginCurrent {
		return Artifact{Kind: KindConfig, Path: path, Action: ActionRemoved}, true, nil
	}

	if force {
		return Artifact{Kind: KindConfig, Path: path, Action: ActionRemoved, Detail: "edited locally"}, true, nil
	}

	return Artifact{Kind: KindConfig, Path: path, Action: ActionKept, Detail: "edited locally", ForceRemovable: true}, true, nil
}

// applyUninstall strips or deletes the CLAUDE.md block first (when
// hasSnippet and snippetArt reports ActionRemoved: a rewrite with its
// remaining bytes goes to res.Modified, an emptied file is deleted and
// goes to res.Removed), then rewrites every boundAgentArts entry in place
// (Rule 8: verifyFileUnchanged against a concurrent edit, then
// writeBoundAgent, which preserves the file's own mode — never deleted, so
// it is appended to res.Modified, not res.Removed), then removes every
// other res.Artifacts entry reporting ActionRemoved except a KindBoundAgent
// one (already handled above), in list order, appending each removed path
// to res.Removed as it lands, then, for hostName == HostClaudeCode, prunes
// the plugin's own now-empty directories (pruneEmptyPluginDirs) — never
// added to res.Removed, which names files only, symmetric with
// Result.Created. A failure after at least one earlier write already
// landed is wrapped in ErrPartialWrite; a failure before any write landed
// is returned as-is.
func applyUninstall(res Result, root, hostName string, snippetArt snippetArtifact, hasSnippet bool, boundAgentArts []boundAgentArtifact) (Result, error) {
	var removedAny bool

	if hasSnippet && snippetArt.Action == ActionRemoved {
		if err := verifyFileUnchanged(snippetArt.Path, true, snippetArt.existing, "brief uninstall"); err != nil {
			return Result{}, err
		}

		if len(snippetArt.remains) == 0 {
			if err := os.Remove(snippetArt.Path); err != nil {
				wrapped := fmt.Errorf("setup: %w", err)

				if removedAny {
					return res, markPartial(wrapped)
				}

				return Result{}, wrapped
			}

			res.Removed = append(res.Removed, snippetArt.Path)
		} else {
			if err := writeSnippetFile(snippetArt.Path, snippetArt.remains); err != nil {
				if removedAny {
					return res, markPartial(err)
				}

				return Result{}, err
			}

			res.Modified = append(res.Modified, snippetArt.Path)
		}

		removedAny = true
	}

	for _, ba := range boundAgentArts {
		if err := verifyFileUnchanged(ba.Path, true, ba.existing, "brief uninstall"); err != nil {
			if removedAny {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if err := writeBoundAgent(ba.Path, ba.edited, ba.perm); err != nil {
			if removedAny {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		res.Modified = append(res.Modified, ba.Path)
		removedAny = true
	}

	for _, a := range res.Artifacts {
		if a.Kind == KindSnippet || a.Kind == KindBoundAgent || a.Action != ActionRemoved {
			continue
		}

		if err := os.Remove(a.Path); err != nil {
			wrapped := fmt.Errorf("setup: %w", err)

			if removedAny {
				return res, markPartial(wrapped)
			}

			return Result{}, wrapped
		}

		res.Removed = append(res.Removed, a.Path)
		removedAny = true
	}

	if hostName == HostClaudeCode {
		if err := pruneEmptyPluginDirs(root); err != nil {
			if removedAny {
				return res, markPartial(err)
			}

			return Result{}, err
		}
	}

	return res, nil
}
