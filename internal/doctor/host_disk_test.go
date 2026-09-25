package doctor_test

// OS-subject host-integration tests: every case here is on real disk
// (t.TempDir via newHostFixture), because its own subject is an OS error
// shape classifyProbeError has to discriminate — a chmod'd file or
// directory (present-but-unreadable), or an ancestor path component that
// is itself a regular file (ENOTDIR): a real filesystem reports ENOTDIR
// for the latter, but an equivalent fstest.MapFS fixture reports plain
// fs.ErrNotExist instead, so only real disk exercises that arm.
// host_test.go holds every case whose subject is the classification rule
// itself, run against an in-memory fstest.MapFS.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHostFixture builds a wd with a valid ".brief.yaml" (unbound roles)
// and its default feature root, but no Claude Code integration installed
// — the baseline every case in this file starts from.
func newHostFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: \"## Progress\"\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

	return wd
}

// writeHostArtifact writes f's current artifact.Render at wd/f.RelPath.
func writeHostArtifact(t *testing.T, wd string, f host.File) {
	t.Helper()

	writeHostFile(t, wd, f.RelPath, artifact.Render(f.Kind))
}

// hostCheckCase is one row of a host-check classification table: setup
// mutates newHostFixture's own bare baseline, and Diagnose's report must
// carry checkID at wantSeverity, with wantDetail a substring of Detail,
// wantFix the exact Fix text (nil when the row carries none), and wantRel
// the row's own Path exactly, relative to wd.
type hostCheckCase struct {
	name         string
	setup        func(t *testing.T, wd string, h host.Host)
	checkID      string
	wantSeverity doctor.Severity
	wantDetail   string
	wantFix      *string
	wantRel      string
}

// runHostCheckCases builds newHostFixture(t), applies c.setup, runs
// Diagnose with an injected empty home, and asserts c.checkID's own row.
func runHostCheckCases(t *testing.T, cases []hostCheckCase) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHostFixture(t)
			h := claudeCodeHost(t)
			c.setup(t, wd, h)

			srv := doctor.NewServer(emptyHomeDir(t))
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, c.checkID)
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Contains(t, check.Detail, c.wantDetail)
			assert.Equal(t, c.wantFix, check.Fix)
			assert.Equal(t, filepath.Join(wd, filepath.FromSlash(c.wantRel)), check.Path)
		})
	}
}

// chmodUnreadable chmods path to 0o000 and registers a t.Cleanup that
// restores it to 0o600 before TempDir's removal runs. Skips under euid 0,
// where chmod's permission bits have no effect.
func chmodUnreadable(t *testing.T, path string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
}

// chmodUnreadableDir chmods dir to 0o000 and registers a t.Cleanup that
// restores it to 0o755 before TempDir's removal runs. Skips under euid 0.
func chmodUnreadableDir(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// A ".claude" ancestor that is itself a regular file fails Lstat with
// ENOTDIR, which classifyProbeError proves is absence, never
// present-but-unreadable.
func Test_diagnose_reports_not_installed_when_the_claude_root_is_a_regular_file(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name: "host-plugin",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      host.PluginDir,
		},
		{
			name: "host-hook",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			name: "host-agents",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitWithAgents),
			wantRel:      host.PluginDir + "/agents",
		},
	})
}

// An ancestor directory doctor cannot even Lstat into is unreadable,
// never "missing"; both are ERROR. The fix targets whichever call
// actually failed: a failed Lstat targets blockingDir's own result; a
// failed ReadFile targets the subject file itself (chmod +r).
func Test_diagnose_classifies_host_plugin_unreadable(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			// Control: the identical install, fully readable, OK.
			name: "every subject file is current",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      host.PluginDir,
		},
		{
			name: "a subject file is unreadable, not missing",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				chmodUnreadableDir(t, filepath.Join(wd, ".claude"))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail: "not readable (permission denied): " + strings.Join([]string{
				host.PluginDir + "/.claude-plugin/plugin.json",
				host.PluginDir + "/skills/start/SKILL.md",
				host.PluginDir + "/skills/finish/SKILL.md",
			}, ", "),
			wantFix: new("chmod u+rwx .claude, then " + runInitClaudeCode),
			wantRel: host.PluginDir,
		},
		{
			// The blocking directory is two levels below root
			// (".claude/skills"), not root's immediate child (".claude",
			// left at 0o755): the walk must climb past every
			// unresolvable descendant and stop at the first ancestor
			// whose own Lstat succeeds.
			name: "the blocking dir is .claude/skills, .claude itself stays 0755",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				chmodUnreadableDir(t, filepath.Join(wd, ".claude", "skills"))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail: "not readable (permission denied): " + strings.Join([]string{
				host.PluginDir + "/.claude-plugin/plugin.json",
				host.PluginDir + "/skills/start/SKILL.md",
				host.PluginDir + "/skills/finish/SKILL.md",
			}, ", "),
			wantFix: new("chmod u+rwx .claude/skills, then " + runInitClaudeCode),
			wantRel: host.PluginDir,
		},
		{
			// Every directory stays searchable; only the manifest file
			// itself is chmodded 0o000, so the failure is in the
			// ReadFile call rather than the Lstat call (statFailed is
			// false).
			name: "a subject file itself is unreadable, its directory is searchable",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				var manifestPath string

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)

					if f.Kind == artifact.KindPluginManifest {
						manifestPath = filepath.Join(wd, filepath.FromSlash(f.RelPath))
					}
				}

				chmodUnreadable(t, manifestPath)
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "not readable (permission denied): " + host.PluginDir + "/.claude-plugin/plugin.json",
			wantFix:      new("chmod +r " + host.PluginDir + "/.claude-plugin/plugin.json, then " + runInitClaudeCode),
			wantRel:      host.PluginDir,
		},
		{
			// The install root itself (wd) is the one unsearchable
			// directory: blockingDir's walk never finds a resolvable
			// ancestor before dir == root, so it falls to its own
			// "return root" fallback.
			name: "the install root itself is unsearchable",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				chmodUnreadableDir(t, wd)
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail: "not readable (permission denied): " + strings.Join([]string{
				host.PluginDir + "/.claude-plugin/plugin.json",
				host.PluginDir + "/skills/start/SKILL.md",
				host.PluginDir + "/skills/finish/SKILL.md",
			}, ", "),
			wantFix: new("chmod u+rwx ., then " + runInitClaudeCode),
			wantRel: host.PluginDir,
		},
	})
}

func Test_diagnose_classifies_host_hook_unreadable(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			// An unreadable hook file is WARN "not readable", never the
			// ERROR "not a regular file" a genuine wrong-shape hook gets.
			name: "the hook file is unreadable, not wrong-shaped",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				chmodUnreadableDir(t, filepath.Join(wd, ".claude"))
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied)",
			wantFix:      new("chmod u+rwx .claude, then " + runInitClaudeCode),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			// The unreadable-directory case above fails at the Lstat
			// call itself; this one chmods the hook file directly, so
			// the failure is in the ReadFile call instead — the fix must
			// target the file (chmod +r), not its parent directory.
			name: "the hook file itself is unreadable, its directory is searchable",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				var hookPath string

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)

					if f.Hook {
						hookPath = filepath.Join(wd, filepath.FromSlash(f.RelPath))
					}
				}

				chmodUnreadable(t, hookPath)
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied)",
			wantFix:      new("chmod +r " + host.PluginDir + "/hooks/hooks.json, then " + runInitClaudeCode),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
	})
}

func Test_diagnose_classifies_host_agents_unreadable(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			// An unreadable agent file is WARN "not readable", never the
			// "missing" wording a genuinely absent one gets.
			name: "an agent file is unreadable, not missing",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Agents() {
					writeHostArtifact(t, wd, f)
				}

				chmodUnreadableDir(t, filepath.Join(wd, ".claude"))
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail: "not readable (permission denied): " + strings.Join([]string{
				host.PluginDir + "/agents/planner.md",
				host.PluginDir + "/agents/implementer.md",
				host.PluginDir + "/agents/reviewer.md",
			}, ", "),
			wantFix: new("chmod u+rwx .claude, then " + runInitClaudeCode),
			wantRel: host.PluginDir + "/agents",
		},
		{
			// Every directory stays searchable; only the planner agent
			// file is chmodded 0o000, so the failure is in the ReadFile
			// call rather than the Lstat call.
			name: "an agent file itself is unreadable, its directory is searchable",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				var plannerPath string

				for _, f := range h.Agents() {
					writeHostArtifact(t, wd, f)

					if f.Kind == artifact.KindAgentPlanner {
						plannerPath = filepath.Join(wd, filepath.FromSlash(f.RelPath))
					}
				}

				chmodUnreadable(t, plannerPath)
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied): " + host.PluginDir + "/agents/planner.md",
			wantFix:      new("chmod +r " + host.PluginDir + "/agents/planner.md, then " + runInitClaudeCode),
			wantRel:      host.PluginDir + "/agents",
		},
	})
}

func Test_diagnose_classifies_host_skill_unreadable(t *testing.T) {
	skillPath := host.WorkflowSkillDir + "/SKILL.md"

	runHostCheckCases(t, []hostCheckCase{
		{
			name: "skill mode 0o000 beside the plugin",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				var skillFile host.File

				for _, f := range h.Skills() {
					writeHostArtifact(t, wd, f)
					skillFile = f
				}

				chmodUnreadable(t, filepath.Join(wd, filepath.FromSlash(skillFile.RelPath)))
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied)",
			wantFix:      new("chmod +r " + skillPath + ", then " + runInitClaudeCode),
			wantRel:      skillPath,
		},
	})
}

// Diagnose run from a subdirectory below root must recommend
// "chmod u+rwx ../<dir>", not the bare root-relative form every
// hostCheckCase table in this file pins.
func Test_diagnose_host_plugin_hook_agents_unreadable_fix_is_relative_to_wd(t *testing.T) {
	wd := newHostFixture(t)
	h := claudeCodeHost(t)

	for _, f := range h.Plugin(true) {
		writeHostArtifact(t, wd, f)
	}

	for _, f := range h.Agents() {
		writeHostArtifact(t, wd, f)
	}

	chmodUnreadableDir(t, filepath.Join(wd, ".claude"))

	subdir := filepath.Join(wd, "docs")

	srv := doctor.NewServer(emptyHomeDir(t))
	report := srv.Diagnose(t.Context(), subdir)

	pluginCheck := findCheck(t, report, "host-plugin")
	assert.Equal(t, doctor.SeverityError, pluginCheck.Severity)
	require.NotNil(t, pluginCheck.Fix)
	assert.Equal(t, "chmod u+rwx ../.claude, then "+runInitClaudeCode, *pluginCheck.Fix)

	hookCheck := findCheck(t, report, "host-hook")
	assert.Equal(t, doctor.SeverityWarn, hookCheck.Severity)
	require.NotNil(t, hookCheck.Fix)
	assert.Equal(t, "chmod u+rwx ../.claude, then "+runInitClaudeCode, *hookCheck.Fix)

	agentsCheck := findCheck(t, report, "host-agents")
	assert.Equal(t, doctor.SeverityWarn, agentsCheck.Severity)
	require.NotNil(t, agentsCheck.Fix)
	assert.Equal(t, "chmod u+rwx ../.claude, then "+runInitClaudeCode, *agentsCheck.Fix)
}

// Only ".claude-plugin" (holding plugin.json) is chmodded unreadable
// here; "skills/start" and "skills/finish" are never created, so their
// own SKILL.md files read as plain-missing, not unreadable.
func Test_diagnose_host_plugin_detail_names_unreadable_and_missing_together(t *testing.T) {
	wd := newHostFixture(t)
	h := claudeCodeHost(t)

	for _, f := range h.Plugin(true) {
		if f.Kind == artifact.KindPluginManifest {
			writeHostArtifact(t, wd, f)
		}
	}

	chmodUnreadableDir(t, filepath.Join(wd, host.PluginDir, ".claude-plugin"))

	srv := doctor.NewServer(emptyHomeDir(t))
	report := srv.Diagnose(t.Context(), wd)

	check := findCheck(t, report, "host-plugin")
	assert.Equal(t, doctor.SeverityError, check.Severity)
	assert.Contains(t, check.Detail, "not readable (permission denied): "+host.PluginDir+"/.claude-plugin/plugin.json")
	assert.Contains(t, check.Detail, "; missing "+host.PluginDir+"/skills/start/SKILL.md, "+host.PluginDir+"/skills/finish/SKILL.md")
}

func Test_diagnose_classifies_host_snippet_unreadable(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name: "CLAUDE.md exists but is not readable",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				path := filepath.Join(wd, "CLAUDE.md")
				require.NoError(t, os.WriteFile(path, []byte("unrelated prose\n"), 0o600))
				chmodUnreadable(t, path)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied); cannot check for brief block",
			wantFix:      new("chmod +r CLAUDE.md, then " + runInitClaudeCode),
			wantRel:      "CLAUDE.md",
		},
		{
			// An ancestor directory doctor cannot even Lstat into is
			// unreadable at the Lstat call itself, not the ReadFile
			// call — the fix must target the broken directory, not a
			// "chmod +r" on a file it never reached.
			name: "no root CLAUDE.md, .claude itself cannot be Lstat'd",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				claudeDir := filepath.Join(wd, ".claude")
				require.NoError(t, os.MkdirAll(claudeDir, 0o755))
				chmodUnreadableDir(t, claudeDir)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not readable (permission denied); cannot check for brief block",
			wantFix:      new("chmod u+rwx .claude, then " + runInitClaudeCode),
			wantRel:      filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			// Mirrors host-plugin's own ENOTDIR case — a ".claude" that
			// is a regular file proves absence, never
			// present-but-unreadable.
			name: "the .claude root is a regular file, not a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      "CLAUDE.md",
		},
		{
			// brief cannot tell whether root's own CLAUDE.md carries a
			// block, but .claude/CLAUDE.md's real block still wins.
			name: "CLAUDE.md is not readable but .claude/CLAUDE.md holds a real block",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				path := filepath.Join(wd, "CLAUDE.md")
				require.NoError(t, os.WriteFile(path, []byte("unrelated prose\n"), 0o600))
				chmodUnreadable(t, path)

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "CLAUDE.md"), block, 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      filepath.Join(".claude", "CLAUDE.md"),
		},
	})
}

// host-snippet's own "not readable" WARN must render its fix command
// relative to the same working directory as the row's own Path, never
// relative to the install root.
func Test_diagnose_host_snippet_unreadable_fix_is_relative_to_wd(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, subdir string)
	}{
		{
			name:  "no CLAUDE.md in the subdirectory",
			setup: func(t *testing.T, _ string) { t.Helper() },
		},
		{
			name: "the subdirectory holds its own unrelated CLAUDE.md",
			setup: func(t *testing.T, subdir string) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(subdir, "CLAUDE.md"), []byte("decoy\n"), 0o600))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHostFixture(t)

			claudeMD := filepath.Join(wd, "CLAUDE.md")
			require.NoError(t, os.WriteFile(claudeMD, []byte("unrelated prose\n"), 0o600))
			chmodUnreadable(t, claudeMD)

			subdir := filepath.Join(wd, "docs")
			c.setup(t, subdir)

			srv := doctor.NewServer(emptyHomeDir(t))
			report := srv.Diagnose(t.Context(), subdir)

			check := findCheck(t, report, "host-snippet")
			assert.Equal(t, doctor.SeverityWarn, check.Severity)
			assert.Contains(t, check.Detail, "not readable (permission denied); cannot check for brief block")
			assert.Equal(t, claudeMD, check.Path)
			require.NotNil(t, check.Fix)
			assert.Equal(t, "chmod +r ../CLAUDE.md, then "+runInitClaudeCode, *check.Fix)
		})
	}
}
