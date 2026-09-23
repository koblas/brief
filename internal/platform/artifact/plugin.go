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

// skillWorkflowBody is SkillWorkflow's own exact bytes, ruled by
// specification.md's "The skill file" block: a YAML frontmatter block
// scoping allowed-tools to brief's step-protocol verbs, user-invocable
// false (it is preloaded via an agent's frontmatter "skills:", never run as
// a slash command), and a body teaching the brief start / tick / brief
// finish / brief new step protocol.
const skillWorkflowBody = "---\n" +
	"description: brief's step protocol — pick up a feature's next open step with brief start, tick its checklist as items go green, close it with " +
	"brief finish. Use when planning or implementing a step of a brief-tracked feature.\n" +
	"user-invocable: false\n" +
	"allowed-tools:\n" +
	"  - Bash(brief start *)\n" +
	"  - Bash(brief finish *)\n" +
	"  - Bash(brief new step *)\n" +
	"  - Bash(brief status *)\n" +
	"  - Bash(brief check *)\n" +
	"---\n" +
	"\n" +
	"# brief step protocol\n" +
	"\n" +
	"A feature is a directory of markdown: a specification, ordered step files, and one\n" +
	"state file. `brief status` lists every feature and its next open step.\n" +
	"\n" +
	"1. **Start.** `brief start <feature>` prints the next open step — its id, acceptance\n" +
	"   criteria and checklist — and the decisions it inherits from the state file. Work\n" +
	"   from that output; do not read the specification or earlier handoffs whole. It\n" +
	"   writes nothing.\n" +
	"2. **Work.** As each checklist item goes green, tick it by hand in the step file:\n" +
	"   `- [ ]` becomes `- [x]`. This is the only bookkeeping edit you make yourself.\n" +
	"   `brief finish` refuses while any item is unticked.\n" +
	"3. **Finish.** Write two bodies to scratch files (or pass `-` for one, read from stdin):\n" +
	"   - the handoff: what this step decided, what it left undone, what the next step\n" +
	"     must know;\n" +
	"   - the state: a COMPLETE replacement of the feature's state file — every inherited\n" +
	"     section `brief start` printed, updated, not just this step's delta. Anything you\n" +
	"     leave out is dropped.\n" +
	"   Then run `brief finish <feature> <step> --handoff <path> --state <path>`. It ticks\n" +
	"   the progress list, marks the step done, writes the step's handoff file and replaces\n" +
	"   the state file — all or nothing. A refusal names what to fix (an unticked item, a\n" +
	"   missing state heading, a body over its line cap) and changes no files; fix it and\n" +
	"   run it again.\n" +
	"4. **Add a step.** `brief new step <feature>` scaffolds the next step file and its\n" +
	"   progress entry; fill in its body.\n" +
	"\n" +
	"Never tick the progress list, mark a step done, write a handoff file or edit the state\n" +
	"file by hand: `brief finish` is the only way a step closes. Headings, file names and\n" +
	"caps are configured per repository, and brief's own output names the ones in force.\n" +
	"`brief <command> --help` covers every flag.\n"

// SkillWorkflow renders ".claude/skills/brief-workflow/SKILL.md": the
// step-protocol skill any agent — brief's own or one the repository already
// has — preloads via its frontmatter "skills:" to learn brief start /
// tick-by-hand / brief finish / brief new step. Unlike SkillStart and
// SkillFinish, it is not user-invocable and carries no argument-hint: it is
// never run as a slash command, only preloaded.
func SkillWorkflow() []byte {
	return []byte(skillWorkflowBody)
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
	case KindAgentPlanner:
		return AgentPlanner()
	case KindAgentImplementer:
		return AgentImplementer()
	case KindAgentReviewer:
		return AgentReviewer()
	case KindSkillWorkflow:
		return SkillWorkflow()
	case KindSnippet:
		// Deliberately excluded: SnippetBlock is the snippet's own render
		// function, taking the feature directory Render's signature has no
		// room for (see doc.go).
		return nil
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
