package artifact_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// config.Inspect treats a zero-live-line file the same as a zero-byte one,
// so ConfigFile's all-commented bytes must still decode to config.Default().
func Test_config_file_resolves_to_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, artifact.ConfigFile(), 0o600))

	cfg, violations, err := config.Inspect(path)

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, config.Default(), cfg)
}

func Test_uncommenting_the_config_file_yields_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, uncomment(artifact.ConfigFile()), 0o600))

	cfg, violations, err := config.Inspect(path)

	// OptionalConventions round-trips as a non-nil "[]", not Default()'s nil.
	want := config.Default()
	want.OptionalConventions = []string{}

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, want, cfg)
}

// looksLikeKeyValue reports whether rest (a ConfigFile line with its
// leading "#" stripped) is a YAML "key: value" line rather than doc prose.
func looksLikeKeyValue(rest string) bool {
	trimmed := strings.TrimLeft(rest, " ")

	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return false
	}

	for _, r := range trimmed[:idx] {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", r) {
			return false
		}
	}

	return true
}

func Test_every_config_key_is_documented(t *testing.T) {
	body := strings.TrimRight(string(artifact.ConfigFile()), "\n")
	lines := strings.Split(body, "\n")
	require.NotEmpty(t, lines)

	for i, line := range lines {
		require.Truef(t, strings.HasPrefix(line, "#"), "line %d (%q) must be commented", i, line)

		if !looksLikeKeyValue(strings.TrimPrefix(line, "#")) {
			continue
		}

		require.Positivef(t, i, "value line %d (%q) has no preceding doc line", i, line)

		prev := lines[i-1]
		assert.Truef(t, strings.HasPrefix(prev, "# ") && !looksLikeKeyValue(strings.TrimPrefix(prev, "#")),
			"value line %d (%q) must be preceded by a doc line, got %q", i, line, prev)
	}
}

func Test_the_bound_config_file_binds_only_the_roles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, artifact.ConfigFileWithRoles(), 0o600))

	cfg, violations, err := config.Inspect(path)

	require.NoError(t, err)
	assert.Empty(t, violations)

	want := config.Default()
	want.Roles = artifact.AgentBindings()
	assert.Equal(t, want, cfg)

	plain := strings.Split(strings.TrimRight(string(artifact.ConfigFile()), "\n"), "\n")
	bound := strings.Split(strings.TrimRight(string(artifact.ConfigFileWithRoles()), "\n"), "\n")
	require.Len(t, bound, len(plain))

	for i, line := range bound {
		if strings.Contains(line, "roles:") || strings.Contains(line, "planner:") || strings.Contains(line, "implementer:") || strings.Contains(line, "reviewer:") {
			continue
		}

		assert.Equalf(t, plain[i], line, "line %d must be unchanged outside the roles bindings", i)
	}

	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindConfig, artifact.ConfigFile()))
	assert.Equal(t, artifact.OriginCurrent, artifact.Recognize(artifact.KindConfig, artifact.ConfigFileWithRoles()))
	assert.Equal(t, artifact.ConfigFile(), artifact.Render(artifact.KindConfig))
}

func Test_the_current_config_render_is_a_known_digest(t *testing.T) {
	origin := artifact.Recognize(artifact.KindConfig, artifact.ConfigFile())

	assert.Equal(t, artifact.OriginCurrent, origin)
}

func Test_Recognize(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want artifact.Origin
	}{
		{name: "current render", body: artifact.ConfigFile(), want: artifact.OriginCurrent},
		{name: "locally edited bytes", body: []byte("feature-directory: elsewhere\n"), want: artifact.OriginEdited},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, artifact.Recognize(artifact.KindConfig, c.body))
		})
	}
}

// editOneByte returns a copy of body with its last byte changed — the
// smallest edit that still differs from body.
func editOneByte(body []byte) []byte {
	out := append([]byte(nil), body...)
	out[len(out)-1]++

	return out
}

func Test_recognize_classifies_each_plugin_file_against_its_own_kind(t *testing.T) {
	cases := []struct {
		name string
		kind artifact.Kind
		body []byte
		want artifact.Origin
	}{
		{name: "plugin manifest: current render", kind: artifact.KindPluginManifest, body: artifact.PluginManifest(), want: artifact.OriginCurrent},
		{name: "plugin manifest: one byte edited", kind: artifact.KindPluginManifest, body: editOneByte(artifact.PluginManifest()), want: artifact.OriginEdited},
		{name: "skill start: current render", kind: artifact.KindSkillStart, body: artifact.SkillStart(), want: artifact.OriginCurrent},
		{name: "skill start: one byte edited", kind: artifact.KindSkillStart, body: editOneByte(artifact.SkillStart()), want: artifact.OriginEdited},
		{name: "skill finish: current render", kind: artifact.KindSkillFinish, body: artifact.SkillFinish(), want: artifact.OriginCurrent},
		{name: "skill finish: one byte edited", kind: artifact.KindSkillFinish, body: editOneByte(artifact.SkillFinish()), want: artifact.OriginEdited},
		{name: "claude hooks: current render", kind: artifact.KindClaudeHooks, body: artifact.ClaudeHooks(), want: artifact.OriginCurrent},
		{name: "claude hooks: one byte edited", kind: artifact.KindClaudeHooks, body: editOneByte(artifact.ClaudeHooks()), want: artifact.OriginEdited},
		{name: "agent planner: current render", kind: artifact.KindAgentPlanner, body: artifact.AgentPlanner(), want: artifact.OriginCurrent},
		{name: "agent planner: one byte edited", kind: artifact.KindAgentPlanner, body: editOneByte(artifact.AgentPlanner()), want: artifact.OriginEdited},
		{name: "agent implementer: current render", kind: artifact.KindAgentImplementer, body: artifact.AgentImplementer(), want: artifact.OriginCurrent},
		{name: "agent implementer: one byte edited", kind: artifact.KindAgentImplementer, body: editOneByte(artifact.AgentImplementer()), want: artifact.OriginEdited},
		{name: "agent reviewer: current render", kind: artifact.KindAgentReviewer, body: artifact.AgentReviewer(), want: artifact.OriginCurrent},
		{name: "agent reviewer: one byte edited", kind: artifact.KindAgentReviewer, body: editOneByte(artifact.AgentReviewer()), want: artifact.OriginEdited},
		{name: "skill start bytes checked against skill finish's kind", kind: artifact.KindSkillFinish, body: artifact.SkillStart(), want: artifact.OriginEdited},
		{name: "agent planner bytes checked against agent reviewer's kind", kind: artifact.KindAgentReviewer, body: artifact.AgentPlanner(), want: artifact.OriginEdited},
		{name: "workflow skill: current render", kind: artifact.KindSkillWorkflow, body: artifact.SkillWorkflow(), want: artifact.OriginCurrent},
		{name: "workflow skill: one byte edited", kind: artifact.KindSkillWorkflow, body: editOneByte(artifact.SkillWorkflow()), want: artifact.OriginEdited},
		{name: "skill start bytes checked against workflow skill's kind", kind: artifact.KindSkillWorkflow, body: artifact.SkillStart(), want: artifact.OriginEdited},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, artifact.Recognize(c.kind, c.body))
		})
	}
}

func Test_hooks_file_runs_brief_check_on_post_tool_use_edits(t *testing.T) {
	var doc struct {
		Hooks struct {
			PostToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PostToolUse"`
		} `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(artifact.ClaudeHooks(), &doc))

	require.Len(t, doc.Hooks.PostToolUse, 1)
	assert.Equal(t, "Edit|Write|MultiEdit", doc.Hooks.PostToolUse[0].Matcher)
	require.Len(t, doc.Hooks.PostToolUse[0].Hooks, 1)
	assert.Equal(t, "command", doc.Hooks.PostToolUse[0].Hooks[0].Type)
	assert.Equal(t, "brief check --hook claude-code", doc.Hooks.PostToolUse[0].Hooks[0].Command)
}

func Test_plugin_manifest_names_the_plugin_brief_and_carries_no_version(t *testing.T) {
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(artifact.PluginManifest(), &manifest))

	assert.Equal(t, "brief", manifest["name"])
	assert.NotContains(t, manifest, "version")
}

// skillFile is one SKILL.md file's frontmatter and body.
type skillFile struct {
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
	AllowedTools           string `yaml:"allowed-tools"`
	ArgumentHint           string `yaml:"argument-hint"`
}

// parseSkillFile splits body into its YAML frontmatter and markdown body.
// The returned keys map reports every frontmatter key, so a test can
// assert an absence skillFile's fixed field set cannot represent.
func parseSkillFile(t *testing.T, body []byte) (skillFile, map[string]any, string) {
	t.Helper()

	s := string(body)
	require.True(t, strings.HasPrefix(s, "---\n"), "must open with a frontmatter fence")

	after := strings.TrimPrefix(s, "---\n")
	idx := strings.Index(after, "\n---\n")
	require.GreaterOrEqual(t, idx, 0, "must close the frontmatter fence")

	frontmatter := after[:idx]
	rest := after[idx+len("\n---\n"):]

	var fm skillFile
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &fm))

	var keys map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &keys))

	return fm, keys, rest
}

func Test_skill_files_are_user_invoked_and_scoped_to_their_command(t *testing.T) {
	cases := []struct {
		name         string
		body         []byte
		allowedTools string
		argumentHint string
		runs         string
	}{
		{name: "start", body: artifact.SkillStart(), allowedTools: "Bash(brief start *)", argumentHint: "<feature>", runs: "brief start $ARGUMENTS"},
		{name: "finish", body: artifact.SkillFinish(), allowedTools: "Bash(brief finish *)", argumentHint: "<feature> <step> --handoff <path> --state <path>", runs: "brief finish $ARGUMENTS"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm, keys, body := parseSkillFile(t, c.body)

			assert.NotEmpty(t, fm.Description)
			assert.True(t, fm.DisableModelInvocation)
			assert.Equal(t, c.allowedTools, fm.AllowedTools)
			assert.Equal(t, c.argumentHint, fm.ArgumentHint)
			assert.NotContains(t, keys, "name")
			assert.NotContains(t, keys, "user-invocable")
			assert.Contains(t, body, c.runs)
		})
	}
}

// wantSkillWorkflow is transcribed independently, never derived from artifact.SkillWorkflow().
const wantSkillWorkflow = "---\n" +
	"description: brief's feature workflow — open a feature with brief new feature, add steps with brief new step, pick up the next open step with " +
	"brief start, tick its checklist, close it with brief finish. Use when starting multi-step work, or when planning, implementing or reviewing a " +
	"step of a brief-tracked feature.\n" +
	"user-invocable: false\n" +
	"allowed-tools:\n" +
	"  - Bash(brief new feature *)\n" +
	"  - Bash(brief new step *)\n" +
	"  - Bash(brief start *)\n" +
	"  - Bash(brief finish *)\n" +
	"  - Bash(brief status *)\n" +
	"  - Bash(brief check *)\n" +
	"---\n" +
	"\n" +
	"# brief workflow\n" +
	"\n" +
	"A feature is a directory of markdown: a specification, ordered step files, and one\n" +
	"state file. `brief status` lists every feature and its next open step.\n" +
	"\n" +
	"## Lifecycle\n" +
	"\n" +
	"1. **Open.** Multi-step work gets a feature: `brief new feature <name>` writes its\n" +
	"   skeleton. Write the specification — what the feature is for and the acceptance\n" +
	"   criteria that say it is done — before adding steps; brief never writes it for you.\n" +
	"2. **Plan.** `brief new step <feature>` scaffolds each step in order with an\n" +
	"   acceptance heading and a checklist heading; fill both. brief does not decide what\n" +
	"   the steps are: have the list approved by whoever owns the feature before the first\n" +
	"   `brief start`. A step with no checklist items cannot be finished.\n" +
	"3. **One step at a time.** Only one step per feature is open for work: take it from\n" +
	"   `brief start` through `brief finish` before starting the next.\n" +
	"4. **Review before finish.** brief does not decide whether a step is reviewed. If it\n" +
	"   is, review after the checklist is ticked and before `brief finish`, and send\n" +
	"   findings back to whoever implements it. A finished step's handoff and state are\n" +
	"   fixed — `brief finish` refuses different content for it later — so a finding after\n" +
	"   finish becomes a new step.\n" +
	"5. **Roles.** Where `.brief.yaml` binds `roles:`, the planner does step 2, the\n" +
	"   implementer runs the step protocol below, and the reviewer reads with `brief start`\n" +
	"   and `brief check` without editing.\n" +
	"\n" +
	"## Step protocol\n" +
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

func Test_workflow_skill_renders_the_ruled_bytes(t *testing.T) {
	assert.Equal(t, []byte(wantSkillWorkflow), artifact.SkillWorkflow())
}

// uncomment strips one leading "#" from every line not beginning "# ",
// the inverse of what ConfigFile writes.
func uncomment(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			continue
		}

		lines[i] = strings.TrimPrefix(line, "#")
	}

	return []byte(strings.Join(lines, "\n"))
}
