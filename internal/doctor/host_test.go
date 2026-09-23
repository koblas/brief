package doctor_test

import (
	"fmt"
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

// claudeCodeHost returns the claude-code Host every fixture in this file
// builds files for.
func claudeCodeHost(t *testing.T) host.Host {
	t.Helper()

	h, ok := host.Lookup(host.ClaudeCode)
	require.True(t, ok)

	return h
}

// writeHostArtifact writes f's own current artifact.Render at wd/f.RelPath.
func writeHostArtifact(t *testing.T, wd string, f host.File) {
	t.Helper()

	writeHostFile(t, wd, f.RelPath, artifact.Render(f.Kind))
}

// writeHostDir creates a directory at wd/relPath, standing in for an
// integration file a repository owner replaced with a directory.
func writeHostDir(t *testing.T, wd, relPath string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(wd, filepath.FromSlash(relPath)), 0o755))
}

// hostCheckCase is one row of a host-check classification table: setup
// mutates newHostFixture's own bare baseline, and Diagnose's report must
// carry checkID at wantSeverity, with wantDetail a substring of Detail and
// wantFix the exact Fix text (nil when the row carries none). wantPathSuffix,
// when non-empty, is asserted as a suffix of the row's own Path — most
// cases only need severity/detail/fix, but a row whose Path names one of
// two candidates (host-snippet's own CLAUDE.md/​.claude/CLAUDE.md choice)
// needs the stronger check.
type hostCheckCase struct {
	name              string
	setup             func(t *testing.T, wd string, h host.Host)
	checkID           string
	wantSeverity      doctor.Severity
	wantDetail        string
	wantFix           *string
	wantPathSuffix    string
	wantPathNotSuffix string
}

// runHostCheckCases builds newHostFixture(t), applies c.setup, runs
// Diagnose with an injected empty home, and asserts c.checkID's own row
// against c.wantSeverity, c.wantDetail (substring), c.wantFix (exact) and,
// when set, c.wantPathSuffix.
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

			if c.wantPathSuffix != "" {
				assert.True(t, strings.HasSuffix(check.Path, c.wantPathSuffix), "path %q must end with %q", check.Path, c.wantPathSuffix)
			}

			if c.wantPathNotSuffix != "" {
				assert.False(t, strings.HasSuffix(check.Path, c.wantPathNotSuffix), "path %q must not end with %q", check.Path, c.wantPathNotSuffix)
			}
		})
	}
}

// runInit is short for "run 'brief init'" — the fix text repeated across
// host-plugin, host-hook, host-snippet and host-agents rows whenever a
// plain re-run would repair the finding.
const runInit = "run 'brief init'"

// runInitClaudeCode is the SKIP fix every "not installed" host row shares.
const runInitClaudeCode = "run 'brief init --host claude-code'"

// runInitWithAgents is the fix host-agents and roles share whenever the
// remedy needs --with-agents specifically.
const runInitWithAgents = "run 'brief init --with-agents'"

// Test_diagnose_classifies_host_plugin pins host-plugin's own precedence:
// nothing installed anywhere is SKIP; once something is installed
// (Plugin(true) ∪ Agents()), a missing or non-regular subject file
// (Plugin(false)) is ERROR "incomplete", naming it; an edited one is OK
// "edited locally"; every subject file current is OK "installed".
// Mutation-verified: disabling hostPluginCheck's own missing-file
// precedence branch reddens exactly the three "missing"/"is a directory"
// cases here — never "edited locally" or "every subject file is
// current" — proving this table actually discriminates on it rather than
// passing regardless.
func Test_diagnose_classifies_host_plugin(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name:         "nothing installed anywhere",
			setup:        func(t *testing.T, _ string, _ host.Host) { t.Helper() },
			checkID:      "host-plugin",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
		},
		{
			name: "manifest missing while the rest of the plugin is installed",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					if f.Kind == artifact.KindPluginManifest {
						continue
					}

					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/.claude-plugin/plugin.json",
			wantFix:      new(runInit),
		},
		{
			name: "a skill file is a directory",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					if f.Kind == artifact.KindSkillStart {
						writeHostDir(t, wd, f.RelPath)

						continue
					}

					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/skills/start/SKILL.md",
			wantFix:      new(runInit),
		},
		{
			name: "only the hook is installed",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					if f.Hook {
						writeHostArtifact(t, wd, f)
					}
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/.claude-plugin/plugin.json, .claude/skills/brief/skills/start/SKILL.md, .claude/skills/brief/skills/finish/SKILL.md",
			wantFix:      new(runInit),
		},
		{
			name: "a subject file was edited locally",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				writeHostFile(t, wd, host.PluginDir+"/.claude-plugin/plugin.json", []byte(`{"name": "brief", "custom": true}`))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally: .claude/skills/brief/.claude-plugin/plugin.json",
			wantFix:      nil,
		},
		{
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
		},
		{
			// P1: an ancestor path component that is a regular file, not a
			// directory, proves absence (os.Lstat fails with ENOTDIR) — the
			// same "not installed" every genuinely missing ".claude" gets,
			// never present-but-unreadable. Mutation-verified: reverting
			// classifyProbeError to only recognize fs.ErrNotExist (dropping
			// the ENOTDIR arm) reddens this case alone into ERROR
			// "incomplete: missing …", restored after.
			name: "the .claude root is a regular file, not a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
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
			// readable, OK.
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
		},
		{
			// Control: "every subject file is current" above is the
			// identical install, readable, OK. Mutation-verified:
			// dropping integrationFileRowDetail's own
			// call ahead of missingRelPaths turns this case OK
			// "installed" — every file is unreadable, so missingRelPaths
			// itself finds nothing left to call missing — reddening this
			// case and every other case or test that depends on
			// host-plugin's own unreadable arm, restored after.
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
		},
		{
			// Every directory stays searchable; only the manifest file
			// itself is chmodded 0o000, so the failure is in the ReadFile
			// call rather than the Lstat call (statFailed is false) —
			// mirrors host-hook's own "the hook file itself is unreadable,
			// its directory is searchable" case. Control: "every subject
			// file is current" above is the identical install, readable,
			// OK. Mutation-verified: hardcoding first.statFailed to true
			// in integrationFileRowDetail's own notReadableFix call
			// reddens this case alone (the fix reverts to "chmod u+rwx
			// .claude/skills/brief/.claude-plugin, then …"), restored
			// after.
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
		},
	})
}

// writeHostPluginWithoutHook writes every h.Plugin(true) file at wd except
// the trailing hook entry.
func writeHostPluginWithoutHook(t *testing.T, wd string, h host.Host) {
	t.Helper()

	for _, f := range h.Plugin(true) {
		if f.Hook {
			continue
		}

		writeHostArtifact(t, wd, f)
	}
}

// Test_diagnose_classifies_host_hook pins host-hook's own precedence:
// nothing installed anywhere is SKIP; the rest of the plugin installed
// with no hook file is WARN, since doctor cannot tell a lost file from
// --no-hook; a hook path that is a directory is ERROR; an edited hook is
// OK "edited locally"; a current hook is OK "installed".
func Test_diagnose_classifies_host_hook(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name:         "nothing installed anywhere",
			setup:        func(t *testing.T, _ string, _ host.Host) { t.Helper() },
			checkID:      "host-hook",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
		},
		{
			name: "the rest of the plugin is installed but the hook file is missing",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				writeHostPluginWithoutHook(t, wd, h)
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "installed without the check hook",
			wantFix:      new("run 'brief init' to add it"),
		},
		{
			name: "the hook path is a directory",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				writeHostPluginWithoutHook(t, wd, h)

				for _, f := range h.Plugin(true) {
					if f.Hook {
						writeHostDir(t, wd, f.RelPath)
					}
				}
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "not a regular file",
			wantFix:      new(runInit),
		},
		{
			name: "the hook file was edited locally",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}

				writeHostFile(t, wd, host.PluginDir+"/hooks/hooks.json", []byte(`{"hooks": {}}`))
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally",
			wantFix:      nil,
		},
		{
			name: "the hook file is current",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Plugin(true) {
					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
		},
		{
			// P1: mirrors host-plugin's own ENOTDIR case — a ".claude" that
			// is a regular file proves absence, never present-but-unreadable.
			name: "the .claude root is a regular file, not a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
		},
		{
			// An unreadable hook file is WARN "not readable", never the
			// ERROR "not a regular file" a genuine wrong-shape hook gets.
			// Control: "the hook file is current" above is the identical
			// install, readable, OK.
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
		},
		{
			// The unreadable-directory case above fails at the Lstat call
			// itself (statFailed); this one leaves every directory
			// searchable and chmods the hook file directly, so the failure
			// is in the ReadFile call instead — the fix must target the
			// file (chmod +r), not its parent directory.
			// Mutation-verified: dropping probeIntegrationFile's own
			// `state.unreadable = true` in its ReadFile-failure arm reddens
			// this case alone (the row falls to ERROR "not a regular
			// file"), restored after.
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
		},
	})
}

// Test_diagnose_classifies_host_agents pins host-agents' own precedence,
// distinct from host-plugin's: none of the three agent files present is
// SKIP; a missing or non-regular one, given at least one of the three
// exists, is WARN (never ERROR — an unbound agent is not itself a fault);
// an older render is WARN naming it; an edited one is OK "edited locally";
// all three current is OK "installed". Every fix here names --with-agents,
// since a plain "brief init" never touches agent files. Mutation-verified:
// disabling hostAgentsCheck's own missing-file precedence branch reddens
// exactly "one of the three agent files is missing" here — never "edited
// locally" or "all three agent files are current".
func Test_diagnose_classifies_host_agents(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name:         "none of the three agent files present",
			setup:        func(t *testing.T, _ string, _ host.Host) { t.Helper() },
			checkID:      "host-agents",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitWithAgents),
		},
		{
			name: "one of the three agent files is missing",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Agents() {
					if f.Kind == artifact.KindAgentReviewer {
						continue
					}

					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "missing .claude/skills/brief/agents/reviewer.md",
			wantFix:      new(runInitWithAgents),
		},
		{
			// SCENARIO-02: the pre-scenario planner render, captured
			// mechanically (%q dump) before agents.go changed and now moved
			// to olderAgentPlannerDigests — the one fixture that actually
			// reaches host-agents' own OriginOlder arm today (every other
			// Kind's older…Digests list still ships empty). Mutation-verify
			// by emptying olderAgentPlannerDigests: this case alone reddens
			// (falls through to "edited locally"), the others above and
			// below stay green.
			name: "an older planner render",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Agents() {
					if f.Kind == artifact.KindAgentPlanner {
						continue
					}

					writeHostArtifact(t, wd, f)
				}

				older := []byte("---\nname: planner\ndescription: Turn a feature's specification into ordered scenario " +
					"plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's " +
					"specification into ordered scenario plans: run `brief new step <feature>` for the next " +
					"scenario, then fill its plan file. Never write production or test code.\n")
				writeHostFile(t, wd, host.PluginDir+"/agents/planner.md", older)
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "installed by an older brief release: .claude/skills/brief/agents/planner.md",
			wantFix:      new(runInitWithAgents),
		},
		{
			name: "an agent file was edited locally",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Agents() {
					writeHostArtifact(t, wd, f)
				}

				writeHostFile(t, wd, host.PluginDir+"/agents/planner.md", []byte("---\nname: planner\n---\n\ncustom\n"))
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally: .claude/skills/brief/agents/planner.md",
			wantFix:      nil,
		},
		{
			name: "all three agent files are current",
			setup: func(t *testing.T, wd string, h host.Host) {
				t.Helper()

				for _, f := range h.Agents() {
					writeHostArtifact(t, wd, f)
				}
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
		},
		{
			// P1: mirrors host-plugin's own ENOTDIR case — a ".claude" that
			// is a regular file proves absence, never present-but-unreadable
			// (which would otherwise flip this row's own SKIP to WARN).
			name: "the .claude root is a regular file, not a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitWithAgents),
		},
		{
			// An unreadable agent file is WARN "not readable", never the
			// "missing" wording a genuinely absent one gets. Control: "all
			// three agent files are current" above is the identical install,
			// readable, OK.
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
		},
		{
			// Every directory stays searchable; only the planner agent
			// file itself is chmodded 0o000, so the failure is in the
			// ReadFile call rather than the Lstat call (statFailed is
			// false) — mirrors host-plugin's own equivalent case. Control:
			// "all three agent files are current" above is the identical
			// install, readable, OK. Mutation-verified: hardcoding
			// first.statFailed to true in integrationFileRowDetail's own
			// notReadableFix call reddens this case alone (the fix
			// reverts to "chmod u+rwx .claude/skills/brief/agents,
			// then …"), restored after.
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

// Test_diagnose_classifies_host_snippet pins host-snippet's own rules: a
// marker defect in either CLAUDE.md candidate is ERROR, naming its own
// line; two candidates each holding a block is ERROR on ".claude/CLAUDE.md";
// a block only in ".claude/CLAUDE.md" is OK; a current block for a
// directory other than the configured one is WARN; a current block is OK
// but not dir-compared when ".brief.yaml" itself is unparseable; an edited
// block is OK "edited locally"; no CLAUDE.md at all, or a block whose line
// endings are CRLF (which can never exactly match the LF marker), is SKIP
// "not installed"; a candidate that exists but is not a regular file — a
// directory or a symlink — is WARN, naming which. The unreadable-vs-absent
// split — a candidate Lstat or ReadFile cannot resolve, or an ancestor
// path component that is itself a regular file — is pinned separately in
// Test_diagnose_classifies_host_snippet_unreadable.
func Test_diagnose_classifies_host_snippet(t *testing.T) {
	runHostCheckCases(t, []hostCheckCase{
		{
			name: "a lone begin marker",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("intro\n"+artifact.SnippetBegin+"\nno end after this\n"), 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "line 2",
			wantFix:      new("add " + artifact.SnippetEnd + " after it, or remove the lone marker"),
		},
		{
			name: "a block exists in both candidates",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), block, 0o600))
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "CLAUDE.md"), block, 0o600))
			},
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityError,
			wantDetail:     "a brief block already exists in CLAUDE.md",
			wantFix:        new("delete that block"),
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			name: "a block only in .claude/CLAUDE.md",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "CLAUDE.md"), block, 0o600))
			},
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityOK,
			wantDetail:     "installed",
			wantFix:        nil,
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			name: "a current block names a different feature directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				block := append(append([]byte{}, artifact.SnippetBlock("elsewhere")...), '\n')
				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), block, 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "names elsewhere/, .brief.yaml says docs/specifications/",
			wantFix:      new(runInit),
		},
		{
			name: "the config is unparseable so the directory is not compared",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), block, 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed (feature directory not compared: .brief.yaml did not parse)",
			wantFix:      nil,
		},
		{
			name: "the block was edited locally",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				body := artifact.SnippetBegin + "\ncustom prose\n" + artifact.SnippetEnd + "\n"
				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte(body), 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally",
			wantFix:      nil,
		},
		{
			name:         "no CLAUDE.md anywhere",
			setup:        func(t *testing.T, _ string, _ host.Host) { t.Helper() },
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
		},
		{
			name: "CLAUDE.md is a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				writeHostDir(t, wd, "CLAUDE.md")
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not a regular file (directory); brief block not installed",
			wantFix:      new("run 'brief init --print' and add the CLAUDE.md block by hand"),
		},
		{
			name: "CLAUDE.md is a symlink",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				elsewhere := filepath.Join(wd, "elsewhere.md")
				require.NoError(t, os.WriteFile(elsewhere, []byte("elsewhere"), 0o600))
				require.NoError(t, os.Symlink(elsewhere, filepath.Join(wd, "CLAUDE.md")))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not a regular file (symlink); brief block not installed",
			wantFix:      new("run 'brief init --print' and add the CLAUDE.md block by hand"),
		},
		{
			name: "CLAUDE.md is a symlink but .claude/CLAUDE.md holds a real block",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				elsewhere := filepath.Join(wd, "elsewhere.md")
				require.NoError(t, os.WriteFile(elsewhere, []byte("elsewhere"), 0o600))
				require.NoError(t, os.Symlink(elsewhere, filepath.Join(wd, "CLAUDE.md")))

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "CLAUDE.md"), block, 0o600))
			},
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityOK,
			wantDetail:     "installed",
			wantFix:        nil,
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			// Mutation-verified: hardcoding states[0] instead of looping
			// (`if states[0].notRegular` in place of the `for` loop) turns
			// this WARN into the fallback SKIP "not installed" — reddened
			// by this case alone, restored after.
			name: "no root CLAUDE.md but .claude/CLAUDE.md is a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				writeHostDir(t, wd, filepath.Join(".claude", "CLAUDE.md"))
			},
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityWarn,
			wantDetail:     "not a regular file (directory); brief block not installed",
			wantFix:        new("run 'brief init --print' and add the CLAUDE.md block by hand"),
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			// Pins C1's fix: planSnippet would choose root CLAUDE.md here
			// (chooseSnippetLocation's own "first candidate that exists at
			// all" rule) and merge into it, never touching the directory at
			// ".claude/CLAUDE.md" — doctor must agree, not WARN about a
			// candidate init would never look at. Mutation-verified twice:
			// (1) reverting to scanning every state for notRegular (the
			// pre-C1 shape) turns this SKIP into the WARN case above's own
			// detail; (2) wantPathNotSuffix itself — a plain "CLAUDE.md"
			// suffix assertion here would pass vacuously against either
			// candidate's own path, so it must be the stronger negative
			// check: replacing the SKIP row's own Path selection with
			// states[len(states)-1].path (always the last candidate) reddens
			// this case alone via wantPathNotSuffix, leaving the sibling case
			// below — where the last candidate is also the first-present one
			// — green for the wrong reason. Each reddened, restored after.
			name: "root CLAUDE.md exists with no block, .claude/CLAUDE.md is a directory",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("unrelated prose\n"), 0o600))
				writeHostDir(t, wd, filepath.Join(".claude", "CLAUDE.md"))
			},
			checkID:           "host-snippet",
			wantSeverity:      doctor.SeveritySkip,
			wantDetail:        "not installed",
			wantFix:           new(runInitClaudeCode),
			wantPathNotSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			// Mutation-verified: hardcoding states[0].path as the SKIP row's
			// own Path (rather than the first-present candidate found by the
			// loop above) reddens this case alone — it would name root's own
			// missing "CLAUDE.md" instead of the regular, blockless
			// ".claude/CLAUDE.md" that chooseSnippetLocation, and so planSnippet,
			// would actually choose here — restored after.
			name: "no root CLAUDE.md but .claude/CLAUDE.md is a regular file with no block",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "CLAUDE.md"), []byte("unrelated prose\n"), 0o600))
			},
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeveritySkip,
			wantDetail:     "not installed",
			wantFix:        new(runInitClaudeCode),
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
		},
		{
			name: "a current block with CRLF line endings",
			setup: func(t *testing.T, wd string, _ host.Host) {
				t.Helper()

				block := string(artifact.SnippetBlock("docs/specifications")) + "\n"
				crlf := strings.ReplaceAll(block, "\n", "\r\n")
				require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte(crlf), 0o600))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
		},
	})
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
			checkID:           "host-snippet",
			wantSeverity:      doctor.SeverityWarn,
			wantDetail:        "not readable (permission denied); cannot check for brief block",
			wantFix:           new("chmod +r CLAUDE.md, then " + runInitClaudeCode),
			wantPathNotSuffix: filepath.Join(".claude", "CLAUDE.md"),
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
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityWarn,
			wantDetail:     "not readable (permission denied); cannot check for brief block",
			wantFix:        new("chmod u+rwx .claude, then " + runInitClaudeCode),
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
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
		},
		{
			// Pins the block-wins carve-out against an unreadable root
			// candidate specifically (P1): brief cannot tell whether root's
			// own CLAUDE.md carries a block, but .claude/CLAUDE.md's own
			// real block still wins, exactly as it does against a
			// notRegular root candidate in
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
			checkID:        "host-snippet",
			wantSeverity:   doctor.SeverityOK,
			wantDetail:     "installed",
			wantFix:        nil,
			wantPathSuffix: filepath.Join(".claude", "CLAUDE.md"),
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
// leaving the WARN in place. Mutation-verified: reverting
// notReadableFix's own caller to firstPresent.relPath (the pre-fix
// root-relative field) reddens both cases here — the fix text stops
// changing between them — while leaving every case in
// Test_diagnose_classifies_host_snippet (wd == root there, so the two
// relativizations coincide) green.
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

// writeRolesConfig overwrites wd's own ".brief.yaml" with role bindings for
// all three positions — a bare "" leaves that position unbound.
func writeRolesConfig(t *testing.T, wd, planner, implementer, reviewer string) {
	t.Helper()

	body := "progress-heading: \"## Progress\"\n" +
		"roles:\n" +
		fmt.Sprintf("  planner: %q\n", planner) +
		fmt.Sprintf("  implementer: %q\n", implementer) +
		fmt.Sprintf("  reviewer: %q\n", reviewer)

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(body), 0o600))
}

// writePluginAgent writes host.PluginDir's own current render for role's
// plugin agent file (planner, implementer or reviewer).
func writePluginAgent(t *testing.T, wd, role string) {
	t.Helper()

	h := claudeCodeHost(t)

	for _, f := range h.Agents() {
		if strings.HasSuffix(f.RelPath, "/"+role+".md") {
			writeHostArtifact(t, wd, f)
		}
	}
}

// rolesCase is one row of Test_diagnose_classifies_roles: setup mutates
// newHostFixture's own bare baseline, home overrides WithHomeDir (an empty
// temp dir when nil), and the roles row must carry wantSeverity, with
// wantDetail a substring of Detail and wantFix the exact Fix.
type rolesCase struct {
	name         string
	setup        func(t *testing.T, wd string)
	home         func(t *testing.T) string
	wantSeverity doctor.Severity
	wantDetail   string
	wantFix      *string
}

// Test_diagnose_classifies_roles pins roles' own resolution rules (R7,
// Rule 5): no config, or every binding empty, is SKIP "no roles bound"; an
// unparseable config is SKIP naming why; a bound "brief:<name>" resolves
// through the plugin agent file or a project ".claude/agents/<name>.md"
// override; a bound bare "<name>" resolves through a project or injected
// home agent file under ".claude/agents/" whose frontmatter "name:"
// matches; any other "<plugin>:<name>" counts as bound but unverified; any
// unbound or unresolved position is WARN, never ERROR; everything bound
// and resolved is OK. Test_diagnose_roles_resolves_by_frontmatter_name
// covers the frontmatter rule's own nested-layout, shadowing, duplicate
// and user-level cases; this test's own bare-name fixtures stay flat,
// named after the bound role, with frontmatter added only so they still
// resolve.
func Test_diagnose_classifies_roles(t *testing.T) {
	cases := []rolesCase{
		{
			name: "no config anywhere",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no roles bound",
			wantFix:      new(runInitWithAgents),
		},
		{
			name:         "every binding is empty",
			setup:        func(t *testing.T, _ string) { t.Helper() },
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no roles bound",
			wantFix:      new(runInitWithAgents),
		},
		{
			name: "the config is unparseable",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   ".brief.yaml did not parse",
			wantFix:      nil,
		},
		{
			name: "the planner role is unbound",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "", "brief:implementer", "brief:reviewer")
				writePluginAgent(t, wd, "implementer")
				writePluginAgent(t, wd, "reviewer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "planner unbound",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "brief:reviewer is bound but its plugin agent file was removed",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "brief:reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: brief:reviewer not found",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "brief:reviewer's plugin file is gone but a project override exists",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "brief:reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude", "agents"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "agents", "reviewer.md"), []byte("custom reviewer\n"), 0o600))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves via the project's own .claude/agents",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
				require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude", "agents"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "agents", "my-reviewer.md"), []byte("---\nname: my-reviewer\n---\n\ncustom reviewer\n"), 0o600))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves only via the injected home",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
			},
			home: func(t *testing.T) string {
				t.Helper()
				home := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude", "agents"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "agents", "my-reviewer.md"), []byte("---\nname: my-reviewer\n---\n\ncustom reviewer\n"), 0o600))
				return home
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves nowhere",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: my-reviewer not found",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "a binding for a different plugin counts as bound but unverified",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "other:reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "not verified: reviewer",
			wantFix:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHostFixture(t)
			c.setup(t, wd)

			home := t.TempDir()
			if c.home != nil {
				home = c.home(t)
			}

			srv := doctor.NewServer(doctor.WithHomeDir(func() (string, error) { return home, nil }))
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, "roles")
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Contains(t, check.Detail, c.wantDetail)
			assert.Equal(t, c.wantFix, check.Fix)
		})
	}
}

// rolesFrontmatterCase is one row of
// Test_diagnose_roles_resolves_by_frontmatter_name: unlike rolesCase,
// wantDetail is asserted for exact equality, since these cases pin the
// literal wording of the new duplicate WARN and user-level/not-verified OK
// suffixes rather than just the presence of a substring.
type rolesFrontmatterCase struct {
	name         string
	setup        func(t *testing.T, wd string)
	home         func(t *testing.T) string
	wantSeverity doctor.Severity
	wantDetail   string
	wantFix      *string
}

// writeAgentFrontmatter writes a minimal agent file at wd's relPath
// declaring frontmatter "name: name".
func writeAgentFrontmatter(t *testing.T, wd, relPath, name string) {
	t.Helper()

	path := filepath.Join(wd, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("---\nname: "+name+"\n---\n\nbody\n"), 0o600))
}

// rolesUnresolvedFix is the WARN fix roles reports whenever any binding is
// unbound, unresolved or duplicated (host.go's own rolesCheck).
const rolesUnresolvedFix = "bind each role to an existing agent in .brief.yaml, or " + runInitWithAgents

// Test_diagnose_roles_resolves_by_frontmatter_name pins Rule 5: a bare
// binding matches frontmatter "name:" anywhere under ".claude/agents/"
// (nested layout, any filename), a project definition shadows a
// same-named "~/.claude/agents" one, a name defined twice under the
// project's own ".claude/agents" WARNs naming both paths, and a
// user-level-only resolution adds "; user-level: <role>".
func Test_diagnose_roles_resolves_by_frontmatter_name(t *testing.T) {
	cases := []rolesFrontmatterCase{
		{
			name: "a nested project agent resolves by frontmatter name, not filename",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "developer", "brief:reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "reviewer")
				writeAgentFrontmatter(t, wd, ".claude/agents/developer/Agent.md", "developer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a filename match whose frontmatter name differs is not found",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
				writeAgentFrontmatter(t, wd, ".claude/agents/my-reviewer.md", "someone-else")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: my-reviewer not found",
			wantFix:      new(rolesUnresolvedFix),
		},
		{
			name: "a project definition shadows a same-named user definition, no suffix",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
				writeAgentFrontmatter(t, wd, ".claude/agents/team/y.md", "my-reviewer")
			},
			home: func(t *testing.T) string {
				t.Helper()
				home := t.TempDir()
				writeAgentFrontmatter(t, home, ".claude/agents/team/x.md", "my-reviewer")
				return home
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "control: the same user definition with no project file adds the user-level suffix",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
			},
			home: func(t *testing.T) string {
				t.Helper()
				home := t.TempDir()
				writeAgentFrontmatter(t, home, ".claude/agents/team/x.md", "my-reviewer")
				return home
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound; user-level: reviewer",
			wantFix:      nil,
		},
		{
			name: "two project definitions of the same name WARN naming both paths",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "brief:planner", "brief:implementer", "my-reviewer")
				writePluginAgent(t, wd, "planner")
				writePluginAgent(t, wd, "implementer")
				writeAgentFrontmatter(t, wd, ".claude/agents/my-reviewer.md", "my-reviewer")
				writeAgentFrontmatter(t, wd, ".claude/agents/team/r.md", "my-reviewer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: my-reviewer defined 2 times under .claude/agents (.claude/agents/my-reviewer.md, .claude/agents/team/r.md)",
			wantFix:      new(rolesUnresolvedFix),
		},
		{
			name: "user-level and not-verified suffixes follow RoleBindings order",
			setup: func(t *testing.T, wd string) {
				t.Helper()
				writeRolesConfig(t, wd, "my-planner", "brief:implementer", "other:reviewer")
				writePluginAgent(t, wd, "implementer")
			},
			home: func(t *testing.T) string {
				t.Helper()
				home := t.TempDir()
				writeAgentFrontmatter(t, home, ".claude/agents/my-planner.md", "my-planner")
				return home
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound; user-level: planner; not verified: reviewer",
			wantFix:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHostFixture(t)
			c.setup(t, wd)

			home := t.TempDir()
			if c.home != nil {
				home = c.home(t)
			}

			srv := doctor.NewServer(doctor.WithHomeDir(func() (string, error) { return home, nil }))
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, "roles")
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Equal(t, c.wantDetail, check.Detail)
			assert.Equal(t, c.wantFix, check.Fix)
		})
	}
}
