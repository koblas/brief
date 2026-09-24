package doctor_test

// Host-integration classification tests: every case here builds its
// subject file tree in an in-memory fstest.MapFS (newHostFixtureFS) rather
// than on disk. Diagnose's own root-dir and env-path rows still consult
// the real OS (root-dir os.Stats the fabricated "repo/docs/specifications"
// that does not exist there; env-path calls exec.LookPath and reads the
// running binary's build info) — neither row is asserted against by any
// case here, so those incidental reads never affect a result this file
// checks. Cases whose own subject is an OS error shape — a chmod'd file or
// directory (classifyProbeError's own unreadable arm), or an ancestor path
// component that is a regular file (ENOTDIR) — live in host_disk_test.go
// instead, where the OS itself, not a fabricated fs.FS, produces the error
// classifyProbeError has to discriminate.

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runInit is short for "run 'brief init'" — the fix text repeated across
// host-plugin, host-hook, host-snippet and host-agents rows whenever a
// plain re-run would repair the finding.
const runInit = "run 'brief init'"

// runInitClaudeCode is the SKIP fix every "not installed" host row shares.
const runInitClaudeCode = "run 'brief init --host claude-code'"

// runInitWithAgents is the fix host-agents and roles share whenever the
// remedy needs --with-agents specifically.
const runInitWithAgents = "run 'brief init --with-agents'"

// claudeCodeHost returns the claude-code Host every fixture in this file
// builds files for.
func claudeCodeHost(t *testing.T) host.Host {
	t.Helper()

	h, ok := host.Lookup(host.ClaudeCode)
	require.True(t, ok)

	return h
}

// newHostFixtureFS returns an in-memory fstest.MapFS, rooted at
// fsAbs("repo"), holding a valid ".brief.yaml" (unbound roles, the default
// feature directory) with no Claude Code integration installed. Each case
// adds exactly the files its own scenario needs, via
// setHostArtifact/setHostFile/setHostDir/setHostSymlink.
func newHostFixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"repo/.brief.yaml": &fstest.MapFile{Data: []byte("progress-heading: \"## Progress\"\n")},
	}
}

// setHostArtifact sets f's own current artifact.Render at repo/f.RelPath
// in fsys.
func setHostArtifact(fsys fstest.MapFS, f host.File) {
	fsys["repo/"+f.RelPath] = &fstest.MapFile{Data: artifact.Render(f.Kind)}
}

// setHostFile sets body at repo/relPath in fsys.
func setHostFile(fsys fstest.MapFS, relPath string, body []byte) {
	fsys["repo/"+relPath] = &fstest.MapFile{Data: body}
}

// setHostDir sets a directory entry at repo/relPath in fsys, standing in
// for an integration file a repository owner replaced with a directory.
func setHostDir(fsys fstest.MapFS, relPath string) {
	fsys["repo/"+relPath] = &fstest.MapFile{Mode: fs.ModeDir}
}

// setHostSymlink sets a symlink at repo/relPath in fsys pointing at
// target — target is resolved relative to relPath's own directory, the
// same way fstest.MapFS resolves every symlink it holds; an absolute
// target never resolves, mirroring the restriction fs.ReadLinkFS's own
// production adapter (os.DirFS) does not share but this package's probes
// never rely on: only Lstat's own non-regular classification is ever
// exercised against a symlink here, never a followed read.
func setHostSymlink(fsys fstest.MapFS, relPath, target string) {
	fsys["repo/"+relPath] = &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(target)}
}

// hostFSCheckCase is one row of an in-memory host-check classification
// table: setup mutates newHostFixtureFS's own bare baseline, and
// Diagnose's report must carry checkID at wantSeverity, with wantDetail a
// substring of Detail, wantFix the exact Fix text (nil when the row
// carries none), and wantRel the row's own Path exactly, relative to
// fsAbs("repo") — MapFS paths are deterministic, so every case pins the
// full path rather than guessing at a suffix the way host_disk_test.go's
// own hostCheckCase sometimes has to.
type hostFSCheckCase struct {
	name         string
	setup        func(fsys fstest.MapFS, h host.Host)
	checkID      string
	wantSeverity doctor.Severity
	wantDetail   string
	wantFix      *string
	wantRel      string
}

// runHostFSCheckCases builds newHostFixtureFS(), applies c.setup, runs
// Diagnose over the resulting fstest.MapFS (doctor.WithRootFS) rooted at
// fsAbs("repo"), and asserts c.checkID's own row against c.wantSeverity,
// c.wantDetail (substring), c.wantFix (exact) and c.wantRel (the row's own
// Path, exactly).
func runHostFSCheckCases(t *testing.T, cases []hostFSCheckCase) {
	t.Helper()

	h := claudeCodeHost(t)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newHostFixtureFS()
			c.setup(fsys, h)

			srv := doctor.NewServer(emptyHomeDir(t), doctor.WithRootFS(fsys))
			report := srv.Diagnose(t.Context(), fsAbs("repo"))

			check := findCheck(t, report, c.checkID)
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Contains(t, check.Detail, c.wantDetail)
			assert.Equal(t, c.wantFix, check.Fix)
			assert.Equal(t, fsAbs("repo", c.wantRel), check.Path)
		})
	}
}

// Test_diagnose_classifies_host_plugin pins host-plugin's own precedence:
// nothing installed anywhere is SKIP; once something is installed
// (Plugin(true) ∪ Agents()), a missing or non-regular subject file
// (Plugin(false)) is ERROR "incomplete", naming it; an edited one is OK
// "edited locally"; every subject file current is OK "installed". The
// ENOTDIR "ancestor is a regular file" arm lives in host_disk_test.go: a
// direct probe against the stdlib shows the same fixture over
// fstest.MapFS reports plain fs.ErrNotExist rather than ENOTDIR, so it
// cannot exercise classifyProbeError's own ENOTDIR arm here.
// Mutation-verified, package-wide with no -run filter: forcing
// hostPluginCheck's own missing-file precedence branch to `false` reddens
// exactly the three ERROR "incomplete" cases here ("manifest missing …",
// "a skill file is a directory", "only the hook is installed") — never
// "edited locally" or "every subject file is current" — proving this
// table actually discriminates on it rather than passing regardless.
func Test_diagnose_classifies_host_plugin(t *testing.T) {
	runHostFSCheckCases(t, []hostFSCheckCase{
		{
			name:         "nothing installed anywhere",
			setup:        func(fstest.MapFS, host.Host) {},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      host.PluginDir,
		},
		{
			name: "manifest missing while the rest of the plugin is installed",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					if f.Kind == artifact.KindPluginManifest {
						continue
					}

					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/.claude-plugin/plugin.json",
			wantFix:      new(runInit),
			wantRel:      host.PluginDir,
		},
		{
			name: "a skill file is a directory",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					if f.Kind == artifact.KindSkillStart {
						setHostDir(fsys, f.RelPath)

						continue
					}

					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/skills/start/SKILL.md",
			wantFix:      new(runInit),
			wantRel:      host.PluginDir,
		},
		{
			name: "only the hook is installed",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					if f.Hook {
						setHostArtifact(fsys, f)
					}
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "incomplete: missing .claude/skills/brief/.claude-plugin/plugin.json, .claude/skills/brief/skills/start/SKILL.md, .claude/skills/brief/skills/finish/SKILL.md",
			wantFix:      new(runInit),
			wantRel:      host.PluginDir,
		},
		{
			name: "a subject file was edited locally",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}

				setHostFile(fsys, host.PluginDir+"/.claude-plugin/plugin.json", []byte(`{"name": "brief", "custom": true}`))
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally: " + host.PluginDir + "/.claude-plugin/plugin.json",
			wantFix:      nil,
			wantRel:      host.PluginDir,
		},
		{
			name: "every subject file is current",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-plugin",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      host.PluginDir,
		},
	})
}

// writeHostPluginWithoutHookFS sets every h.Plugin(true) file in fsys
// except the trailing hook entry.
func writeHostPluginWithoutHookFS(fsys fstest.MapFS, h host.Host) {
	for _, f := range h.Plugin(true) {
		if f.Hook {
			continue
		}

		setHostArtifact(fsys, f)
	}
}

// Test_diagnose_classifies_host_hook pins host-hook's own precedence:
// nothing installed anywhere is SKIP; the rest of the plugin installed
// with no hook file is WARN, since doctor cannot tell a lost file from
// --no-hook; a hook path that is a directory is ERROR; an edited hook is
// OK "edited locally"; a current hook is OK "installed". The ENOTDIR and
// unreadable arms live in host_disk_test.go — see
// Test_diagnose_classifies_host_plugin's own doc comment for why.
// Mutation-verified, package-wide with no -run filter: inverting
// hostHookCheck's own absent-branch condition (`if !installed` to `if
// installed`) reddens both "nothing installed anywhere" (SKIP flips to
// WARN) and "the rest of the plugin is installed but the hook file is
// missing" (WARN flips to SKIP) — no other case here — restored after.
func Test_diagnose_classifies_host_hook(t *testing.T) {
	runHostFSCheckCases(t, []hostFSCheckCase{
		{
			name:         "nothing installed anywhere",
			setup:        func(fstest.MapFS, host.Host) {},
			checkID:      "host-hook",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			name: "the rest of the plugin is installed but the hook file is missing",
			setup: func(fsys fstest.MapFS, h host.Host) {
				writeHostPluginWithoutHookFS(fsys, h)
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "installed without the check hook",
			wantFix:      new("run 'brief init' to add it"),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			name: "the hook path is a directory",
			setup: func(fsys fstest.MapFS, h host.Host) {
				writeHostPluginWithoutHookFS(fsys, h)

				for _, f := range h.Plugin(true) {
					if f.Hook {
						setHostDir(fsys, f.RelPath)
					}
				}
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "not a regular file",
			wantFix:      new(runInit),
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			name: "the hook file was edited locally",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}

				setHostFile(fsys, host.PluginDir+"/hooks/hooks.json", []byte(`{"hooks": {}}`))
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally",
			wantFix:      nil,
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
		{
			name: "the hook file is current",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-hook",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      host.PluginDir + "/hooks/hooks.json",
		},
	})
}

// Test_diagnose_classifies_host_agents pins host-agents' own precedence,
// distinct from host-plugin's: none of the three agent files present is
// SKIP; a missing one, given at least one of the three exists, is WARN
// (never ERROR — an unbound agent is not itself a fault); an older render
// is WARN naming it; an edited one is OK "edited locally"; all three
// current is OK "installed". Every fix here names --with-agents, since a
// plain "brief init" never touches agent files. The ENOTDIR and unreadable
// arms live in host_disk_test.go — see Test_diagnose_classifies_host_plugin's
// own doc comment for why. Mutation-verified, package-wide with no -run
// filter, one precedence branch at a time: forcing hostAgentsCheck's own
// missing branch (`len(missing) > 0`) to false reddens exactly "one of the
// three agent files is missing"; forcing the older branch
// (`len(older) > 0`) to false reddens exactly "an older planner render";
// forcing the edited branch (`len(edited) > 0`) to false reddens exactly
// "an agent file was edited locally" — each restored before the next.
func Test_diagnose_classifies_host_agents(t *testing.T) {
	runHostFSCheckCases(t, []hostFSCheckCase{
		{
			name:         "none of the three agent files present",
			setup:        func(fstest.MapFS, host.Host) {},
			checkID:      "host-agents",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitWithAgents),
			wantRel:      host.PluginDir + "/agents",
		},
		{
			name: "one of the three agent files is missing",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Agents() {
					if f.Kind == artifact.KindAgentReviewer {
						continue
					}

					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "missing .claude/skills/brief/agents/reviewer.md",
			wantFix:      new(runInitWithAgents),
			wantRel:      host.PluginDir + "/agents",
		},
		{
			// SCENARIO-02: the pre-scenario planner render, captured
			// mechanically (%q dump) before agents.go changed — the one
			// fixture that actually reaches host-agents' own OriginOlder
			// arm today (every other Kind's older…Digests list still ships
			// empty). Mutation-verify by emptying olderAgentPlannerDigests:
			// this case alone reddens (falls through to "edited locally"),
			// the others above and below stay green.
			name: "an older planner render",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Agents() {
					if f.Kind == artifact.KindAgentPlanner {
						continue
					}

					setHostArtifact(fsys, f)
				}

				older := []byte("---\nname: planner\ndescription: Turn a feature's specification into ordered scenario " +
					"plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's " +
					"specification into ordered scenario plans: run `brief new step <feature>` for the next " +
					"scenario, then fill its plan file. Never write production or test code.\n")
				setHostFile(fsys, host.PluginDir+"/agents/planner.md", older)
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "installed by an older brief release: .claude/skills/brief/agents/planner.md",
			wantFix:      new(runInitWithAgents),
			wantRel:      host.PluginDir + "/agents",
		},
		{
			name: "an agent file was edited locally",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Agents() {
					setHostArtifact(fsys, f)
				}

				setHostFile(fsys, host.PluginDir+"/agents/planner.md", []byte("---\nname: planner\n---\n\ncustom\n"))
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally: .claude/skills/brief/agents/planner.md",
			wantFix:      nil,
			wantRel:      host.PluginDir + "/agents",
		},
		{
			name: "all three agent files are current",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Agents() {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-agents",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      host.PluginDir + "/agents",
		},
	})
}

// Test_diagnose_classifies_host_skill pins host-skill's own precedence:
// nothing installed anywhere is SKIP; the skill missing while some other
// Claude Code integration file is installed is WARN, since a bound role
// can never preload a skill that is not there; a skill path that is a
// directory is ERROR — Rule 7's one new ERROR arm; an edited skill is OK
// "edited locally"; a current one, alongside a full install, is OK
// "installed"; and the skill alone, with no plugin or agent file present
// at all, still classifies itself OK "installed" rather than SKIP — only
// an absent skill defers to whether anything else is installed. The
// unreadable arm (skill mode 0o000) lives in host_disk_test.go.
// Mutation-verified, package-wide with no -run filter, one arm at a time:
// inverting hostSkillRow's own absent-branch condition (`if !installed` to
// `if installed`) reddens both "nothing installed anywhere" and "plugin
// files present and skill absent"; changing the ERROR not-a-regular-file
// arm's own Severity to WARN reddens "skill path is a directory" alone;
// changing that same arm's own Fix to runInit (dropping
// hostSkillNotRegularFix) reddens "skill path is a directory" alone too;
// replacing the final originRow-derived return with a hardcoded ERROR row
// reddens "the skill was edited locally", "the skill is current, alongside
// a full install" and "the skill alone, with no plugin or agent file" —
// every case that actually reaches it — together. Each restored before the
// next.
func Test_diagnose_classifies_host_skill(t *testing.T) {
	skillPath := host.WorkflowSkillDir + "/SKILL.md"

	runHostFSCheckCases(t, []hostFSCheckCase{
		{
			name:         "nothing installed anywhere",
			setup:        func(fstest.MapFS, host.Host) {},
			checkID:      "host-skill",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      skillPath,
		},
		{
			name: "plugin files present and skill absent",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not installed; bound agents cannot preload it",
			wantFix:      new(runInit),
			wantRel:      skillPath,
		},
		{
			name: "skill path is a directory",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostDir(fsys, skillPath)
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "not a regular file",
			wantFix:      new("remove " + skillPath + ", then " + runInit),
			wantRel:      skillPath,
		},
		{
			name: "the skill was edited locally",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, skillPath, []byte("custom skill body\n"))
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally",
			wantFix:      nil,
			wantRel:      skillPath,
		},
		{
			name: "the skill is current, alongside a full install",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Plugin(true) {
					setHostArtifact(fsys, f)
				}

				for _, f := range h.Skills() {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      skillPath,
		},
		{
			name: "the skill alone, with no plugin or agent file",
			setup: func(fsys fstest.MapFS, h host.Host) {
				for _, f := range h.Skills() {
					setHostArtifact(fsys, f)
				}
			},
			checkID:      "host-skill",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      skillPath,
		},
	})
}

// Test_diagnose_host_plugin_stays_skip_when_only_the_skill_is_installed
// pins anyIntegrationFilePresent's own h.Skills() exclusion (R13's own
// doc comment on that function): the skill is never an install signal on
// its own, so host-plugin — whose own "installed" flag is
// anyIntegrationFilePresent's result — must stay SKIP "not installed" even
// though the skill file itself is genuinely present and OK (pinned
// separately by "the skill alone, with no plugin or agent file" above).
// Control: "nothing installed anywhere" in Test_diagnose_classifies_host_plugin
// is the same SKIP row with nothing at all present. Mutation-verified,
// package-wide with no -run filter: adding h.Skills() to
// anyIntegrationFilePresent's own OR reddens this case (host-plugin flips
// to ERROR "incomplete: missing …") and, since the same result also feeds
// (*Server).Diagnose's own integrationInstalled for env-path, doctor_test.go's
// own Test_diagnose_classifies_env_path_by_whether_the_integration_is_installed/
// "not on PATH, only the brief-workflow skill is installed" — no other
// case in the package, restored after.
func Test_diagnose_host_plugin_stays_skip_when_only_the_skill_is_installed(t *testing.T) {
	fsys := newHostFixtureFS()
	h := claudeCodeHost(t)

	for _, f := range h.Skills() {
		setHostArtifact(fsys, f)
	}

	srv := doctor.NewServer(emptyHomeDir(t), doctor.WithRootFS(fsys))
	report := srv.Diagnose(t.Context(), fsAbs("repo"))

	check := findCheck(t, report, "host-plugin")
	assert.Equal(t, doctor.SeveritySkip, check.Severity)
	assert.Equal(t, "not installed", check.Detail)
	require.NotNil(t, check.Fix)
	assert.Equal(t, runInitClaudeCode, *check.Fix)
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
// split lives in host_disk_test.go's own
// Test_diagnose_classifies_host_snippet_unreadable. Mutation-verified,
// package-wide with no -run filter: short-circuiting hostSnippetCheck's
// own marker-defect loop (`for _, s := range states { if s.prob != nil
// {…} }`) to never fire reddens "a lone begin marker" alone — the only
// case here whose subject is a marker defect.
func Test_diagnose_classifies_host_snippet(t *testing.T) {
	runHostFSCheckCases(t, []hostFSCheckCase{
		{
			name: "a lone begin marker",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, "CLAUDE.md", []byte("intro\n"+artifact.SnippetBegin+"\nno end after this\n"))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "line 2",
			wantFix:      new("add " + artifact.SnippetEnd + " after it, or remove the lone marker"),
			wantRel:      "CLAUDE.md",
		},
		{
			name: "a block exists in both candidates",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				setHostFile(fsys, "CLAUDE.md", block)
				setHostFile(fsys, ".claude/CLAUDE.md", block)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityError,
			wantDetail:   "a brief block already exists in CLAUDE.md",
			wantFix:      new("delete that block"),
			wantRel:      ".claude/CLAUDE.md",
		},
		{
			name: "a block only in .claude/CLAUDE.md",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				setHostFile(fsys, ".claude/CLAUDE.md", block)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      ".claude/CLAUDE.md",
		},
		{
			name: "a current block names a different feature directory",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				block := append(append([]byte{}, artifact.SnippetBlock("elsewhere")...), '\n')
				setHostFile(fsys, "CLAUDE.md", block)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "names elsewhere/, .brief.yaml says docs/specifications/",
			wantFix:      new(runInit),
			wantRel:      "CLAUDE.md",
		},
		{
			name: "the config is unparseable so the directory is not compared",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, ".brief.yaml", []byte("progress-heading: [not a scalar\n"))

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				setHostFile(fsys, "CLAUDE.md", block)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed (feature directory not compared: .brief.yaml did not parse)",
			wantFix:      nil,
			wantRel:      "CLAUDE.md",
		},
		{
			name: "the block was edited locally",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				body := artifact.SnippetBegin + "\ncustom prose\n" + artifact.SnippetEnd + "\n"
				setHostFile(fsys, "CLAUDE.md", []byte(body))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "edited locally",
			wantFix:      nil,
			wantRel:      "CLAUDE.md",
		},
		{
			name:         "no CLAUDE.md anywhere",
			setup:        func(fstest.MapFS, host.Host) {},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      "CLAUDE.md",
		},
		{
			name: "CLAUDE.md is a directory",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostDir(fsys, "CLAUDE.md")
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not a regular file (directory); brief block not installed",
			wantFix:      new("run 'brief init --print' and add the CLAUDE.md block by hand"),
			wantRel:      "CLAUDE.md",
		},
		{
			// This is this table's own discriminator between fs.Lstat and
			// fs.Stat: a dangling symlink target ("elsewhere.md" is never
			// created in this fixture) still resolves under Lstat, since
			// Lstat never follows it, but resolves as absent under Stat.
			// Mutation-verified, package-wide with no -run filter: changing
			// scanSnippetCandidateStates's own fs.Lstat call to fs.Stat
			// reddens this case alone — Stat follows the dangling symlink,
			// finds nothing, and the row falls to SKIP "not installed" —
			// restored after.
			name: "CLAUDE.md is a symlink",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostSymlink(fsys, "CLAUDE.md", "elsewhere.md")
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not a regular file (symlink); brief block not installed",
			wantFix:      new("run 'brief init --print' and add the CLAUDE.md block by hand"),
			wantRel:      "CLAUDE.md",
		},
		{
			name: "CLAUDE.md is a symlink but .claude/CLAUDE.md holds a real block",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostSymlink(fsys, "CLAUDE.md", "elsewhere.md")

				block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
				setHostFile(fsys, ".claude/CLAUDE.md", block)
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "installed",
			wantFix:      nil,
			wantRel:      ".claude/CLAUDE.md",
		},
		{
			// Mutation-verified, package-wide with no -run filter: replacing
			// the firstPresent-finding loop with an unconditional
			// "firstPresent = &states[0] if states[0] is present, else nil"
			// (dropping the search past states[0]) reddens this case — root
			// CLAUDE.md is absent here, so firstPresent never reaches
			// ".claude/CLAUDE.md" and the row falls to the fallback SKIP "not
			// installed" — together with every other case whose own
			// first-present candidate is not states[0]: "no root CLAUDE.md
			// but .claude/CLAUDE.md is a regular file with no block" below,
			// host_disk_test.go's own
			// Test_diagnose_classifies_host_snippet_unreadable/"no root
			// CLAUDE.md, .claude itself cannot be Lstat'd", and doctor_test.go's
			// own
			// Test_diagnose_treats_an_unreadable_host_snippet_directory_as_present_not_absent.
			// Restored after.
			name: "no root CLAUDE.md but .claude/CLAUDE.md is a directory",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostDir(fsys, ".claude/CLAUDE.md")
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "not a regular file (directory); brief block not installed",
			wantFix:      new("run 'brief init --print' and add the CLAUDE.md block by hand"),
			wantRel:      ".claude/CLAUDE.md",
		},
		{
			// Pins C1's fix: planSnippet would choose root CLAUDE.md here
			// (chooseSnippetLocation's own "first candidate that exists at
			// all" rule) and merge into it, never touching the directory at
			// ".claude/CLAUDE.md" — doctor must agree, not WARN about a
			// candidate init would never look at. wantRel's own exact-path
			// assertion is the stronger check here: it fails both if the
			// SKIP row ever named ".claude/CLAUDE.md" instead and if it
			// named neither candidate.
			name: "root CLAUDE.md exists with no block, .claude/CLAUDE.md is a directory",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, "CLAUDE.md", []byte("unrelated prose\n"))
				setHostDir(fsys, ".claude/CLAUDE.md")
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      "CLAUDE.md",
		},
		{
			// Mutation-verified, package-wide with no -run filter: collapsing
			// the SKIP row's own Path selection (the firstPresent/states[0]
			// switch) to always use states[0].path reddens this case alone
			// — it would name root's own missing "CLAUDE.md" instead of the
			// regular, blockless ".claude/CLAUDE.md" that
			// chooseSnippetLocation, and so planSnippet, would actually choose
			// here — restored after.
			name: "no root CLAUDE.md but .claude/CLAUDE.md is a regular file with no block",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, ".claude/CLAUDE.md", []byte("unrelated prose\n"))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      ".claude/CLAUDE.md",
		},
		{
			name: "a current block with CRLF line endings",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				block := string(artifact.SnippetBlock("docs/specifications")) + "\n"
				crlf := strings.ReplaceAll(block, "\n", "\r\n")
				setHostFile(fsys, "CLAUDE.md", []byte(crlf))
			},
			checkID:      "host-snippet",
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "not installed",
			wantFix:      new(runInitClaudeCode),
			wantRel:      "CLAUDE.md",
		},
	})
}

// setPluginAgent sets host.PluginDir's own current render for role's
// plugin agent file (planner, implementer or reviewer) in fsys.
func setPluginAgent(fsys fstest.MapFS, h host.Host, role string) {
	for _, f := range h.Agents() {
		if strings.HasSuffix(f.RelPath, "/"+role+".md") {
			setHostArtifact(fsys, f)
		}
	}
}

// setAgentFrontmatter sets a minimal agent file at repo/relPath in fsys
// declaring frontmatter "name: name".
func setAgentFrontmatter(fsys fstest.MapFS, relPath, name string) {
	setHostFile(fsys, relPath, []byte("---\nname: "+name+"\n---\n\nbody\n"))
}

// Test_diagnose_roles_warns_once_when_a_bare_name_has_two_project_definitions
// pins duplicateDefinitionProblem's own WARN, exercised through
// (*Server).projectTree's own fs.Sub-backed project tree: two project
// agent files under ".claude/agents" both declare frontmatter "name:
// my-reviewer" — the bare binding the reviewer role names — so
// duplicateDefinitionProblem's own WARN fires once, naming both paths in
// findIn's own Path order, never twice and never silently picking one.
// Mutation-verified, package-wide with no -run filter: raising
// duplicateDefinitionProblem's own `len(defs) < 2` threshold to `< 3`
// reddens this case alone (two definitions no longer count as a
// duplicate); reverting (*Server).resolveRoleBinding to call
// agentfile.DirTree(root) directly, bypassing projectTree, also reddens
// this case alone (root/.claude/agents then reads through the real "/"
// filesystem instead of fsys, and the injected fixture is never on disk)
// — proving projectTree is the path actually exercised here, not merely
// present in the call graph. Both restored after.
func Test_diagnose_roles_warns_once_when_a_bare_name_has_two_project_definitions(t *testing.T) {
	fsys := newHostFixtureFS()
	h := claudeCodeHost(t)

	setHostFile(fsys, ".brief.yaml", []byte(
		"progress-heading: \"## Progress\"\n"+
			"roles:\n"+
			"  planner: brief:planner\n"+
			"  implementer: brief:implementer\n"+
			"  reviewer: my-reviewer\n"))
	setPluginAgent(fsys, h, "planner")
	setPluginAgent(fsys, h, "implementer")
	setAgentFrontmatter(fsys, ".claude/agents/my-reviewer.md", "my-reviewer")
	setAgentFrontmatter(fsys, ".claude/agents/team/r.md", "my-reviewer")

	srv := doctor.NewServer(emptyHomeDir(t), doctor.WithRootFS(fsys))
	report := srv.Diagnose(t.Context(), fsAbs("repo"))

	check := findCheck(t, report, "roles")
	assert.Equal(t, doctor.SeverityWarn, check.Severity)
	assert.Equal(t, "reviewer: my-reviewer defined 2 times under .claude/agents (.claude/agents/my-reviewer.md, .claude/agents/team/r.md)", check.Detail)
	require.NotNil(t, check.Fix)
	assert.Equal(t, "bind each role to an existing agent in .brief.yaml, or "+runInitWithAgents, *check.Fix)
}
