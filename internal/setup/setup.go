package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/host"
)

// configFileName is the config file Init writes and looks for — the same
// name internal/platform/config resolves.
const configFileName = ".brief.yaml"

// HostNone is one host InitRequest.Host accepts: no agent-host integration
// is installed, only the config and the feature root.
const HostNone = "none"

// HostClaudeCode is the other host InitRequest.Host accepts: a Claude Code
// skills-directory plugin (host.ClaudeCode) is installed alongside the
// config and the feature root.
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
	// KindAgent is one of a host's three role-agent files, installed only
	// under InitRequest.WithAgents; Uninstall always plans their removal
	// regardless of any flag Init was run with.
	KindAgent Kind = "agent"
	// KindSkill is the brief-workflow skill file (host.Host.Skills),
	// installed on every claude-code install, with or without WithAgents —
	// unlike KindAgent, it lives outside host.PluginDir.
	KindSkill Kind = "skill"
	// KindSnippet is the CLAUDE.md instruction block (R5).
	KindSnippet Kind = "snippet"
	// KindBoundAgent is a repository agent file InitRequest.EditAgents
	// edited, or found already satisfying or unable to satisfy, Rule 4's
	// own "skills:" edit — a bare-name planner or implementer binding's own
	// ScopeProject agentfile.Definition, never a "brief:*" binding, another
	// plugin's, or one under "~/.claude".
	KindBoundAgent Kind = "bound-agent"
)

// Action names what Init did, or would do, to one Artifact.
type Action string

const (
	// ActionCreated marks an artifact that did not exist and was written,
	// or, for the config file under --force, one rewritten from defaults.
	ActionCreated Action = "created"
	// ActionUnchanged marks an artifact whose bytes, or whose existence as
	// a directory, already matched what Init would have written.
	ActionUnchanged Action = "unchanged"
	// ActionKept marks an existing, valid config file left as it was
	// because its bytes differ from Init's own render — a repository
	// owner's local edit — or, for Uninstall, any artifact left on disk
	// rather than removed.
	ActionKept Action = "kept"
	// ActionRemoved marks an artifact Uninstall deleted.
	ActionRemoved Action = "removed"
	// ActionMerged marks either of two rewrites in place: the CLAUDE.md
	// instruction block appended to an existing file that carried none, or
	// replaced because its bytes were a brief-written render other than
	// today's own (an older release, or the same release rendered for a
	// different feature directory); or a whole plugin, skill or agent file
	// whose bytes are artifact.OriginOlder — Rule 6 — replaced wholesale
	// with today's own render, detail "updated".
	ActionMerged Action = "merged"
)

// Server plans and applies brief's own install write path. homeDir backs
// detectHost's own home-directory check (WithHomeDir); every other
// dependency is read from the real filesystem directly.
type Server struct {
	homeDir func() (string, error)
}

// Option configures a Server built by NewServer.
type Option func(*Server)

// NewServer returns a Server ready to call Init on, homeDir defaulted to
// os.UserHomeDir.
func NewServer(opts ...Option) *Server {
	s := &Server{homeDir: os.UserHomeDir}

	for _, o := range opts {
		o(s)
	}

	return s
}

// InitRequest is Init's own input: Host selects the agent-host integration
// (Hosts); "" means detect (R8, detectHost) rather than refuse —
// Result.NoHostDetected reports whether detection found nothing, the only
// way a caller distinguishes that from an explicit Host: HostNone. NoHook
// omits a claude-code host's hook wiring file entirely (no
// plan, no row) while leaving any other plugin file untouched, WithAgents
// installs the three role-agent files (host.Host.Agents) and, only when
// this run also creates the config file, binds every role to them (R7) —
// like NoHook, false plans no agent row at all, never reading or writing
// them; it is refused as ErrAgentsNeedHost unless Host is
// HostClaudeCode. EditAgents plans and, for a real run, applies
// planBoundAgents (Rule 3, Rule 4): every bare-name planner or implementer
// binding's own ScopeProject agentfile.Definition gets a KindBoundAgent
// row and, when its own "skills:" shape allows it, its frontmatter edited
// to add artifact.WorkflowSkillName — it is refused as
// ErrEditAgentsNeedHost, checked after ErrAgentsNeedHost, unless Host is
// HostClaudeCode. DryRun computes the same plan without writing anything,
// and Force rewrites an existing config from defaults — the bound variant
// under WithAgents — rather than keeping or refusing it; it never rewrites
// an edited plugin or agent file, only the config. Print computes the same
// plan and writes nothing either, like DryRun, but additionally skips the
// writability pre-check (checkWritable): Result.Print is populated either
// way, but only a real run — neither DryRun nor Print — ever runs the
// check or writes anything.
type InitRequest struct {
	Host       string
	NoHook     bool
	WithAgents bool
	EditAgents bool
	DryRun     bool
	Force      bool
	Print      bool
}

// Artifact is one thing Init installs, or Init or Uninstall found already
// installed: Kind and Path identify it, Action reports what happened, and
// Detail carries an optional parenthetical ("edited locally", "rewritten
// from defaults"), empty when there is nothing to add. ForceRemovable is
// true exactly on an Uninstall-side ActionKept artifact that --force would
// turn into ActionRemoved (an edited file — planPluginRemoval,
// planConfigRemoval, planSnippetRemoval's own OriginEdited/OriginOlder
// arm) — false on every other Action, including that same edited file once
// --force has already removed it, and on a non-regular file (never
// followed or removed regardless of --force). It is unused, always false,
// on every Init-side artifact. This is the typed distinction cli's own
// uninstallNextAction counts on, rather than matching Detail's own free
// text: install-side and removal-side kept detail wording are free to
// differ (planSnippet's own longer "add the block by hand" versus
// planSnippetRemoval's plain "not a regular file") without silently
// breaking that count.
type Artifact struct {
	Kind           Kind
	Path           string
	Action         Action
	Detail         string
	ForceRemovable bool
}

// Result is what Init and Uninstall both return: Host and DryRun echo the
// request, Root is the absolute install root both operated against —
// config.LocateInRepo's directory, or wd when no config was found, or when
// the config found lies above the nearest enclosing git repository (R3) —
// Artifacts lists what was found and what happened to it — for Init, the
// config file, the feature root, then a claude-code host's own plugin
// manifest, start skill, finish skill, hook wiring, the brief-workflow
// skill, and, under WithAgents, the three role-agent files, then the
// CLAUDE.md block last, the fixed order R11's stdout rows render in; for
// Uninstall, the CLAUDE.md block first, then one KindBoundAgent row per
// planBoundAgentRemovals target (Rule 8, planned only when the
// brief-workflow skill's own row is not itself ActionKept), then a
// claude-code host's own agent files (reviewer, implementer, planner —
// always planned, independent of any flag Init was run with), the
// brief-workflow skill, and plugin files (hook, finish skill, start skill,
// manifest), then the config file last, so a partial uninstall never
// removes the repository's opt-in marker before everything else. Created
// names every path Init wrote that did not exist before; Modified names
// every path either command rewrote in place — Init's own CLAUDE.md merge
// or replace, an ActionMerged plugin, skill or agent file Init upgrades
// from an OriginOlder render (Rule 6), Uninstall's own CLAUDE.md block
// strip that leaves the file non-empty, and Uninstall's own bound-agent
// "skills:" edit (Rule 8) — a bound-agent row is ActionRemoved but never
// appears in Removed, since the file itself is rewritten, not deleted;
// Removed names
// every path Uninstall actually deleted — both absolute, in the order each
// command touched them. A pruned, now-empty plugin directory is never in
// any of the three. RolesToAdd is Init's own hint (R7): empty unless
// WithAgents and the config was not written this run, in which case it
// lists a "roles:" header line plus one "  <role>: brief:<role>" line for
// every role the kept or unchanged config still leaves unbound — a role
// already bound to anything, brief's own agent or the adopter's own, is
// never listed. No slice is ever nil; Created, Modified and Removed are
// empty under DryRun; RolesToAdd is populated even under DryRun.
// NoHostDetected is true only when InitRequest.Host was "" and detectHost
// found nothing — the one signal a caller needs to render R8's
// no-host-detected line instead of the ordinary next action; it is always
// false when Host was given explicitly, HostNone included. DetectedBy names
// the signal detectHost found — ".claude", "CLAUDE.md", or "~/.claude" —
// only when Host was "" and detection succeeded; it is "" both when Host
// was given explicitly and when NoHostDetected is true, so a caller can
// tell "the caller chose claude-code" from "brief guessed it, and here is
// why" without also checking NoHostDetected. Print is R9's own
// pending-artifact set (printArtifacts), never nil, populated regardless of
// DryRun or Print. AgentsMissingSkill is Init's own report (agentsMissingSkill):
// every bare-name planner or implementer binding, from this run's own
// post-plan config.Roles, whose resolved agent does not preload the
// brief-workflow skill — populated only when the resolved Host is
// HostClaudeCode, regardless of DryRun or Print, empty and non-nil
// otherwise, including on every Uninstall Result.
type Result struct {
	Host               string
	DryRun             bool
	Root               string
	Artifacts          []Artifact
	Created            []string
	Modified           []string
	Removed            []string
	RolesToAdd         []string
	NoHostDetected     bool
	DetectedBy         string
	Print              []PrintArtifact
	AgentsMissingSkill []MissingSkillAgent
}

// Init plans then, unless req.DryRun, applies brief's own install: the
// config file, the feature root the kept or freshly written config names,
// and, for req.Host == HostClaudeCode, that host's own skills-directory
// plugin files (host.Host.Plugin), the brief-workflow skill
// (host.Host.Skills) — written on every claude-code install, with or
// without req.WithAgents — under req.WithAgents its three role-agent files
// (host.Host.Agents, R7) — with a config this same call creates or
// --force-rewrites bound to them (artifact.AgentBindings) — and its
// CLAUDE.md instruction block (R5, planSnippet) — independent of
// req.NoHook, which only ever omits the hook file. Every refusal — an
// unknown host, req.WithAgents without a resolved HostClaudeCode
// (ErrAgentsNeedHost), an invalid existing config, a feature root that
// exists as something other than a directory, a CLAUDE.md marker defect —
// is decided during planning, before any artifact is touched; DryRun
// therefore returns exactly the plan a real run would apply, including any
// refusal, and writes nothing either way. Result.RolesToAdd is computed
// even under DryRun.
//
// The install root is config.LocateInRepo's own directory (R3): a found
// config is adopted only when it sits at or below the nearest enclosing git
// repository, walked from wd; one found above that boundary — a HOME-level
// config, say — is treated as though none existed, and Init writes a fresh
// one at wd instead of adopting or merging into a repository elsewhere on
// disk. With no enclosing git repository anywhere above wd, Init keeps its
// own plain ancestor walk, unbounded, exactly as before this rule existed.
//
// Applying writes the feature root, then every plugin, skill and agent
// file reporting ActionCreated or ActionMerged (Rule 6's OriginOlder
// upgrade, guarded by verifyFileUnchanged against a concurrent edit), then
// the CLAUDE.md block, then the config file last, so the config file — the
// repository's opt-in marker — never appears before everything else has
// landed. A plugin, skill or agent file already present and unedited
// (ActionUnchanged) or edited locally (ActionKept) is never rewritten,
// --force included: R3's --force only ever rewrites the config from
// defaults (the bound variant under req.WithAgents); the same holds for a
// CLAUDE.md block reporting ActionKept. A failure after at least one
// earlier write already landed is wrapped in ErrPartialWrite; a failure
// before anything was written is returned as-is.
func (s *Server) Init(_ context.Context, wd string, req InitRequest) (Result, error) {
	if req.Host != "" && !validHost(req.Host) {
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

	var (
		noHostDetected bool
		detectedBy     string
	)

	if req.Host == "" {
		var detected bool

		req.Host, detected, detectedBy = detectHost(root, s.homeDir)
		noHostDetected = !detected
	}

	if req.WithAgents && req.Host != HostClaudeCode {
		return Result{}, ErrAgentsNeedHost
	}

	if req.EditAgents && req.Host != HostClaudeCode {
		return Result{}, ErrEditAgentsNeedHost
	}

	configArt, cfg, err := planConfig(nearest, root, req.Force, req.WithAgents)
	if err != nil {
		return Result{}, err
	}

	featureRoot := filepath.Join(root, cfg.FeatureDirectory)

	featureArt, err := planFeatureRoot(featureRoot)
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

		pluginArts, err = planPluginFiles(root, h, !req.NoHook)
		if err != nil {
			return Result{}, err
		}

		skillArts, err = planSkillFiles(root, h)
		if err != nil {
			return Result{}, err
		}

		if req.WithAgents {
			agentArts, err = planAgentFiles(root, h)
			if err != nil {
				return Result{}, err
			}
		}

		snippetArt, err = planSnippet(root, h, cfg.FeatureDirectory)
		if err != nil {
			return Result{}, err
		}

		hasSnippet = true

		home, homeErr := s.homeDir()
		if homeErr != nil {
			home = ""
		}

		if req.EditAgents {
			boundAgentArts, err = planBoundAgents(root, home, cfg.Roles)
			if err != nil {
				return Result{}, err
			}
		}

		agentsMissingSkillList = agentsMissingSkill(root, home, cfg.Roles)
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

	if err := checkWritable(writableTargets(featureArt, writeArts, boundAgentArts, snippetArt, hasSnippet, configArt)); err != nil {
		return res, err
	}

	return apply(res, featureArt, writeArts, boundAgentArts, snippetArt, hasSnippet, configArt, configBody)
}

// rolesToAdd renders Result.RolesToAdd (R7): empty unless withAgents and
// the config was not written this run (configAction != ActionCreated —
// bindings are only ever written into a config Init creates in the same
// run), else a "roles:" header line plus one "  <role>: brief:<role>" line
// for every role current leaves unbound (empty), in RoleBindings' own
// field order (planner, implementer, reviewer) — a role already bound to
// anything, brief's own agent or the adopter's own, is never listed.
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

// apply writes featureArt, then every pluginArts entry reporting
// ActionCreated or ActionMerged (plugin files, then the brief-workflow
// skill, then, under WithAgents, the three agent files — all share this one
// list and its own write order) — an ActionMerged entry (an OriginOlder
// render Rule 6 upgrades) re-reads its own path immediately before writing
// (verifyFileUnchanged) and refuses ErrConcurrentEdit rather than
// overwriting a file changed since planning — then every boundAgentArts
// entry reporting ActionMerged (guarded by the same verifyFileUnchanged
// check, then writeBoundAgent, which preserves the file's own mode), then
// snippetArt (when hasSnippet, and it reports ActionCreated or
// ActionMerged), then configArt last with configBody as its bytes — the
// plain ConfigFile() or, under WithAgents, ConfigFileWithRoles() — into
// res's own Created or Modified list — Created for ActionCreated, Modified
// for ActionMerged, since a merge rewrites bytes an existing file already
// held. A write failure is wrapped in ErrPartialWrite iff at least one
// earlier write already landed in this same call.
func apply(
	res Result, featureArt Artifact, pluginArts []pluginArtifact, boundAgentArts []boundAgentArtifact,
	snippetArt snippetArtifact, hasSnippet bool, configArt Artifact, configBody []byte,
) (Result, error) {
	var wroteSomething bool

	if featureArt.Action == ActionCreated {
		if err := os.MkdirAll(featureArt.Path, 0o755); err != nil {
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
			if err := verifyFileUnchanged(p.Path, true, p.existing, "brief init"); err != nil {
				if wroteSomething {
					return res, markPartial(err)
				}

				return Result{}, err
			}
		}

		if err := writePluginFile(p.Path, artifact.Render(p.renderKind)); err != nil {
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

		if err := verifyFileUnchanged(ba.Path, true, ba.existing, "brief init --edit-agents"); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		if err := writeBoundAgent(ba.resolvedRoot, ba.rel, ba.edited, ba.perm); err != nil {
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

		if err := verifyFileUnchanged(snippetArt.Path, existedBefore, snippetArt.existing, "brief init"); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		body := mergeSnippet(snippetArt.existing, snippetArt.span, artifact.SnippetBlock(snippetArt.dir))

		if err := writeSnippetFile(snippetArt.Path, body); err != nil {
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
		if err := writeConfigFile(configArt.Path, configBody); err != nil {
			if wroteSomething {
				return res, markPartial(err)
			}

			return Result{}, err
		}

		res.Created = append(res.Created, configArt.Path)
	}

	return res, nil
}

// pluginArtifact pairs one plugin file's Artifact with the artifact.Kind
// its Render/Recognize digest list is checked against — a different
// vocabulary from Artifact.Kind's own setup-level Kind (KindPlugin or
// KindHook), kept out of Artifact itself since nothing outside this
// package ever needs it — and, for an ActionMerged row (an OriginOlder
// file being upgraded), the bytes planning actually read from disk:
// apply's own write-path guard (verifyFileUnchanged, reused) re-reads
// the path immediately before writing and refuses ErrConcurrentEdit unless
// it still matches existing, the same read-modify-write protection the
// CLAUDE.md merge already has. Nil for every Action other than
// ActionMerged.
type pluginArtifact struct {
	Artifact

	renderKind artifact.Kind
	existing   []byte
}

// planPluginFiles plans every file h.Plugin(withHook) lists, each joined
// under root, in that same order.
func planPluginFiles(root string, h host.Host, withHook bool) ([]pluginArtifact, error) {
	files := h.Plugin(withHook)
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		kind := KindPlugin
		if f.Hook {
			kind = KindHook
		}

		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(path, kind, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planAgentFiles plans every file h.Agents() lists, each joined under
// root, in that same order (planner, implementer, reviewer) — mirroring
// planPluginFiles, but every entry is tagged KindAgent rather than
// KindPlugin or KindHook: Agents carries no hook file of its own.
func planAgentFiles(root string, h host.Host) ([]pluginArtifact, error) {
	files := h.Agents()
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(path, KindAgent, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planSkillFiles plans every file h.Skills() lists, each joined under
// root, in that same order — mirroring planPluginFiles and planAgentFiles,
// but every entry is tagged KindSkill: unlike Agents, Skills is planned on
// every claude-code install, with or without WithAgents.
func planSkillFiles(root string, h host.Host) ([]pluginArtifact, error) {
	files := h.Skills()
	out := make([]pluginArtifact, 0, len(files))

	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.RelPath))

		art, existing, err := planPluginFile(path, KindSkill, f.Kind)
		if err != nil {
			return nil, err
		}

		out = append(out, pluginArtifact{Artifact: art, renderKind: f.Kind, existing: existing})
	}

	return out, nil
}

// planPluginFile decides one plugin file's own Artifact and, for an
// OriginOlder file, the bytes planning read (returned separately so
// planPluginFiles can carry them on pluginArtifact.existing) — mirroring
// planConfigRemoval's own Lstat-first shape: missing reports ActionCreated;
// a path that exists but is not a regular file (a directory, a symlink)
// reports ActionKept, detail "not a regular file", never followed; a
// regular file whose bytes are artifact.Recognize's OriginCurrent for
// renderKind reports ActionUnchanged; OriginOlder — an earlier release's
// own render, Rule 6 — reports ActionMerged, detail "updated", the file's
// own bytes returned alongside so apply can guard the rewrite against a
// concurrent edit; any other bytes (OriginEdited) report ActionKept, detail
// "edited locally" — Init never rewrites an edited plugin file the way
// --force rewrites the config. An ENOTDIR Lstat — an ancestor component
// exists as something other than a directory — is treated the same as
// "does not exist yet": os.IsNotExist never matches it, but the path still
// is not there, and the pre-write check (checkWritable) is what refuses on
// that blocking ancestor, not planning.
func planPluginFile(path string, kind Kind, renderKind artifact.Kind) (Artifact, []byte, error) {
	info, err := os.Lstat(path)

	switch {
	case os.IsNotExist(err), errors.Is(err, syscall.ENOTDIR):
		return Artifact{Kind: kind, Path: path, Action: ActionCreated}, nil, nil
	case err != nil:
		return Artifact{}, nil, fmt.Errorf("setup: lstat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return Artifact{Kind: kind, Path: path, Action: ActionKept, Detail: "not a regular file"}, nil, nil
	}

	body, err := os.ReadFile(path)
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
// atomically writes body to path (0o644), through
// internal/platform/atomicfile so a reader never observes a truncated or
// half-renamed plugin file.
func writePluginFile(path string, body []byte) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("setup: create %s: %w", dir, err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	w, err := atomicfile.Create(root, filepath.Base(path), 0o644)
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	if _, err := w.Write(body); err != nil {
		_ = w.Close()

		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	return nil
}

// planConfig decides the config file's own Artifact and, when it can be
// trusted, the Config governing the feature root and Result.RolesToAdd: a
// fresh path (nothing found) always reports ActionCreated against
// config.Default() (Roles bound to artifact.AgentBindings() under
// withAgents — the config this call reports ActionCreated for is always
// the one apply writes). Under --force, the target is the desired render
// alone — ConfigFile(), or ConfigFileWithRoles() under withAgents — and
// only an exact byte match against it (configFileCurrent) is
// ActionUnchanged; any other content, invalid, unparseable, or unreadable
// for any other reason (a directory at that path, say), reports
// ActionCreated, detail "rewritten from defaults" — never refusing on the
// old file's content, since --force's own promise is to rewrite it; the
// write attempt itself, not this planning step, is where an unreadable
// path's own failure surfaces. Without --force, an existing file
// recognized as any current KindConfig render (artifact.Recognize's
// OriginCurrent — the plain render or the bound one, either) is
// ActionUnchanged, its own bytes decoded (decodeCurrentConfig) so a bound
// file's own Roles are never reported unbound; otherwise it is decoded
// with config.Inspect — a decode failure or any R1 violation refuses,
// naming the first one in Config's own field order — and a config with
// none is ActionKept, detail "edited locally", governing the feature root
// and RolesToAdd with its own decoded values. --with-agents never edits an
// existing config either way (R7): only the nearest == "" branch above,
// and --force's own rewrite, ever report ActionCreated.
func planConfig(nearest, root string, force, withAgents bool) (Artifact, config.Config, error) {
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

	if force {
		if configFileCurrent(nearest, desired) {
			cfg, err := decodeCurrentConfig(nearest)
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

	existing, err := os.ReadFile(nearest)
	if err != nil {
		return Artifact{}, config.Config{}, fmt.Errorf("setup: read %s: %w", nearest, err)
	}

	if artifact.Recognize(artifact.KindConfig, existing) == artifact.OriginCurrent {
		cfg, decodeErr := decodeCurrentConfig(nearest)
		if decodeErr != nil {
			return Artifact{}, config.Config{}, decodeErr
		}

		return Artifact{Kind: KindConfig, Path: nearest, Action: ActionUnchanged}, cfg, nil
	}

	cfg, violations, inspectErr := config.Inspect(nearest)
	if inspectErr != nil {
		return Artifact{}, config.Config{}, configRefusal(nearest, inspectErr)
	}

	if len(violations) > 0 {
		return Artifact{}, config.Config{}, configRefusal(nearest, &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return Artifact{Kind: KindConfig, Path: nearest, Action: ActionKept, Detail: "edited locally"}, cfg, nil
}

// configFileCurrent reports whether path's own bytes already equal
// desired exactly, false for any read failure — a directory at path, a
// permission error, or a genuinely different render all take the same
// "not current" branch here. Unlike the non-force branch's own
// artifact.Recognize check, this is an exact match against desired alone,
// never "any known current render": under --force --with-agents a config
// holding the plain render is not current — it must be rewritten to the
// bound one — even though artifact.Recognize would call it OriginCurrent.
func configFileCurrent(path string, desired []byte) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return bytes.Equal(existing, desired)
}

// decodeCurrentConfig decodes nearest — already known to hold one of this
// package's own compiled-in KindConfig renders (artifact.Recognize's
// OriginCurrent) — into its Config. A decode failure or R1 violation here
// would mean a compiled-in render itself stopped decoding cleanly, not a
// condition a caller can fix; it is still reported as a *RefusalError
// (configRefusal) rather than panicking, so a defect here fails loudly
// instead of silently reporting every role unbound.
func decodeCurrentConfig(nearest string) (config.Config, error) {
	cfg, violations, err := config.Inspect(nearest)
	if err != nil {
		return config.Config{}, configRefusal(nearest, err)
	}

	if len(violations) > 0 {
		return config.Config{}, configRefusal(nearest, &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return cfg, nil
}

// configRefusal builds the *RefusalError an invalid or unparseable existing
// config reports: Problem recovers the underlying *config.ValueError or
// decode failure's own text via errors.AsType[*config.InvalidConfigError],
// and Fix always points at --force, the only way init rewrites an existing
// file. err is wrapped unchanged, so errors.Is(result, config.ErrInvalidConfig)
// still holds.
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
func planFeatureRoot(path string) (Artifact, error) {
	info, err := os.Stat(path)

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
	case os.IsNotExist(err):
		return Artifact{Kind: KindFeatureRoot, Path: path, Action: ActionCreated}, nil
	default:
		return Artifact{}, fmt.Errorf("setup: stat %s: %w", path, err)
	}
}

// writeConfigFile atomically replaces name — an absolute path — with body,
// through internal/platform/atomicfile so a reader never observes a
// truncated or half-renamed config file.
func writeConfigFile(name string, body []byte) error {
	dir := filepath.Dir(name)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	w, err := atomicfile.Create(root, filepath.Base(name), 0o644)
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	if _, err := w.Write(body); err != nil {
		_ = w.Close()

		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	return nil
}

// validHost reports whether host appears in Hosts().
func validHost(host string) bool {
	return slices.Contains(Hosts(), host)
}

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each — yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and a RefusalError's
// Problem is always one line.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
