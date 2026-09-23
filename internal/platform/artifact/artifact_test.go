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

// Test_config_file_resolves_to_the_shipped_defaults guards the trap an
// all-comment ".brief.yaml" is exposed to: config.Inspect treats a
// zero-live-line file the same as a zero-byte one (io.EOF-as-empty), so
// ConfigFile's own bytes — as written, every line commented — must decode
// to config.Default() with no violations. A render that ever emits one
// live line (a "---" marker, say) would break this without touching the
// uncommented form at all.
func Test_config_file_resolves_to_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, artifact.ConfigFile(), 0o600))

	cfg, violations, err := config.Inspect(path)

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, config.Default(), cfg)
}

// Test_uncommenting_the_config_file_yields_the_shipped_defaults strips
// exactly one leading "#" from every line not beginning "# " — the
// uncomment rule R2's copy documents — and proves the result is valid YAML
// that decodes to config.Default() with no violations: the real round trip
// a repository owner performs by hand when they want to change a value.
func Test_uncommenting_the_config_file_yields_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, uncomment(artifact.ConfigFile()), 0o600))

	cfg, violations, err := config.Inspect(path)

	// optional-conventions round-trips through YAML's empty flow sequence
	// "[]" as a non-nil empty slice, never the nil Default() itself holds;
	// the two are the same "nothing set" value, so want is normalized to
	// match rather than treated as a mismatch.
	want := config.Default()
	want.OptionalConventions = []string{}

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, want, cfg)
}

// looksLikeKeyValue reports whether rest — a ConfigFile line with its own
// leading "#" stripped — is a YAML "key: value" line rather than doc prose:
// its own leading-whitespace-trimmed run of lower-case letters, digits and
// hyphens is immediately followed by ":". This is the discriminator
// Test_every_config_key_is_documented needs and a bare "# " prefix check
// cannot give it: ConfigFile comments every line with one leading "#", so a
// nested key under "roles:" ("#  reviewer: \"\"") already starts with "# "
// from its own two-space YAML indent, indistinguishable from a real doc
// line by prefix alone.
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

// Test_every_config_key_is_documented enforces the doc/value line pairing
// structurally: every line of ConfigFile() is commented (R2's "no live
// line at all"), and every value line (looksLikeKeyValue on its own bytes
// with the leading "#" stripped) is immediately preceded by a doc prose
// line — one starting "# " that does not itself look like a key:value pair
// — so no value line, nested or not, documents itself only by accident of
// being readable YAML.
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

// Test_the_bound_config_file_binds_only_the_roles pins ConfigFileWithRoles
// (S08's second KindConfig render, written only by "init --with-agents" in
// the same run it creates the config): it decodes to config.Default() with
// Roles equal to AgentBindings() and no violations, every key outside
// "roles:" stays commented exactly as ConfigFile() writes it, both variants
// recognize as OriginCurrent for KindConfig, and Render(KindConfig) is
// still the plain ConfigFile() — the bound variant is never the "one true"
// render, only a second recognized body.
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

// Test_the_current_config_render_is_a_known_digest pins that ConfigFile's
// own bytes, as shipped today, are recognized by Recognize as the current
// render — the compiled-in registry Recognize checks must always contain
// today's own digest.
func Test_the_current_config_render_is_a_known_digest(t *testing.T) {
	origin := artifact.Recognize(artifact.KindConfig, artifact.ConfigFile())

	assert.Equal(t, artifact.OriginCurrent, origin)
}

// Test_Recognize reports current for bytes matching a compiled-in digest
// and edited for anything else, by table: the current render on one side,
// an arbitrary edited body on the other.
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

// editOneByte returns a copy of body with its last byte changed, the
// smallest possible edit that still differs from body — a fixture for
// proving Recognize rejects even a one-byte mismatch.
func editOneByte(body []byte) []byte {
	out := append([]byte(nil), body...)
	out[len(out)-1]++

	return out
}

// Test_recognize_classifies_each_plugin_file_against_its_own_kind pins
// Recognize for every plugin Kind this package renders: its own current
// render is OriginCurrent, a one-byte edit of it is OriginEdited, and — the
// discriminator proving Recognize checks the kind, not just any known
// digest — another Kind's own current render, checked against a different
// Kind, is OriginEdited too.
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

// Test_hooks_file_runs_brief_check_on_post_tool_use_edits pins ClaudeHooks'
// own JSON shape: a PostToolUse entry matching "Edit|Write|MultiEdit" that
// runs "brief check --hook claude-code" as a "command" hook.
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

// Test_plugin_manifest_names_the_plugin_brief_and_carries_no_version pins
// R4: the manifest names the plugin "brief" and carries no "version" key —
// a skills-directory plugin does not require one, and omitting it keeps
// the render identical release to release.
func Test_plugin_manifest_names_the_plugin_brief_and_carries_no_version(t *testing.T) {
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(artifact.PluginManifest(), &manifest))

	assert.Equal(t, "brief", manifest["name"])
	assert.NotContains(t, manifest, "version")
}

// skillFile is one SKILL.md file's frontmatter and body, parsed by
// parseSkillFile.
type skillFile struct {
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
	AllowedTools           string `yaml:"allowed-tools"`
	ArgumentHint           string `yaml:"argument-hint"`
}

// parseSkillFile splits body into its YAML frontmatter (between "---"
// lines) and the markdown body following it. The returned keys map
// reports every key the frontmatter carries, so a test can assert an
// absence ("name", "user-invocable") that skillFile's own fixed field set
// cannot represent.
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

// Test_skill_files_are_user_invoked_and_scoped_to_their_command pins R4's
// SKILL.md shape for both start and finish: a non-empty description,
// disable-model-invocation true, allowed-tools scoped to that command's
// own "brief <cmd> *" Bash prefix, the matching argument-hint, no "name"
// or "user-invocable" key (both already hold the value brief wants), and a
// body that runs the command with $ARGUMENTS.
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

// wantSkillWorkflow is the brief-workflow skill's ruled bytes, transcribed
// independently from specification.md's own "The skill file" block — never
// derived from artifact.SkillWorkflow() or artifact.Render(), so this test
// actually proves the render against the spec rather than against itself.
const wantSkillWorkflow = "---\n" +
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

// Test_workflow_skill_renders_the_ruled_bytes pins artifact.SkillWorkflow()
// against wantSkillWorkflow, a literal transcribed from specification.md
// rather than from the render itself — R1's own promise that the skill's
// bytes are exactly what the spec ruled, not merely self-consistent.
func Test_workflow_skill_renders_the_ruled_bytes(t *testing.T) {
	assert.Equal(t, []byte(wantSkillWorkflow), artifact.SkillWorkflow())
}

// uncomment strips exactly one leading "#" from every line not beginning
// "# " (hash, space) — the inverse of what ConfigFile writes: a "# " line
// is doc prose and stays a comment forever; every other line is a
// commented YAML line and becomes live.
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
