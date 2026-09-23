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
	// KindSkillWorkflow is SkillWorkflow's own Kind.
	KindSkillWorkflow Kind = "skill-workflow"
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
// digest lists: OriginCurrent for a byte-for-byte match against today's own
// render (or, for KindConfig, any current render this package ships),
// OriginOlder for a match against an earlier release's own render, and
// OriginEdited for anything else.
type Origin string

const (
	// OriginCurrent marks bytes equal to one of this binary's own current
	// renders for the given Kind.
	OriginCurrent Origin = "current"
	// OriginOlder marks bytes equal to an earlier release's own render for
	// the given Kind, never today's.
	OriginOlder Origin = "older"
	// OriginEdited marks bytes that match no compiled-in digest, current or
	// older, for the given Kind.
	OriginEdited Origin = "edited"
)

// configDigests holds the sha256 digest of this release's own ConfigFile
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

// olderConfigDigests holds the sha256 digest of every earlier release's own
// ConfigFile or ConfigFileWithRoles render, empty until a release changes
// either one.
var olderConfigDigests = [][32]byte{}

// pluginManifestDigests holds the sha256 digest of this release's own
// PluginManifest render.
var pluginManifestDigests = [][32]byte{
	sha256.Sum256(PluginManifest()),
}

// olderPluginManifestDigests holds the sha256 digest of every earlier
// release's own PluginManifest render, empty until a release changes it.
var olderPluginManifestDigests = [][32]byte{}

// skillStartDigests holds the sha256 digest of this release's own
// SkillStart render.
var skillStartDigests = [][32]byte{
	sha256.Sum256(SkillStart()),
}

// olderSkillStartDigests holds the sha256 digest of every earlier release's
// own SkillStart render, empty until a release changes it.
var olderSkillStartDigests = [][32]byte{}

// skillFinishDigests holds the sha256 digest of this release's own
// SkillFinish render.
var skillFinishDigests = [][32]byte{
	sha256.Sum256(SkillFinish()),
}

// olderSkillFinishDigests holds the sha256 digest of every earlier
// release's own SkillFinish render, empty until a release changes it.
var olderSkillFinishDigests = [][32]byte{}

// claudeHooksDigests holds the sha256 digest of this release's own
// ClaudeHooks render.
var claudeHooksDigests = [][32]byte{
	sha256.Sum256(ClaudeHooks()),
}

// olderClaudeHooksDigests holds the sha256 digest of every earlier
// release's own ClaudeHooks render, empty until a release changes it.
var olderClaudeHooksDigests = [][32]byte{}

// agentPlannerDigests holds the sha256 digest of this release's own
// AgentPlanner render.
var agentPlannerDigests = [][32]byte{
	sha256.Sum256(AgentPlanner()),
}

// olderAgentPlannerDigests holds the sha256 digest of every earlier
// release's own AgentPlanner render, one fixed file under files/older per
// release — never a live render, which would duplicate agentPlannerDigests
// and make the OriginOlder arm unreachable.
var olderAgentPlannerDigests = [][32]byte{
	sha256.Sum256(mustReadFile("older/agents/planner.md")),
}

// agentImplementerDigests holds the sha256 digest of this release's own
// AgentImplementer render.
var agentImplementerDigests = [][32]byte{
	sha256.Sum256(AgentImplementer()),
}

// olderAgentImplementerDigests holds the sha256 digest of every earlier
// release's own AgentImplementer render, one fixed file under files/older
// per release — never a live render, which would duplicate
// agentImplementerDigests and make the OriginOlder arm unreachable.
var olderAgentImplementerDigests = [][32]byte{
	sha256.Sum256(mustReadFile("older/agents/implementer.md")),
}

// agentReviewerDigests holds the sha256 digest of this release's own
// AgentReviewer render.
var agentReviewerDigests = [][32]byte{
	sha256.Sum256(AgentReviewer()),
}

// olderAgentReviewerDigests holds the sha256 digest of every earlier
// release's own AgentReviewer render, empty until a release changes it.
var olderAgentReviewerDigests = [][32]byte{}

// skillWorkflowDigests holds the sha256 digest of this release's own
// SkillWorkflow render.
var skillWorkflowDigests = [][32]byte{
	sha256.Sum256(SkillWorkflow()),
}

// olderSkillWorkflowDigests holds the sha256 digest of every earlier
// release's own SkillWorkflow render, empty until a release changes it.
var olderSkillWorkflowDigests = [][32]byte{}

// Recognize reports body's Origin against kind's own compiled-in digest
// lists: OriginCurrent when body's sha256 digest matches one of this
// binary's own current renders for kind, OriginOlder when it matches only
// an earlier release's own render, OriginEdited otherwise. An unrecognized
// Kind reports OriginEdited, since there are no digest lists to match
// against — including a Kind's bytes checked against a different Kind's own
// lists, which never match.
func Recognize(kind Kind, body []byte) Origin {
	current, older := digestsFor(kind)

	return classify(current, older, body)
}

// classify reports body's Origin against a current and an older digest
// list: OriginCurrent when body's sha256 digest is in current (checked
// first, so a digest present in both lists is always current), OriginOlder
// when it is only in older, OriginEdited otherwise.
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

// digestsFor returns kind's own compiled-in current and older digest
// lists, both nil for a Kind this package does not render.
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
