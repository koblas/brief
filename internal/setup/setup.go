package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/repo"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// configFileName is the config file Init writes and looks for.
const configFileName = ".brief.yaml"

// HostNone means no agent-host integration is installed — only the config and feature root.
const HostNone = "none"

// HostClaudeCode installs a Claude Code skills-directory plugin alongside the config and feature root.
const HostClaudeCode = host.ClaudeCode

// Hosts returns every host InitRequest.Host and UninstallRequest.Host
// accept, in the order a usage error's "expected one of:" clause lists
// them.
func Hosts() []string {
	return []string{HostClaudeCode, HostNone}
}

// Kind names what an Artifact reports: the config file, the feature root
// directory, a Claude Code plugin file (manifest or skill), or its hook
// wiring.
type Kind string

const (
	// KindConfig is the ".brief.yaml" config file.
	KindConfig Kind = "config"
	// KindFeatureRoot is the configured feature directory.
	KindFeatureRoot Kind = "feature-root"
	// KindPlugin is a host's plugin manifest or skill file.
	KindPlugin Kind = "plugin"
	// KindHook is a host's hook wiring file.
	KindHook Kind = "hook"
	// KindAgent is one of a host's three role-agent files, installed only under InitRequest.WithAgents.
	KindAgent Kind = "agent"
	// KindSkill is the brief-workflow skill file, installed on every claude-code install.
	KindSkill Kind = "skill"
	// KindSnippet is the CLAUDE.md instruction block.
	KindSnippet Kind = "snippet"
	// KindBoundAgent is a repository agent file InitRequest.EditAgents edited, or tried to, to add the workflow skill.
	KindBoundAgent Kind = "bound-agent"
)

// Action names what Init did, or would do, to one Artifact.
type Action string

const (
	// ActionCreated marks an artifact that did not exist and was written, or, for the config file under --force, one rewritten from defaults.
	ActionCreated Action = "created"
	// ActionUnchanged marks an artifact whose bytes already matched what Init would have written.
	ActionUnchanged Action = "unchanged"
	// ActionKept marks an artifact left unwritten because it was edited locally, or, for Uninstall, any artifact left on disk.
	ActionKept Action = "kept"
	// ActionRemoved marks an artifact Uninstall deleted.
	ActionRemoved Action = "removed"
	// ActionMerged marks a rewrite in place: the CLAUDE.md block merged into an existing file, or a plugin, skill or agent file replaced wholesale with today's render.
	ActionMerged Action = "merged"
)

// Server plans and applies brief's install write path. homeDir, fsRoot,
// resolveRoot and writableCheck are injected seams so a test can substitute
// an in-memory filesystem and skip real-disk checks; production uses
// NewServer's defaults throughout.
type Server struct {
	homeDir       func() (string, error)
	fsRoot        func() rwfs.FS
	resolveRoot   func(string) (string, error)
	writableCheck func([]string) error
}

// Option configures a Server built by NewServer.
type Option func(*Server)

// WithFSRoot overrides the filesystem every read and write under a
// repository root goes through, so a test can substitute an rwfs.Mem. It
// never affects the confined bound-agent file reads and writes
// (bound_agent.go), which always use real disk.
func WithFSRoot(fsys rwfs.FS) Option {
	return func(s *Server) { s.fsRoot = func() rwfs.FS { return fsys } }
}

// WithResolveRoot overrides the resolveRoot used to symlink-resolve root
// before walking agent bindings. Never affects bound-agent file reads and
// writes, which stay on real disk regardless.
func WithResolveRoot(fn func(string) (string, error)) Option {
	return func(s *Server) { s.resolveRoot = fn }
}

// WithWritableCheck overrides the pre-apply writability check Init runs
// before applying, so a WithFSRoot-backed test doesn't consult real disk.
func WithWritableCheck(fn func([]string) error) Option {
	return func(s *Server) { s.writableCheck = fn }
}

// NewServer returns a Server with production defaults: homeDir
// os.UserHomeDir, fsRoot the real disk filesystem, resolveRoot
// filepath.EvalSymlinks, and writableCheck checkWritable.
func NewServer(opts ...Option) *Server {
	s := &Server{
		homeDir:       os.UserHomeDir,
		fsRoot:        func() rwfs.FS { return diskFS{} },
		resolveRoot:   filepath.EvalSymlinks,
		writableCheck: checkWritable,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// locateInRepo is config.LocateInRepo's fsys-backed twin: a config found
// above the nearest enclosing git repository is treated as not found.
func locateInRepo(fsys fs.FS, wd string) (string, error) {
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", fmt.Errorf("resolve config: %w", err)
	}

	boundary := ""
	if root, ok := repo.RootFS(fsys, abs); ok {
		boundary = root
	}

	nearest, _, err := config.LocateWithinFS(fsys, abs, boundary)
	if err != nil {
		return "", fmt.Errorf("resolve config: %w", err)
	}

	return nearest, nil
}

// inspectConfig is config.Inspect's fsys-backed twin; abs is already
// absolute, so unlike config.Inspect this never calls filepath.Abs.
func inspectConfig(fsys fs.FS, abs string) (config.Config, []*config.ValueError, error) {
	cfg, violations, err := config.InspectFS(fsys, abs)
	if err != nil {
		if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
			return config.Config{}, nil, fmt.Errorf("resolve config: %w", invalidCfg)
		}

		return config.Config{}, nil, fmt.Errorf("resolve config: %s: %w", abs, err)
	}

	return cfg, violations, nil
}

// writeThrough replaces name's bytes under fsys, wrapping an open failure
// as "setup: open %s" and any other failure as "setup: write %s", using
// path (the caller's absolute display path) rather than the fsys-relative
// error path.
func writeThrough(fsys rwfs.FS, name, path string, body []byte) error {
	err := fsys.WriteFile(name, body, 0o644)
	if err == nil {
		return nil
	}

	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		if pe.Op == "open" {
			return fmt.Errorf("setup: open %s: %w", filepath.Dir(path), pe.Err)
		}

		return fmt.Errorf("setup: write %s: %w", path, pe.Err)
	}

	return fmt.Errorf("setup: write %s: %w", path, err)
}

// InitRequest is Init's input. Host selects the agent-host integration;
// "" detects one instead of refusing. WithAgents and EditAgents each
// require a resolved HostClaudeCode. DryRun and Print both compute the
// plan without writing; Force rewrites an existing config from defaults.
type InitRequest struct {
	Host string
	// NoHook omits a claude-code host's hook wiring file entirely.
	NoHook bool
	// WithAgents installs the three role-agent files and, only when this
	// run also creates the config, binds every role to them.
	WithAgents bool
	// EditAgents edits every bare-name planner or implementer binding's
	// agent file to add the workflow skill, where its "skills:" shape allows it.
	EditAgents bool
	DryRun     bool
	Force      bool
	// Print additionally skips the pre-apply writability check.
	Print bool
}

// Artifact is one thing Init installs, or Init or Uninstall found already
// installed. Kind and Path identify it, Action reports what happened, and
// Detail carries an optional note. ForceRemovable is true only for an
// Uninstall-side ActionKept artifact that --force would turn into
// ActionRemoved; it is always false on an Init-side artifact.
type Artifact struct {
	Kind           Kind
	Path           string
	Action         Action
	Detail         string
	ForceRemovable bool
}

// Result is what Init and Uninstall both return: Host and DryRun echo the
// request, Root is the absolute install root, and Artifacts lists every
// managed file, in the fixed order each command's stdout rows render in,
// and what happened to it. Created, Modified and Removed name the paths
// actually written or deleted, in that same order; no slice is ever nil.
type Result struct {
	Host      string
	DryRun    bool
	Root      string
	Artifacts []Artifact
	Created   []string
	Modified  []string
	Removed   []string
	// RolesToAdd is Init's hint: empty unless WithAgents left a role
	// unbound in a config not written this run, else a "roles:" header
	// plus one line per unbound role. Populated even under DryRun.
	RolesToAdd []string
	// NoHostDetected is true only when Host was "" and detection found nothing.
	NoHostDetected bool
	// DetectedBy names the signal that resolved an empty Host (".claude",
	// "CLAUDE.md", or "~/.claude"); "" when Host was given explicitly or NoHostDetected is true.
	DetectedBy string
	// Print is the pending-artifact set, always populated.
	Print []PrintArtifact
	// AgentsMissingSkill lists bindings whose resolved agent lacks the
	// workflow skill. Populated only for a HostClaudeCode Init.
	AgentsMissingSkill []MissingSkillAgent
}

// Init plans then, unless req.DryRun or req.Print, applies brief's
// install: the config file, the feature root, and for a resolved
// HostClaudeCode that host's plugin, skill and agent files plus its
// CLAUDE.md block. Every refusal is decided during planning, before any
// artifact is touched, so DryRun returns exactly the plan a real run would
// apply. A write failure after an earlier write already landed is wrapped
// in ErrPartialWrite.
func (s *Server) Init(_ context.Context, wd string, req InitRequest) (Result, error) {
	if req.Host != "" && !validHost(req.Host) {
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

	var (
		noHostDetected bool
		detectedBy     string
	)

	if req.Host == "" {
		var detected bool

		req.Host, detected, detectedBy = detectHost(fsys, root, s.homeDir)
		noHostDetected = !detected
	}

	if req.WithAgents && req.Host != HostClaudeCode {
		return Result{}, ErrAgentsNeedHost
	}

	if req.EditAgents && req.Host != HostClaudeCode {
		return Result{}, ErrEditAgentsNeedHost
	}

	configArt, cfg, err := planConfig(fsys, nearest, root, req.Force, req.WithAgents)
	if err != nil {
		return Result{}, err
	}

	featureRoot := filepath.Join(root, cfg.FeatureDirectory)

	featureArt, err := planFeatureRoot(fsys, featureRoot)
	if err != nil {
		return Result{}, err
	}

	var (
		pluginArts             []pluginArtifact
		skillArts              []pluginArtifact
		agentArts              []pluginArtifact
		boundAgentArts         []boundAgentArtifact
		snippetArt             snippetArtifact
		hasSnippet             bool
		agentsMissingSkillList = []MissingSkillAgent{}
	)

	if req.Host == HostClaudeCode {
		h, _ := host.Lookup(host.ClaudeCode)

		pluginArts, err = planPluginFiles(fsys, root, h, !req.NoHook)
		if err != nil {
			return Result{}, err
		}

		skillArts, err = planSkillFiles(fsys, root, h)
		if err != nil {
			return Result{}, err
		}

		if req.WithAgents {
			agentArts, err = planAgentFiles(fsys, root, h)
			if err != nil {
				return Result{}, err
			}
		}

		snippetArt, err = planSnippet(fsys, root, h, cfg.FeatureDirectory)
		if err != nil {
			return Result{}, err
		}

		hasSnippet = true

		home, homeErr := s.homeDir()
		if homeErr != nil {
			home = ""
		}

		if req.EditAgents {
			boundAgentArts, err = planBoundAgents(s.resolveRoot, root, home, cfg.Roles)
			if err != nil {
				return Result{}, err
			}
		}

		agentsMissingSkillList, err = agentsMissingSkill(s.resolveRoot, root, home, cfg.Roles)
		if err != nil {
			return Result{}, err
		}

		agentsMissingSkillList = subtractMergedBoundAgents(agentsMissingSkillList, boundAgentArts)
	}

	artifacts := make([]Artifact, 0, 3+len(pluginArts)+len(skillArts)+len(agentArts))
	artifacts = append(artifacts, configArt, featureArt)

	for _, p := range pluginArts {
		artifacts = append(artifacts, p.Artifact)
	}

	for _, s := range skillArts {
		artifacts = append(artifacts, s.Artifact)
	}

	for _, a := range agentArts {
		artifacts = append(artifacts, a.Artifact)
	}

	for _, ba := range boundAgentArts {
		artifacts = append(artifacts, ba.Artifact)
	}

	if hasSnippet {
		artifacts = append(artifacts, snippetArt.Artifact)
	}

	configBody := artifact.ConfigFile()
	if req.WithAgents {
		configBody = artifact.ConfigFileWithRoles()
	}

	writeArts := make([]pluginArtifact, 0, len(pluginArts)+len(skillArts)+len(agentArts))
	writeArts = append(writeArts, pluginArts...)
	writeArts = append(writeArts, skillArts...)
	writeArts = append(writeArts, agentArts...)

	res := Result{
		Host:               req.Host,
		DryRun:             req.DryRun,
		Root:               root,
		Artifacts:          artifacts,
		Created:            []string{},
		Modified:           []string{},
		Removed:            []string{},
		RolesToAdd:         rolesToAdd(req.WithAgents, configArt.Action, cfg.Roles),
		NoHostDetected:     noHostDetected,
		DetectedBy:         detectedBy,
		Print:              printArtifacts(artifacts, configBody, writeArts, boundAgentArts, snippetArt),
		AgentsMissingSkill: agentsMissingSkillList,
	}

	if req.DryRun || req.Print {
		return res, nil
	}

	if err := s.writableCheck(writableTargets(featureArt, writeArts, boundAgentArts, snippetArt, hasSnippet, configArt)); err != nil {
		return res, err
	}

	return apply(fsys, res, featureArt, writeArts, boundAgentArts, snippetArt, hasSnippet, configArt, configBody)
}

// rolesToAdd renders Result.RolesToAdd: empty unless withAgents and the
// config was not created this run, else a "roles:" header plus one line
// per role current leaves unbound.
func rolesToAdd(withAgents bool, configAction Action, current config.RoleBindings) []string {
	if !withAgents || configAction == ActionCreated {
		return []string{}
	}

	want := artifact.AgentBindings()

	var lines []string

	for _, r := range []struct{ current, bound, key string }{
		{current.Planner, want.Planner, "planner"},
		{current.Implementer, want.Implementer, "implementer"},
		{current.Reviewer, want.Reviewer, "reviewer"},
	} {
		if r.current == "" {
			lines = append(lines, fmt.Sprintf("  %s: %s", r.key, r.bound))
		}
	}

	if len(lines) == 0 {
		return []string{}
	}

	return append([]string{"roles:"}, lines...)
}

// apply writes featureArt, then pluginArts, boundAgentArts, snippetArt and
// configArt in that order, verifying each rewrite against a concurrent
// edit immediately before overwriting it. A write failure after an earlier
// write already landed in this call is wrapped in ErrPartialWrite.
func apply(
	fsys rwfs.FS, res Result, featureArt Artifact, pluginArts []pluginArtifact, boundAgentArts []boundAgentArtifact,
	snippetArt snippetArtifact, hasSnippet bool, configArt Artifact, configBody []byte,
) (Result, error) {
	var wroteSomething bool

	if featureArt.Action == ActionCreated {
		if err := fsys.MkdirAll(fsName(featureArt.Path), 0o755); err != nil {
			return Result{}, fmt.Errorf("setup: create %s: %w", featureArt.Path, err)
		}

		res.Created = append(res.Created, featureArt.Path)
		wroteSomething = true
	}

	for _, p := range pluginArts {
		if p.Action != ActionCreated && p.Action != ActionMerged {
			continue
		}

		if p.Action == ActionMerged {
			if err := verifyFileUnchanged(fsys, p.Path, true, p.existing, "brief init"); err != nil {
				if wroteSomething {
					return res, markPartial(err)
				}

				return Result{}, err
			}
		}

		if err := writePluginFile(fsys, p.Path, artifact.Render(p.renderKind)); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if p.Action == ActionCreated {
			res.Created = append(res.Created, p.Path)
		} else {
			res.Modified = append(res.Modified, p.Path)
		}

		wroteSomething = true
	}

	for _, ba := range boundAgentArts {
		if ba.Action != ActionMerged {
			continue
		}

		if err := verifyBoundAgentUnchanged(ba, "brief init --edit-agents"); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if err := ba.agentFile().write(ba.edited, ba.perm); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		res.Modified = append(res.Modified, ba.Path)
		wroteSomething = true
	}

	if hasSnippet && (snippetArt.Action == ActionCreated || snippetArt.Action == ActionMerged) {
		existedBefore := snippetArt.Action == ActionMerged

		if err := verifyFileUnchanged(fsys, snippetArt.Path, existedBefore, snippetArt.existing, "brief init"); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		body := mergeSnippet(snippetArt.existing, snippetArt.span, artifact.SnippetBlock(snippetArt.dir))

		if err := writeSnippetFile(fsys, snippetArt.Path, body); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if snippetArt.Action == ActionCreated {
			res.Created = append(res.Created, snippetArt.Path)
		} else {
			res.Modified = append(res.Modified, snippetArt.Path)
		}

		wroteSomething = true
	}

	if configArt.Action == ActionCreated {
		if err := writeConfigFile(fsys, configArt.Path, configBody); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		res.Created = append(res.Created, configArt.Path)
	}

	return res, nil
}

// pluginArtifact pairs one plugin file's Artifact with its artifact.Kind
// and, for an ActionMerged row, the bytes planning read from disk so apply
// can guard the rewrite against a concurrent edit.
type pluginArtifact struct {
	Artifact

	renderKind artifact.Kind
	existing   []byte
}

// planPluginFiles plans every file h.Plugin(withHook) lists, each joined
// under root, in that same order.
func planPluginFiles(fsys rwfs.FS, root string, h host.Host, withHook bool) ([]pluginArtifact, error) {
	files := h.Plugin(withHook)
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		kind := KindPlugin
		if f.Hook {
			kind = KindHook
		}

		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(fsys, path, kind, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planAgentFiles plans every file h.Agents() lists, each joined under
// root, tagged KindAgent.
func planAgentFiles(fsys rwfs.FS, root string, h host.Host) ([]pluginArtifact, error) {
	files := h.Agents()
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(fsys, path, KindAgent, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planSkillFiles plans every file h.Skills() lists, each joined under
// root, tagged KindSkill.
func planSkillFiles(fsys rwfs.FS, root string, h host.Host) ([]pluginArtifact, error) {
	files := h.Skills()
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(fsys, path, KindSkill, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planPluginFile decides one plugin file's Artifact by comparing its disk
// bytes against renderKind's known renders, returning the bytes read too
// for an ActionMerged row so apply can guard the rewrite.
func planPluginFile(fsys rwfs.FS, path string, kind Kind, renderKind artifact.Kind) (Artifact, []byte, error) {
	info, err := fsys.Lstat(fsName(path))

	switch {
	// An ENOTDIR ancestor is treated as "not there yet"; checkWritable refuses it later.
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return Artifact{Kind: kind, Path: path, Action: ActionCreated}, nil, nil
	case err != nil:
		return Artifact{}, nil, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "not a regular file"}, nil, nil
	}

	body, err := fsys.ReadFile(fsName(path))
	if err != nil {
		return Artifact{}, nil, fmt.Errorf("setup: read %s: %w", path, err)
	}

	switch artifact.Recognize(renderKind, body) {
	case artifact.OriginCurrent:
		return Artifact{Kind: kind, Path: path, Action: ActionUnchanged}, nil, nil
	case artifact.OriginOlder:
		return Artifact{Kind: kind, Path: path, Action: ActionMerged, Detail: "updated"}, body, nil
	case artifact.OriginEdited:
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "edited locally"}, nil, nil
	default:
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "edited locally"}, nil, nil
	}
}

// writePluginFile creates path's parent directories (0o755) and then
// atomically writes body to path (0o644), through fsys's own WriteFile
// (diskFS's own body: internal/platform/atomicfile, confined only to
// path's immediate parent directory — see fs.go — so a reader never
// observes a truncated or half-renamed plugin file).
func writePluginFile(fsys rwfs.FS, path string, body []byte) error {
	dir := filepath.Dir(path)

	if err := fsys.MkdirAll(fsName(dir), 0o755); err != nil {
		return fmt.Errorf("setup: create %s: %w", dir, err)
	}

	return writeThrough(fsys, fsName(path), path, body)
}

// planConfig decides the config file's Artifact and, when it can be
// trusted, the Config governing the feature root and Result.RolesToAdd.
func planConfig(fsys rwfs.FS, nearest, root string, force, withAgents bool) (Artifact, config.Config, error) {
	if nearest == "" {
		path := filepath.Join(root, configFileName)

		cfg := config.Default()
		if withAgents {
			cfg.Roles = artifact.AgentBindings()
		}

		return Artifact{Kind: KindConfig, Path: path, Action: ActionCreated}, cfg, nil
	}

	desired := artifact.ConfigFile()
	if withAgents {
		desired = artifact.ConfigFileWithRoles()
	}

	// --force never refuses on the old file's content; an unreadable path
	// surfaces at the write attempt, not here.
	if force {
		if configFileCurrent(fsys, nearest, desired) {
			cfg, err := decodeCurrentConfig(fsys, nearest)
			if err != nil {
				return Artifact{}, config.Config{}, err
			}

			return Artifact{Kind: KindConfig, Path: nearest, Action: ActionUnchanged}, cfg, nil
		}

		cfg := config.Default()
		if withAgents {
			cfg.Roles = artifact.AgentBindings()
		}

		return Artifact{Kind: KindConfig, Path: nearest, Action: ActionCreated, Detail: "rewritten from defaults"}, cfg, nil
	}

	existing, err := fsys.ReadFile(fsName(nearest))
	if err != nil {
		return Artifact{}, config.Config{}, fmt.Errorf("setup: read %s: %w", nearest, err)
	}

	if artifact.Recognize(artifact.KindConfig, existing) == artifact.OriginCurrent {
		cfg, decodeErr := decodeCurrentConfig(fsys, nearest)
		if decodeErr != nil {
			return Artifact{}, config.Config{}, decodeErr
		}

		return Artifact{Kind: KindConfig, Path: nearest, Action: ActionUnchanged}, cfg, nil
	}

	cfg, violations, inspectErr := inspectConfig(fsys, nearest)
	if inspectErr != nil {
		return Artifact{}, config.Config{}, configRefusal(nearest, inspectErr)
	}

	if len(violations) > 0 {
		return Artifact{}, config.Config{}, configRefusal(nearest, &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return Artifact{Kind: KindConfig, Path: nearest, Action: ActionKept, Detail: "edited locally"}, cfg, nil
}

// configFileCurrent reports whether path's bytes exactly equal desired,
// false on any read failure. Unlike artifact.Recognize, this requires an
// exact match — under --force --with-agents a plain render is not current.
func configFileCurrent(fsys rwfs.FS, path string, desired []byte) bool {
	existing, err := fsys.ReadFile(fsName(path))
	if err != nil {
		return false
	}

	return bytes.Equal(existing, desired)
}

// decodeCurrentConfig decodes nearest, already known to hold a current
// KindConfig render, into its Config.
func decodeCurrentConfig(fsys rwfs.FS, nearest string) (config.Config, error) {
	cfg, violations, err := inspectConfig(fsys, nearest)
	if err != nil {
		return config.Config{}, configRefusal(nearest, err)
	}

	if len(violations) > 0 {
		return config.Config{}, configRefusal(nearest, &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return cfg, nil
}

// configRefusal builds the *RefusalError an invalid or unparseable
// existing config reports, Fix always pointing at --force.
func configRefusal(path string, err error) error {
	problem := err.Error()
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
		problem = flattenOneLine(invalidCfg.Err.Error())
	}

	return &RefusalError{
		Path:    path,
		Problem: problem,
		Fix:     "fix it, or run 'brief init --force' to rewrite it from defaults",
		Err:     err,
	}
}

// planFeatureRoot decides the feature root's own Artifact: ActionUnchanged
// when path already exists as a directory, ActionCreated when nothing
// exists there yet, or a *RefusalError wrapping ErrNotADirectory when path
// exists as something else.
func planFeatureRoot(fsys rwfs.FS, path string) (Artifact, error) {
	info, err := fsys.Stat(fsName(path))

	switch {
	case err == nil && info.IsDir():
		return Artifact{Kind: KindFeatureRoot, Path: path, Action: ActionUnchanged}, nil
	case err == nil:
		return Artifact{}, &RefusalError{
			Path:    path,
			Problem: "exists and is not a directory",
			Fix:     "remove it, or set feature-directory in .brief.yaml to a different path",
			Err:     ErrNotADirectory,
		}
	case errors.Is(err, fs.ErrNotExist):
		return Artifact{Kind: KindFeatureRoot, Path: path, Action: ActionCreated}, nil
	default:
		return Artifact{}, fmt.Errorf("setup: stat %s: %w", path, err)
	}
}

// writeConfigFile atomically replaces name with body.
func writeConfigFile(fsys rwfs.FS, name string, body []byte) error {
	return writeThrough(fsys, fsName(name), name, body)
}

// validHost reports whether host appears in Hosts().
func validHost(host string) bool {
	return slices.Contains(Hosts(), host)
}

// flattenOneLine collapses s to a single line, since a RefusalError's
// Problem must be one line.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
