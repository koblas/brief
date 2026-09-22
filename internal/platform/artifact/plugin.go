package artifact

import (
	"encoding/json"
	"fmt"
	"strings"
)

// pluginManifest is PluginManifest's own JSON shape: a skills-directory
// plugin needs only a name — no version, so every release's manifest bytes
// stay identical and a digest can never distinguish "older release" from
// "edited" on this field alone.
type pluginManifest struct {
	Name string `json:"name"`
}

// PluginManifest renders ".claude-plugin/plugin.json": {"name": "brief"},
// two-space indented, trailing newline. It carries no "version" key —
// code.claude.com/docs/en/plugins-reference confirms version is optional
// for a skills-directory plugin — so this render never changes across a
// release unless the plugin's own name does.
func PluginManifest() []byte {
	return mustMarshalIndent(pluginManifest{Name: "brief"})
}

// Every SKILL.md file this package renders carries the same frontmatter
// shape (renderSkill): a non-empty description, disable-model-invocation
// pinned true (the skill only ever runs as an explicit slash command,
// never model-invoked), allowed-tools scoped to the one "brief <cmd> *"
// Bash prefix that command needs, and an argument-hint naming its
// positional shape. "name" (defaults to the skill's directory name) and
// "user-invocable" (defaults true) are left out — both already hold the
// value brief wants.

// skillStartDescription is SkillStart's own frontmatter "description".
const skillStartDescription = "Run brief start for the named feature and continue from its output."

// skillFinishDescription is SkillFinish's own frontmatter "description".
const skillFinishDescription = "Run brief finish to close a step and continue from its output."

// SkillStart renders "skills/start/SKILL.md": frontmatter scoping
// allowed-tools to "Bash(brief start *)" with argument-hint "<feature>",
// and a body that runs "brief start $ARGUMENTS" and works from its output.
func SkillStart() []byte {
	return renderSkill(skillStartDescription, "Bash(brief start *)", "<feature>", "brief start $ARGUMENTS")
}

// SkillFinish renders "skills/finish/SKILL.md": frontmatter scoping
// allowed-tools to "Bash(brief finish *)" with argument-hint "<feature>
// <step> --handoff <path> --state <path>", and a body that runs "brief
// finish $ARGUMENTS" and works from its output.
func SkillFinish() []byte {
	return renderSkill(skillFinishDescription, "Bash(brief finish *)", "<feature> <step> --handoff <path> --state <path>", "brief finish $ARGUMENTS")
}

// renderSkill builds one SKILL.md's exact bytes: a YAML frontmatter block
// between "---" lines carrying description, disable-model-invocation,
// allowed-tools and argument-hint, then a body naming invocation as the
// command to run and work from.
func renderSkill(description, allowedTools, argumentHint, invocation string) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", description)
	b.WriteString("disable-model-invocation: true\n")
	fmt.Fprintf(&b, "allowed-tools: %q\n", allowedTools)
	fmt.Fprintf(&b, "argument-hint: %q\n", argumentHint)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "Run `%s` and work from its output.\n", invocation)

	return []byte(b.String())
}

// claudeHooksFile is ClaudeHooks's own JSON shape: the settings.json hook
// format code.claude.com/docs/en/hooks documents, scoped to one
// PostToolUse entry.
type claudeHooksFile struct {
	Hooks claudeHooksSection `json:"hooks"`
}

// claudeHooksSection is claudeHooksFile's own "hooks" object.
type claudeHooksSection struct {
	PostToolUse []claudeHookMatcher `json:"PostToolUse"`
}

// claudeHookMatcher is one PostToolUse entry: the tool-name matcher and
// the commands it runs.
type claudeHookMatcher struct {
	Matcher string              `json:"matcher"`
	Hooks   []claudeHookCommand `json:"hooks"`
}

// claudeHookCommand is one hook command: its type, always "command" for
// what brief installs, and the shell command it runs.
type claudeHookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// claudeCheckHookCommand is the shell command ClaudeHooks wires to every
// Edit/Write/MultiEdit PostToolUse event: check --hook's own claude-code
// invocation (R12).
const claudeCheckHookCommand = "brief check --hook claude-code"

// ClaudeHooks renders "hooks/hooks.json": one PostToolUse entry matching
// "Edit|Write|MultiEdit" that runs "brief check --hook claude-code" as a
// "command" hook.
func ClaudeHooks() []byte {
	doc := claudeHooksFile{
		Hooks: claudeHooksSection{
			PostToolUse: []claudeHookMatcher{
				{
					Matcher: "Edit|Write|MultiEdit",
					Hooks: []claudeHookCommand{
						{Type: "command", Command: claudeCheckHookCommand},
					},
				},
			},
		},
	}

	return mustMarshalIndent(doc)
}

// Render returns kind's own current bytes — written bytes and recognized
// bytes are always one value, since digestsFor's own compiled-in digests
// are computed from these same render functions — or nil for a Kind this
// package does not render.
func Render(kind Kind) []byte {
	switch kind {
	case KindConfig:
		return ConfigFile()
	case KindPluginManifest:
		return PluginManifest()
	case KindSkillStart:
		return SkillStart()
	case KindSkillFinish:
		return SkillFinish()
	case KindClaudeHooks:
		return ClaudeHooks()
	default:
		return nil
	}
}

// mustMarshalIndent marshals v as two-space-indented JSON with a trailing
// newline. v is always one of this package's own fixed, hand-written
// values, so a marshal failure would be a bug in this package, not a
// runtime condition a caller can act on.
func mustMarshalIndent(v any) []byte {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("artifact: marshal %T: %v", v, err))
	}

	return append(body, '\n')
}
