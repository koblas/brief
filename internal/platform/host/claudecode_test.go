package host_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newClaudeCode returns the "claude-code" Host through host.Lookup, the
// same path production code reaches it by.
func newClaudeCode(t *testing.T) host.Host {
	t.Helper()

	h, ok := host.Lookup(host.ClaudeCode)
	require.True(t, ok)

	return h
}

func Test_claude_code_reads_the_edited_path_from_a_post_tool_use_payload(t *testing.T) {
	h := newClaudeCode(t)

	payload := `{"cwd":"/repo","tool_name":"Edit","tool_input":{"file_path":"/repo/docs/specifications/auth/specification.md"}}`

	path, err := h.HookPath(strings.NewReader(payload))

	require.NoError(t, err)
	assert.Equal(t, "/repo/docs/specifications/auth/specification.md", path)
}

func Test_claude_code_HookPath_ReturnsErrMalformedPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "empty stdin", payload: ""},
		{name: "not JSON", payload: "not json"},
		{name: "no tool_input", payload: `{"cwd":"/repo"}`},
		{name: "empty file_path", payload: `{"tool_input":{"file_path":""}}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newClaudeCode(t)

			_, err := h.HookPath(strings.NewReader(c.payload))

			require.ErrorIs(t, err, host.ErrMalformedPayload)
		})
	}
}

func Test_claude_code_writes_the_summary_as_post_tool_use_additional_context(t *testing.T) {
	h := newClaudeCode(t)

	summary := `brief check: docs/specifications/auth: 2 ERROR findings; run 'brief check "quoted"' ` + "\nwith a newline"

	var buf strings.Builder
	err := h.WriteHookContext(&buf, summary)
	require.NoError(t, err)

	var doc struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	decErr := json.Unmarshal([]byte(buf.String()), &doc)
	require.NoError(t, decErr)

	assert.Equal(t, "PostToolUse", doc.HookSpecificOutput.HookEventName)
	assert.Equal(t, summary, doc.HookSpecificOutput.AdditionalContext)
}

// Test_claude_code_plugin_layout_lists_every_file_and_drops_only_the_hook_without_it
// pins Plugin's own contract: every file lives under host.PluginDir, the
// hook file (and only the hook file) carries Hook true, withHook false
// drops exactly that one entry and leaves the other three untouched, and
// the files carry the artifact.Kind their own render belongs to.
func Test_claude_code_plugin_layout_lists_every_file_and_drops_only_the_hook_without_it(t *testing.T) {
	h := newClaudeCode(t)

	withHook := h.Plugin(true)
	withoutHook := h.Plugin(false)

	require.Len(t, withHook, 4)
	assert.Equal(t, withHook[:3], withoutHook)

	for _, f := range withHook[:3] {
		assert.False(t, f.Hook)
	}
	assert.True(t, withHook[3].Hook)

	for _, f := range withHook {
		assert.True(t, strings.HasPrefix(f.RelPath, host.PluginDir+"/"), "%s must live under %s", f.RelPath, host.PluginDir)
	}

	assert.Equal(t, artifact.KindPluginManifest, withHook[0].Kind)
	assert.Equal(t, artifact.KindSkillStart, withHook[1].Kind)
	assert.Equal(t, artifact.KindSkillFinish, withHook[2].Kind)
	assert.Equal(t, artifact.KindClaudeHooks, withHook[3].Kind)
}

// Test_claude_code_lists_three_agent_files pins Agents' own contract (R4,
// R7): three files under host.PluginDir + "/agents/", planner then
// implementer then reviewer, each carrying its own artifact.Kind and never
// Hook true — the same file list --with-agents plans and, reversed,
// Uninstall always plans for removal.
func Test_claude_code_lists_three_agent_files(t *testing.T) {
	h := newClaudeCode(t)

	agents := h.Agents()

	require.Len(t, agents, 3)
	assert.Equal(t, host.PluginDir+"/agents/planner.md", agents[0].RelPath)
	assert.Equal(t, artifact.KindAgentPlanner, agents[0].Kind)
	assert.Equal(t, host.PluginDir+"/agents/implementer.md", agents[1].RelPath)
	assert.Equal(t, artifact.KindAgentImplementer, agents[1].Kind)
	assert.Equal(t, host.PluginDir+"/agents/reviewer.md", agents[2].RelPath)
	assert.Equal(t, artifact.KindAgentReviewer, agents[2].Kind)

	for _, a := range agents {
		assert.False(t, a.Hook)
	}
}

// Test_claude_code_lists_the_workflow_skill_outside_the_plugin_directory
// pins Skills' own contract (Rule 1/Rule 2): one File at
// host.WorkflowSkillDir + "/SKILL.md", kind artifact.KindSkillWorkflow, not
// Hook, and not living under host.PluginDir — the workflow skill is a
// standalone project skill, never an entry inside the "brief" plugin — and
// a fresh copy per call, mirroring Plugin's and Agents' own contract, so a
// caller mutating one returned slice never affects a later call.
func Test_claude_code_lists_the_workflow_skill_outside_the_plugin_directory(t *testing.T) {
	h := newClaudeCode(t)

	first := h.Skills()

	require.Len(t, first, 1)
	assert.Equal(t, host.WorkflowSkillDir+"/SKILL.md", first[0].RelPath)
	assert.Equal(t, artifact.KindSkillWorkflow, first[0].Kind)
	assert.False(t, first[0].Hook)
	assert.False(t, strings.HasPrefix(first[0].RelPath, host.PluginDir+"/"), "%s must not live under %s", first[0].RelPath, host.PluginDir)

	first[0].RelPath = "mutated"
	second := h.Skills()
	assert.Equal(t, host.WorkflowSkillDir+"/SKILL.md", second[0].RelPath)
}

// Test_claude_code_lists_its_instruction_files_in_priority_order pins
// InstructionFiles's own contract (R5): the repository-root CLAUDE.md
// first, ".claude/CLAUDE.md" second — the order setup's own location rule
// tries them in — and no filesystem access: the same two relative paths
// come back regardless of what does or does not exist on disk.
func Test_claude_code_lists_its_instruction_files_in_priority_order(t *testing.T) {
	h := newClaudeCode(t)

	assert.Equal(t, []string{"CLAUDE.md", ".claude/CLAUDE.md"}, h.InstructionFiles())
}
