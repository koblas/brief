package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// UninstallRequest is Uninstall's input. Host selects which agent-host
// integration's artifacts to plan for removal; DryRun computes the plan
// without removing anything; Force removes an edited artifact instead of
// keeping it. Unlike InitRequest, there is no WithAgents: role-agent files
// are always planned for removal under HostClaudeCode.
type UninstallRequest struct {
	Host   string
	DryRun bool
	Force  bool
}

// Uninstall plans then, unless req.DryRun, applies the removal of
// everything Init installed, config file last so a failure partway
// through never removes the repository's opt-in marker while something
// else still is. Recognition is digest-only, so an invalid or locally
// edited file is "edited locally", kept unless --force. A failure after
// at least one artifact was already removed or rewritten is wrapped in
// ErrPartialWrite.
func (s *Server) Uninstall(_ context.Context, wd string, req UninstallRequest) (Result, error) {
	if !validHost(req.Host) {
		return Result{}, fmt.Errorf("%q: %w", req.Host, ErrUnknownHost)
	}

	fsys := s.fsRoot()

	nearest, err := locateInRepo(fsys, wd)
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

		snippetArt, present, err = planSnippetRemoval(fsys, root, h, req.Force)
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

			art, present, err := planPluginRemoval(fsys, path, KindSkill, f.Kind, req.Force)
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

			if cfg, _, inspectErr := inspectConfig(fsys, nearest); inspectErr == nil {
				boundAgentArts, err = planBoundAgentRemovals(s.resolveRoot, root, home, cfg.Roles)
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

			art, present, err := planPluginRemoval(fsys, path, KindAgent, f.Kind, req.Force)
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

			art, present, err := planPluginRemoval(fsys, path, kind, f.Kind, req.Force)
			if err != nil {
				return Result{}, err
			}

			if present {
				res.Artifacts = append(res.Artifacts, art)
			}
		}
	}

	configArt, present, err := planConfigRemoval(fsys, nearest, req.Force)
	if err != nil {
		return Result{}, err
	}

	if present {
		res.Artifacts = append(res.Artifacts, configArt)
	}

	if req.DryRun {
		return res, nil
	}

	return applyUninstall(fsys, res, root, req.Host, snippetArt, hasSnippet, boundAgentArts)
}

// planPluginRemoval decides one plugin file's removal Artifact: a current
// or older render is always ActionRemoved; edited content is force-gated.
func planPluginRemoval(fsys rwfs.FS, path string, kind Kind, renderKind artifact.Kind, force bool) (Artifact, bool, error) {
	info, err := fsys.Lstat(fsName(path))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Artifact{}, false, nil
	case err != nil:
		return Artifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "not a regular file"}, true, nil
	}

	body, err := fsys.ReadFile(fsName(path))
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

// pluginPruneDirs lists every directory applyUninstall may remove once empty, deepest first.
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
// exists and is empty. A directory still holding a file brief did not
// write, or that never existed, is left alone rather than removed.
func pruneEmptyPluginDirs(fsys rwfs.FS, root string) error {
	for _, rel := range pluginPruneDirs {
		dir := filepath.Join(root, rel)

		entries, err := fsys.ReadDir(fsName(dir))

		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("setup: read %s: %w", dir, err)
		case len(entries) > 0:
			continue
		}

		if err := fsys.Remove(fsName(dir)); err != nil {
			return fmt.Errorf("setup: remove %s: %w", dir, err)
		}
	}

	return nil
}

// planConfigRemoval decides the config file's removal Artifact: a current
// render is always ActionRemoved; any other content is force-gated, the
// same as planPluginRemoval.
func planConfigRemoval(fsys rwfs.FS, path string, force bool) (Artifact, bool, error) {
	if path == "" {
		return Artifact{}, false, nil
	}

	info, err := fsys.Lstat(fsName(path))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Artifact{}, false, nil
	case err != nil:
		return Artifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: KindConfig, Path: path, Action: ActionKept, Detail: "not a regular file"}, true, nil
	}

	body, err := fsys.ReadFile(fsName(path))
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

// applyUninstall strips or deletes the CLAUDE.md block, rewrites every
// boundAgentArts entry in place (never deleted), then removes every other
// ActionRemoved artifact, then, for hostName == HostClaudeCode, prunes the
// plugin's now-empty directories. A failure after an earlier write landed
// is wrapped in ErrPartialWrite.
func applyUninstall(fsys rwfs.FS, res Result, root, hostName string, snippetArt snippetArtifact, hasSnippet bool, boundAgentArts []boundAgentArtifact) (Result, error) {
	var removedAny bool

	if hasSnippet && snippetArt.Action == ActionRemoved {
		if err := verifyFileUnchanged(fsys, snippetArt.Path, true, snippetArt.existing, "brief uninstall"); err != nil {
			return Result{}, err
		}

		if len(snippetArt.remains) == 0 {
			if err := fsys.Remove(fsName(snippetArt.Path)); err != nil {
				wrapped := fmt.Errorf("setup: %w", err)

				if removedAny {
					return res, markPartial(wrapped)
				}

				return Result{}, wrapped
			}

			res.Removed = append(res.Removed, snippetArt.Path)
		} else {
			if err := writeSnippetFile(fsys, snippetArt.Path, snippetArt.remains); err != nil {
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
		if err := verifyBoundAgentUnchanged(ba, "brief uninstall"); err != nil {
			if removedAny {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if err := ba.agentFile().write(ba.edited, ba.perm); err != nil {
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

		if err := fsys.Remove(fsName(a.Path)); err != nil {
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
		if err := pruneEmptyPluginDirs(fsys, root); err != nil {
			if removedAny {
				return res, markPartial(err)
			}

			return Result{}, err
		}
	}

	return res, nil
}
