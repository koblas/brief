package artifact

import (
	"crypto/sha256"
	"slices"
)

// Kind names one of the file shapes this package renders: the ".brief.yaml"
// config, a Claude Code plugin's manifest, its two skill files and its hook
// wiring, three role-agent files, the brief-workflow skill installed
// outside the plugin (SkillWorkflow), and a snippet.
type Kind string

const (
	// KindConfig is ConfigFile's Kind.
	KindConfig Kind = "config"
	// KindPluginManifest is PluginManifest's Kind.
	KindPluginManifest Kind = "plugin-manifest"
	// KindSkillStart is SkillStart's Kind.
	KindSkillStart Kind = "skill-start"
	// KindSkillFinish is SkillFinish's Kind.
	KindSkillFinish Kind = "skill-finish"
	// KindClaudeHooks is ClaudeHooks's Kind.
	KindClaudeHooks Kind = "claude-hooks"
	// KindAgentPlanner is AgentPlanner's Kind.
	KindAgentPlanner Kind = "agent-planner"
	// KindAgentImplementer is AgentImplementer's Kind.
	KindAgentImplementer Kind = "agent-implementer"
	// KindAgentReviewer is AgentReviewer's Kind.
	KindAgentReviewer Kind = "agent-reviewer"
	// KindSkillWorkflow is SkillWorkflow's Kind.
	KindSkillWorkflow Kind = "skill-workflow"
	// KindSnippet is SnippetBlock's Kind. Unlike every other Kind it is
	// never passed to Render or Recognize, since SnippetBlock takes a
	// feature directory and Recognize's digest list has no per-directory
	// notion of current; SnippetBlock and RecognizeSnippet stand in for it.
	KindSnippet Kind = "snippet"
)

// Origin classifies an existing file's bytes against a Kind's compiled-in
// digest lists: OriginCurrent for a match against today's render (or, for
// KindConfig, any current render this package ships), OriginOlder for a
// match against an earlier release's render, OriginEdited otherwise.
type Origin string

const (
	// OriginCurrent marks bytes equal to one of this binary's current
	// renders for the given Kind.
	OriginCurrent Origin = "current"
	// OriginOlder marks bytes equal to an earlier release's render for the
	// given Kind, never today's.
	OriginOlder Origin = "older"
	// OriginEdited marks bytes matching no compiled-in digest for the given Kind.
	OriginEdited Origin = "edited"
)

// configDigests holds this release's ConfigFile and ConfigFileWithRoles
// digests, so both an unbound and a role-bound config are OriginCurrent
// for KindConfig; Render(KindConfig) still returns ConfigFile alone.
var configDigests = [][32]byte{
	sha256.Sum256(ConfigFile()),
	sha256.Sum256(ConfigFileWithRoles()),
}

// olderConfigDigests holds each earlier release's ConfigFile or
// ConfigFileWithRoles digest; empty until a release changes either.
var olderConfigDigests = [][32]byte{}

// pluginManifestDigests holds this release's PluginManifest digest.
var pluginManifestDigests = [][32]byte{
	sha256.Sum256(PluginManifest()),
}

// olderPluginManifestDigests holds each earlier release's PluginManifest
// digest; empty until a release changes it.
var olderPluginManifestDigests = [][32]byte{}

// skillStartDigests holds this release's SkillStart digest.
var skillStartDigests = [][32]byte{
	sha256.Sum256(SkillStart()),
}

// olderSkillStartDigests holds each earlier release's SkillStart digest;
// empty until a release changes it.
var olderSkillStartDigests = [][32]byte{}

// skillFinishDigests holds this release's SkillFinish digest.
var skillFinishDigests = [][32]byte{
	sha256.Sum256(SkillFinish()),
}

// olderSkillFinishDigests holds each earlier release's SkillFinish digest;
// empty until a release changes it.
var olderSkillFinishDigests = [][32]byte{}

// claudeHooksDigests holds this release's ClaudeHooks digest.
var claudeHooksDigests = [][32]byte{
	sha256.Sum256(ClaudeHooks()),
}

// olderClaudeHooksDigests holds each earlier release's ClaudeHooks digest;
// empty until a release changes it.
var olderClaudeHooksDigests = [][32]byte{}

// agentPlannerDigests holds this release's AgentPlanner digest.
var agentPlannerDigests = [][32]byte{
	sha256.Sum256(AgentPlanner()),
}

// olderAgentPlannerDigests holds each earlier release's AgentPlanner
// digest, read from a fixed file under files/older — never a live render,
// which would make the OriginOlder arm unreachable.
var olderAgentPlannerDigests = [][32]byte{
	sha256.Sum256(mustReadFile("older/agents/planner.md")),
}

// agentImplementerDigests holds this release's AgentImplementer digest.
var agentImplementerDigests = [][32]byte{
	sha256.Sum256(AgentImplementer()),
}

// olderAgentImplementerDigests holds each earlier release's
// AgentImplementer digest, read from a fixed file under files/older.
var olderAgentImplementerDigests = [][32]byte{
	sha256.Sum256(mustReadFile("older/agents/implementer.md")),
}

// agentReviewerDigests holds this release's AgentReviewer digest.
var agentReviewerDigests = [][32]byte{
	sha256.Sum256(AgentReviewer()),
}

// olderAgentReviewerDigests holds each earlier release's AgentReviewer
// digest; empty until a release changes it.
var olderAgentReviewerDigests = [][32]byte{}

// skillWorkflowDigests holds this release's SkillWorkflow digest.
var skillWorkflowDigests = [][32]byte{
	sha256.Sum256(SkillWorkflow()),
}

// olderSkillWorkflowDigests holds each earlier release's SkillWorkflow
// digest; empty until a release changes it.
var olderSkillWorkflowDigests = [][32]byte{}

// Recognize reports body's Origin against kind's compiled-in digest lists:
// OriginCurrent for a match against a current render, OriginOlder for a
// match against only an earlier release's render, OriginEdited otherwise.
// An unrecognized Kind always reports OriginEdited.
func Recognize(kind Kind, body []byte) Origin {
	current, older := digestsFor(kind)

	return classify(current, older, body)
}

// classify reports body's Origin against a current and an older digest
// list, checking current first so a digest present in both is current.
func classify(current, older [][32]byte, body []byte) Origin {
	sum := sha256.Sum256(body)

	if slices.Contains(current, sum) {
		return OriginCurrent
	}

	if slices.Contains(older, sum) {
		return OriginOlder
	}

	return OriginEdited
}

// digestsFor returns kind's compiled-in current and older digest lists,
// both nil for a Kind this package does not render.
func digestsFor(kind Kind) ([][32]byte, [][32]byte) {
	switch kind {
	case KindConfig:
		return configDigests, olderConfigDigests
	case KindPluginManifest:
		return pluginManifestDigests, olderPluginManifestDigests
	case KindSkillStart:
		return skillStartDigests, olderSkillStartDigests
	case KindSkillFinish:
		return skillFinishDigests, olderSkillFinishDigests
	case KindClaudeHooks:
		return claudeHooksDigests, olderClaudeHooksDigests
	case KindAgentPlanner:
		return agentPlannerDigests, olderAgentPlannerDigests
	case KindAgentImplementer:
		return agentImplementerDigests, olderAgentImplementerDigests
	case KindAgentReviewer:
		return agentReviewerDigests, olderAgentReviewerDigests
	case KindSkillWorkflow:
		return skillWorkflowDigests, olderSkillWorkflowDigests
	case KindSnippet:
		// Deliberately excluded: SnippetBlock/RecognizeSnippet are the
		// snippet's own render and recognize functions (see doc.go).
		return nil, nil
	default:
		return nil, nil
	}
}
