package doctor_test

// OS-subject host-integration tests: every case here is on real disk
// (t.TempDir via newHostFixture), because its own subject is an OS error
// shape classifyProbeError has to discriminate — a chmod'd file or
// directory (present-but-unreadable, at both the Lstat and the ReadFile
// call), or an ancestor path component that is itself a regular file
// (ENOTDIR). host_test.go holds every case whose own subject is the
// classification rule itself (missing, current, older, edited,
// not-a-regular-file, marker states, the roles duplicate-definition WARN,
// and the roles/roles-skill resolution tables), run against an in-memory
// fstest.MapFS instead, through (*Server).projectTree and WithHomeTree.

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

// newHostFixture builds a wd with a valid ".brief.yaml" (unbound roles) and
// its default feature root, but no Claude Code integration installed — the
// baseline every case in this file starts from, adding exactly the files
// its own scenario needs. Fixtures in this file are built from
// internal/platform/artifact renders directly, never through internal/setup.
func newHostFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: \"## Progress\"\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

	return wd
}

// writeHostArtifact writes f's own current artifact.Render at wd/f.RelPath.
func writeHostArtifact(t *testing.T, wd string, f host.File) {
	t.Helper()

	writeHostFile(t, wd, f.RelPath, artifact.Render(f.Kind))
}

// hostCheckCase is one row of a host-check classification table: setup
// mutates newHostFixture's own bare baseline, and Diagnose's report must
// carry checkID at wantSeverity, with wantDetail a substring of Detail,
// wantFix the exact Fix text (nil when the row carries none), and wantRel
// the row's own Path exactly, relative to wd (newHostFixture's own root)
// — mandatory, mirroring host_test.go's own hostFSCheckCase: every case in
// this file has a deterministic Path (host.PluginDir, the hook/agents
// path, skillPath, or one of the two CLAUDE.md candidates), so an exact
// assertion is always the stronger check, never a suffix guess.
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
// Diagnose with an injected empty home, and asserts c.checkID's own row
// against c.wantSeverity, c.wantDetail (substring), c.wantFix (exact) and
// c.wantRel (the row's own Path, exactly, relative to wd).
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
// restores it to 0o600 before TempDir's own removal runs — an unreadable
// file left at 0o000 would otherwise make RemoveAll fail on some
// platforms. Skips the test outright under euid 0, where chmod's
// permission bits have no effect and the read would silently succeed,
// which would make the case pass vacuously rather than exercise the WARN
// this file pins.
func chmodUnreadable(t *testing.T, path string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
}

// chmodUnreadableDir chmods dir to 0o000 and registers a t.Cleanup that
// restores it to 0o755 before TempDir's own removal runs — an inaccessible
// directory left at 0o000 (no execute/search bit) would otherwise make
// RemoveAll unable to traverse into it at all, unlike chmodUnreadable's own
// 0o600 restore, which is only safe for a plain file. Skips under euid 0,
// the same as chmodUnreadable, where chmod's permission bits have no
// effect and every Lstat underneath would silently succeed.
func chmodUnreadableDir(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// Test_diagnose_reports_not_installed_when_the_claude_root_is_a_regular_file
// pins P1 across the three rows a ".claude" ancestor that is itself a
// regular file, not a directory, affects: os.Lstat fails with ENOTDIR,
// which classifyProbeError's own ENOTDIR arm proves is absence, the same
// "not installed" every genuinely missing ".claude" gets, never
// present-but-unreadable. This stays on real disk rather than
// fstest.MapFS: a direct probe against the stdlib (fs.Lstat over an
// equivalent fstest.MapFS fixture) reports plain fs.ErrNotExist here, not
// ENOTDIR, so a MapFS fixture cannot exercise classifyProbeError's own
// ENOTDIR arm specifically — only a real filesystem's own directory-walk
// semantics produce it. Mutation-verified, across ./internal/doctor/...
// and ./internal/cli/... with no -run filter: dropping the ENOTDIR arm
// (`|| errors.Is(err, syscall.ENOTDIR)`) reddens all three cases here —
// each subject file is now present-but-unreadable rather than absent, so
// host-plugin becomes ERROR "not readable (not a directory): …" and
// host-hook/host-agents become WARN "not readable (not a directory)" —
// plus host_disk_test.go's own
// Test_diagnose_classifies_host_snippet_unreadable/"the .claude root is a
// regular file, not a directory" and internal/cli's own
// Test_doctor_reports_every_host_row_skip_when_dot_claude_is_a_regular_file,
// restored after.
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

// Test_diagnose_classifies_host_plugin_unreadable pins host-plugin's own
// unreadable-vs-missing split (P1): an ancestor directory doctor cannot even
// Lstat into (mode 0o000) is unreadable, never "missing" — every subject
// file keeps its own "not readable" name rather than the "incomplete:
// missing" wording a genuinely absent file gets, both ERROR, since Claude
// Code cannot load the skill through a file it cannot read any more than
// one that is not there. The fix targets whichever call actually failed: a
// failed Lstat targets blockingDir's own result, the directory missing its
// search bit, never necessarily the subject's immediate parent; a failed
// ReadFile targets the subject file itself (chmod +r).
func Test_diagnose_classifies_host_plugin_unreadable(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			// Control for every case below: the identical install, fully
			// readable, OK. Also pinned, MapFS-backed, by
			// Test_diagnose_classifies_host_plugin's own "every subject file
			// is current" case in host_test.go.
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
			// Control: "every subject file is current" above is the
			// identical install, readable, OK. Mutation-verified,
			// package-wide with no -run filter: dropping
			// integrationFileRowDetail's own call ahead of
			// missingRelPaths (forcing its own `ok` result to false) turns
			// this case OK "installed" — every file is unreadable, so
			// missingRelPaths itself finds nothing left to call missing —
			// reddening every case and test in the package whose own
			// subject includes an unreadable host-plugin file: this case,
			// "the blocking dir is .claude/skills, .claude itself stays
			// 0755" and "a subject file itself is unreadable, its
			// directory is searchable" and "the install root itself is
			// unsearchable" below,
			// Test_diagnose_host_plugin_hook_agents_unreadable_fix_is_relative_to_wd,
			// Test_diagnose_host_plugin_detail_names_unreadable_and_missing_together,
			// and internal/cli's own
			// Test_doctor_env_path_stays_error_when_the_host_snippet_directory_is_unreadable
			// and
			// Test_doctor_reports_host_plugin_error_when_the_plugin_directory_is_unreadable.
			// Restored after.
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
			// (".claude/skills"), not root's own immediate child
			// (".claude", left at 0o755): notReadableFix's own ancestor
			// walk must climb past every unresolvable descendant and stop
			// at the first ancestor whose own Lstat succeeds, not at the
			// subject file's immediate parent and not at root. Control:
			// "a subject file is unreadable, not missing" above chmods
			// ".claude" itself and expects the walk to stop one level
			// higher still. Mutation-verified against this package and
			// internal/cli, with no -run filter: hardcoding blockingDir to
			// always return root's immediate child segment of the
			// resolved ancestor (truncating any deeper walk back down to
			// ".claude") reddens this case alone package-wide, since every
			// other case exercising the walk already resolves to that
			// immediate child or, for "the install root itself is
			// unsearchable" below, to root itself regardless. Checking
			// each ancestor's own parent instead of the ancestor itself
			// reddens this case together with "a subject file is
			// unreadable, not missing" above, host-hook's and
			// host-agents' own unreadable-ancestor cases, "the install
			// root itself is unsearchable" below, and
			// Test_diagnose_host_plugin_hook_agents_unreadable_fix_is_relative_to_wd.
			// Disabling the loop's success branch entirely, so it never
			// returns before dir == root, reddens the same set except
			// "the install root itself is unsearchable" — whose own
			// correct answer already is root — and additionally reddens
			// host-snippet's own "no root CLAUDE.md, .claude itself
			// cannot be Lstat'd" case. Both restored after.
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
			// itself is chmodded 0o000, so the failure is in the ReadFile
			// call rather than the Lstat call (statFailed is false) —
			// mirrors host-hook's own "the hook file itself is unreadable,
			// its directory is searchable" case. Control: "every subject
			// file is current" above is the identical install, readable,
			// OK. Mutation-verified, package-wide with no -run filter:
			// hardcoding integrationFileRowDetail's own notReadableFix call
			// to pass true for statFailed (rather than first.statFailed)
			// reddens this case together with
			// Test_diagnose_classifies_host_agents_unreadable's own "an
			// agent file itself is unreadable, its directory is searchable"
			// — the two rows sharing integrationFileRowDetail — both fixes
			// reverting to their own "chmod u+rwx <dir>, then …" form,
			// restored after.
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
			// directory, not any of its descendants: blockingDir's own
			// walk never finds a resolvable ancestor before dir == root,
			// so it falls through to its own "return root" fallback
			// rather than the loop's success branch the two chmodded-
			// directory cases above ("a subject file is unreadable, not
			// missing" and "the blocking dir is .claude/skills") reach.
			// Control: "a subject file is unreadable, not missing" above
			// leaves root itself searchable and stops one level lower, at
			// ".claude". Mutation-verified against this package and
			// internal/cli, with no -run filter: changing that fallback
			// to "return filepath.Dir(root)" reddens this case alone,
			// package-wide; the exact resulting fix text was not asserted
			// against, only that it stops matching "chmod u+rwx ., then
			// …", restored after.
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

// Test_diagnose_classifies_host_hook_unreadable pins host-hook's own
// unreadable-vs-wrong-shaped split (P1). Control for both cases: "the hook
// file is current" in host_test.go's own Test_diagnose_classifies_host_hook
// is the identical install, readable, OK.
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
			// The unreadable-directory case above fails at the Lstat call
			// itself (statFailed); this one leaves every directory
			// searchable and chmods the hook file directly, so the failure
			// is in the ReadFile call instead — the fix must target the
			// file (chmod +r), not its parent directory. Mutation-verified,
			// package-wide with no -run filter: dropping probeIntegrationFile's
			// own `state.unreadable = true` in its ReadFile-failure arm
			// reddens this case together with every other row that probes
			// a single subject file's own 0o000 bytes through the same
			// function: Test_diagnose_classifies_host_plugin_unreadable's
			// own "a subject file itself is unreadable, its directory is
			// searchable", Test_diagnose_classifies_host_agents_unreadable's
			// own "an agent file itself is unreadable, its directory is
			// searchable", and Test_diagnose_classifies_host_skill_unreadable's
			// own "skill mode 0o000 beside the plugin" — restored after.
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

// Test_diagnose_classifies_host_agents_unreadable pins host-agents' own
// unreadable-vs-missing split (P1). Control for both cases: "all three
// agent files are current" in host_test.go's own
// Test_diagnose_classifies_host_agents is the identical install, readable,
// OK.
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
			// file itself is chmodded 0o000, so the failure is in the
			// ReadFile call rather than the Lstat call (statFailed is
			// false) — mirrors host-plugin's own equivalent case. See
			// Test_diagnose_classifies_host_plugin_unreadable's own "a
			// subject file itself is unreadable, its directory is
			// searchable" for the shared integrationFileRowDetail
			// mutation both cases redden together.
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

// Test_diagnose_classifies_host_skill_unreadable pins host-skill's own
// unreadable arm (P1): a skill file chmod'd unreadable, installed beside a
// full plugin install, is WARN "not readable", never the ERROR
// "not a regular file" a genuine wrong-shape skill gets. Control: "the
// skill is current, alongside a full install" in host_test.go's own
// Test_diagnose_classifies_host_skill is the identical install, readable,
// OK.
func Test_diagnose_classifies_host_skill_unreadable(t *testing.T) {
	skillPath := host.WorkflowSkillDir + "/SKILL.md"

	runHostCheckCases(t, []hostCheckCase{
		{
			// See Test_diagnose_classifies_host_hook_unreadable's own "the
			// hook file itself is unreadable, its directory is searchable"
			// for the probeIntegrationFile mutation this case reddens
			// alongside.
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

// Test_diagnose_host_plugin_hook_agents_unreadable_fix_is_relative_to_wd
// pins the same wd-vs-root split fix pass 8 gave host-snippet's own "not
// readable" fix (Test_diagnose_host_snippet_unreadable_fix_is_relative_to_wd)
// for host-plugin, host-hook and host-agents: Diagnose run from a
// subdirectory below root must recommend "chmod u+rwx ../<dir>", not the
// bare root-relative form every hostCheckCase table in this file pins
// (wd == root there). Mutation-verified: passing root instead of absWd as
// wd into hostPluginCheck/hostHookCheck/hostAgentsCheck in doctor.go's own
// Diagnose reddens all three assertions here — the fix text stops
// changing between wd == root and wd != root — while leaving every table
// in this file green.
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

// Test_diagnose_host_plugin_detail_names_unreadable_and_missing_together
// pins integrationFileRowDetail's own mixed-row wording: a host-plugin row
// spanning one unreadable subject file and one genuinely missing sibling
// must name both, in separate fragments, never silently drop the missing
// one behind the unreadable one's own "not readable" wording. Only
// ".claude-plugin" (holding plugin.json) is chmodded unreadable here;
// "skills/start" and "skills/finish" are never created at all, so their
// own SKILL.md files read as plain-missing, not unreadable.
// Mutation-verified: deleting integrationFileRowDetail's own "; missing …"
// append reddens this case alone — the fragment disappears from Detail —
// restored after.
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

// Test_diagnose_classifies_host_snippet_unreadable pins host-snippet's own
// unreadable-vs-absent split (P1): a candidate that exists, is regular,
// but could not be read is WARN naming the underlying reason, never the
// "not installed" a genuinely absent candidate gets; an ancestor
// directory doctor cannot even Lstat into is WARN too, its fix targeting
// that directory rather than a file it never reached; an ancestor path
// component that is itself a regular file (ENOTDIR) proves absence
// instead, the same "not installed" a genuinely missing CLAUDE.md gets;
// either unreadable arm yields to the other candidate's own real block,
// which wins exactly as it would against a notRegular candidate.
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
			// P1: an ancestor directory doctor cannot even Lstat into (mode
			// 0o000) is unreadable at the Lstat call itself, not the
			// ReadFile call — the fix must target the broken directory
			// (chmod u+rwx, the search bit to read through it again and
			// the write bit init needs to create entries under it), not a
			// "chmod +r" on a file it never reached. Mutation-verified:
			// hardcoding statFailed to false in notReadableFix's own caller
			// reddens this case alone (the fix text reverts to "chmod +r
			// .claude/CLAUDE.md, then …"), restored after.
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
			// P1: mirrors host-plugin's own ENOTDIR case — a ".claude" that
			// is a regular file proves absence (both candidates read as
			// absent), never present-but-unreadable.
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
			// Pins the block-wins carve-out against an unreadable root
			// candidate specifically (P1): brief cannot tell whether root's
			// own CLAUDE.md carries a block, but .claude/CLAUDE.md's own
			// real block still wins, exactly as it does against a
			// notRegular root candidate in host_test.go's own
			// Test_diagnose_classifies_host_snippet.
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

// Test_diagnose_host_snippet_unreadable_fix_is_relative_to_wd pins fix pass
// 8's M1 fix: host-snippet's own "not readable" WARN must render its fix
// command relative to the same working directory as the row's own Path
// (cli.doctorRow's own displayPath(wd, ...)), never relative to the
// install root. Diagnose run from a subdirectory below root must recommend
// "chmod +r ../CLAUDE.md", not the bare root-relative "chmod +r CLAUDE.md"
// the pre-fix code always rendered: run from that subdirectory, the bare
// form either fails outright (no CLAUDE.md there) or — the second case
// here — silently chmods an unrelated file the caller happens to have,
// leaving the WARN in place. Mutation-verified, package-wide with no -run
// filter: passing root instead of absWd as hostSnippetCheck's own wd
// argument in doctor.go's own Diagnose reddens both cases here — the fix
// text stops changing between wd == root and wd != root — while leaving
// every case in host_test.go's own Test_diagnose_classifies_host_snippet
// (wd == root there, so the two relativizations coincide) green.
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
