package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
)

// UninstallRequest is Uninstall's own input: Host selects which agent-host
// integration's own artifacts to plan for removal (Hosts, HostNone in this
// release — a later scenario's host planners key off it the same way
// Init's InitRequest.Host does), DryRun computes the same plan without
// removing anything, and Force removes an edited artifact instead of
// keeping it.
type UninstallRequest struct {
	Host   string
	DryRun bool
	Force  bool
}

// Uninstall plans then, unless req.DryRun, applies the removal of
// everything Init installed: today, only the config file. Recognition is
// digest-only (artifact.Recognize) — Uninstall never decodes the config
// the way Init does, so it has no config-refusal class of its own; an
// invalid or unparseable file is simply "edited locally", the same as any
// other byte mismatch. No config found anywhere (config.Locate's own
// walk-up) means zero artifacts, reported by cli as "nothing installed".
//
// Artifacts is assembled as a list so a later scenario's host planners can
// append their own artifacts ahead of the config file's — the config,
// this repository's opt-in marker, is always removed last, so a failure
// partway through never removes it while something else still is. Apply
// walks that same list in order; a failure after at least one artifact
// was already removed is wrapped in ErrPartialWrite, distinguishing a
// partial uninstall from one that changed nothing.
func (s *Server) Uninstall(_ context.Context, wd string, req UninstallRequest) (Result, error) {
	if !validHost(req.Host) {
		return Result{}, fmt.Errorf("%q: %w", req.Host, ErrUnknownHost)
	}

	nearest, _, err := config.Locate(wd)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Host:      req.Host,
		DryRun:    req.DryRun,
		Artifacts: []Artifact{},
		Created:   []string{},
		Modified:  []string{},
		Removed:   []string{},
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

	return applyUninstall(res)
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
// ActionKept, detail "edited locally", unless Force is set, in which case
// they report ActionRemoved with that same detail.
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

	action := ActionKept
	if force {
		action = ActionRemoved
	}

	return Artifact{Kind: KindConfig, Path: path, Action: action, Detail: "edited locally"}, true, nil
}

// applyUninstall removes every res.Artifacts entry reporting ActionRemoved,
// in list order, appending each removed path to res.Removed as it lands. A
// removal failure after at least one artifact was already removed is
// wrapped in ErrPartialWrite; a failure before any removal landed is
// returned as-is.
func applyUninstall(res Result) (Result, error) {
	var removedAny bool

	for _, a := range res.Artifacts {
		if a.Action != ActionRemoved {
			continue
		}

		if err := os.Remove(a.Path); err != nil {
			wrapped := fmt.Errorf("setup: remove %s: %w", a.Path, err)

			if removedAny {
				return Result{}, markPartial(wrapped)
			}

			return Result{}, wrapped
		}

		res.Removed = append(res.Removed, a.Path)
		removedAny = true
	}

	return res, nil
}
