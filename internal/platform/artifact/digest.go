package artifact

import (
	"crypto/sha256"
	"slices"
)

// Kind names one of the file shapes this package renders: the ".brief.yaml"
// config, a Claude Code plugin's manifest, its two skill files and its
// hook wiring — a snippet and agent files each add their own Kind and
// digest list as init grows to write them.
type Kind string

const (
	// KindConfig is ConfigFile's own Kind: the digest list Recognize checks
	// when kind is KindConfig holds every release's own ConfigFile digest.
	KindConfig Kind = "config"
	// KindPluginManifest is PluginManifest's own Kind.
	KindPluginManifest Kind = "plugin-manifest"
	// KindSkillStart is SkillStart's own Kind.
	KindSkillStart Kind = "skill-start"
	// KindSkillFinish is SkillFinish's own Kind.
	KindSkillFinish Kind = "skill-finish"
	// KindClaudeHooks is ClaudeHooks's own Kind.
	KindClaudeHooks Kind = "claude-hooks"
	// KindAgentPlanner is AgentPlanner's own Kind.
	KindAgentPlanner Kind = "agent-planner"
	// KindAgentImplementer is AgentImplementer's own Kind.
	KindAgentImplementer Kind = "agent-implementer"
	// KindAgentReviewer is AgentReviewer's own Kind.
	KindAgentReviewer Kind = "agent-reviewer"
	// KindSnippet is SnippetBlock's own Kind. Unlike every other Kind, it is
	// never passed to Render or Recognize: SnippetBlock takes a feature
	// directory Render's own signature carries no room for, and Recognize's
	// single compiled-in digest list has no way to check "current for which
	// directory". SnippetBlock and RecognizeSnippet are the snippet's own
	// render and recognize functions instead — internal/setup calls them
	// directly, never Render(KindSnippet) or Recognize(KindSnippet, ...).
	KindSnippet Kind = "snippet"
)

// Origin classifies an existing file's bytes against a Kind's compiled-in
// digest list: OriginCurrent for a byte-for-byte match against today's own
// render, OriginEdited for anything else. A future release's digest list
// grows an OriginOlder arm once a second digest exists to distinguish "an
// earlier release wrote this, unedited" from "a person edited this" —
// today every digest list holds only the current render, so that
// distinction has nothing to select between yet.
type Origin string

const (
	// OriginCurrent marks bytes equal to this binary's own current render.
	OriginCurrent Origin = "current"
	// OriginEdited marks bytes that match no compiled-in digest for the
	// given Kind.
	OriginEdited Origin = "edited"
)

// configDigests holds the sha256 digest of every release's own ConfigFile
// render, plus ConfigFileWithRoles' own — a second render body Recognize
// treats as OriginCurrent for KindConfig, written only by "init
// --with-agents" when it creates a fresh config, so both an unbound and a
// role-bound config a caller ships are recognized rather than reported
// "edited locally". Render(KindConfig) still returns ConfigFile alone —
// this list, not Render, is what makes ConfigFileWithRoles recognized.
var configDigests = [][32]byte{
	sha256.Sum256(ConfigFile()),
	sha256.Sum256(ConfigFileWithRoles()),
}

// pluginManifestDigests holds the sha256 digest of every release's own
// PluginManifest render.
var pluginManifestDigests = [][32]byte{
	sha256.Sum256(PluginManifest()),
}

// skillStartDigests holds the sha256 digest of every release's own
// SkillStart render.
var skillStartDigests = [][32]byte{
	sha256.Sum256(SkillStart()),
}

// skillFinishDigests holds the sha256 digest of every release's own
// SkillFinish render.
var skillFinishDigests = [][32]byte{
	sha256.Sum256(SkillFinish()),
}

// claudeHooksDigests holds the sha256 digest of every release's own
// ClaudeHooks render.
var claudeHooksDigests = [][32]byte{
	sha256.Sum256(ClaudeHooks()),
}

// agentPlannerDigests holds the sha256 digest of every release's own
// AgentPlanner render.
var agentPlannerDigests = [][32]byte{
	sha256.Sum256(AgentPlanner()),
}

// agentImplementerDigests holds the sha256 digest of every release's own
// AgentImplementer render.
var agentImplementerDigests = [][32]byte{
	sha256.Sum256(AgentImplementer()),
}

// agentReviewerDigests holds the sha256 digest of every release's own
// AgentReviewer render.
var agentReviewerDigests = [][32]byte{
	sha256.Sum256(AgentReviewer()),
}

// Recognize reports body's Origin against kind's own compiled-in digest
// list: OriginCurrent when body's sha256 digest matches this binary's own
// current render for kind, OriginEdited otherwise. An unrecognized Kind
// reports OriginEdited, since there is no digest list to match against —
// including a Kind's bytes checked against a different Kind's own list,
// which never matches.
func Recognize(kind Kind, body []byte) Origin {
	sum := sha256.Sum256(body)

	if slices.Contains(digestsFor(kind), sum) {
		return OriginCurrent
	}

	return OriginEdited
}

// digestsFor returns kind's own compiled-in digest list, nil for a Kind
// this package does not render.
func digestsFor(kind Kind) [][32]byte {
	switch kind {
	case KindConfig:
		return configDigests
	case KindPluginManifest:
		return pluginManifestDigests
	case KindSkillStart:
		return skillStartDigests
	case KindSkillFinish:
		return skillFinishDigests
	case KindClaudeHooks:
		return claudeHooksDigests
	case KindAgentPlanner:
		return agentPlannerDigests
	case KindAgentImplementer:
		return agentImplementerDigests
	case KindAgentReviewer:
		return agentReviewerDigests
	case KindSnippet:
		// Deliberately excluded: SnippetBlock/RecognizeSnippet are the
		// snippet's own render and recognize functions (see doc.go).
		return nil
	default:
		return nil
	}
}
