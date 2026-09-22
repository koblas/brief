package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/koblas/brief/internal/platform/artifact"
)

// ClaudeCode is the host name "brief check --hook claude-code" accepts.
const ClaudeCode = "claude-code"

// PluginDir is the path, relative to a repository's install root, every
// claude-code skills-directory plugin file lives under (R4).
const PluginDir = ".claude/skills/brief"

// claudeCodePluginFiles lists claudeCode's own Plugin(true) files, in
// install order: the manifest, the start skill, the finish skill, then the
// PostToolUse hook wiring last.
var claudeCodePluginFiles = []File{
	{RelPath: PluginDir + "/.claude-plugin/plugin.json", Kind: artifact.KindPluginManifest},
	{RelPath: PluginDir + "/skills/start/SKILL.md", Kind: artifact.KindSkillStart},
	{RelPath: PluginDir + "/skills/finish/SKILL.md", Kind: artifact.KindSkillFinish},
	{RelPath: PluginDir + "/hooks/hooks.json", Kind: artifact.KindClaudeHooks, Hook: true},
}

// claudeCode adapts Claude Code's PostToolUse hook protocol
// (code.claude.com/docs/en/hooks): a JSON payload on stdin carrying
// tool_input.file_path, and a JSON hook-context response on stdout.
type claudeCode struct{}

// hookPayload is the subset of a PostToolUse payload HookPath reads. The
// payload also carries cwd, tool_name and hook_event_name; HookPath never
// resolves a relative file_path against cwd — the caller resolves it
// against its own working directory instead.
type hookPayload struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// Name reports "claude-code".
func (claudeCode) Name() string { return ClaudeCode }

// HookPath decodes r as a hookPayload and returns its tool_input.file_path.
// It returns ErrMalformedPayload when r is empty, does not decode as JSON,
// or decodes to an empty file_path.
func (claudeCode) HookPath(r io.Reader) (string, error) {
	var payload hookPayload

	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return "", fmt.Errorf("%w: %w", ErrMalformedPayload, err)
	}

	if payload.ToolInput.FilePath == "" {
		return "", ErrMalformedPayload
	}

	return payload.ToolInput.FilePath, nil
}

// hookSpecificOutput is claudeCode's hook-context response's own nested
// object: the PostToolUse event name, fixed, and the text that reaches the
// model as context.
type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// hookContextDocument is claudeCode's WriteHookContext document.
type hookContextDocument struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

// WriteHookContext writes summary to w as one compact JSON document:
// {"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"<summary>"}},
// encoded into a buffer first so w sees either the complete document or
// nothing. A summary carrying quotes or a newline still renders as one
// valid JSON document: encoding/json escapes it.
func (claudeCode) WriteHookContext(w io.Writer, summary string) error {
	doc := hookContextDocument{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     "PostToolUse",
			AdditionalContext: summary,
		},
	}

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("host: write hook context: %w", err)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("host: write hook context: %w", err)
	}

	return nil
}

// Plugin returns claudeCodePluginFiles, minus its trailing hook entry when
// withHook is false. The returned slice is always a fresh copy, so a
// caller mutating it never affects a later call.
func (claudeCode) Plugin(withHook bool) []File {
	files := claudeCodePluginFiles
	if !withHook {
		files = files[:len(files)-1]
	}

	return append([]File(nil), files...)
}

// claudeCodeAgentFiles lists claudeCode's own Agents, in install order:
// planner, implementer, reviewer.
var claudeCodeAgentFiles = []File{
	{RelPath: PluginDir + "/agents/planner.md", Kind: artifact.KindAgentPlanner},
	{RelPath: PluginDir + "/agents/implementer.md", Kind: artifact.KindAgentImplementer},
	{RelPath: PluginDir + "/agents/reviewer.md", Kind: artifact.KindAgentReviewer},
}

// Agents returns claudeCodeAgentFiles, a fresh copy per call.
func (claudeCode) Agents() []File {
	return append([]File(nil), claudeCodeAgentFiles...)
}

// claudeCodeInstructionFiles lists claudeCode's own InstructionFiles, in
// priority order: the repository-root CLAUDE.md, then ".claude/CLAUDE.md".
var claudeCodeInstructionFiles = []string{"CLAUDE.md", ".claude/CLAUDE.md"}

// InstructionFiles returns claudeCodeInstructionFiles, a fresh copy per
// call.
func (claudeCode) InstructionFiles() []string {
	return append([]string(nil), claudeCodeInstructionFiles...)
}
