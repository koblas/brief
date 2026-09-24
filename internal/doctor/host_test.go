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
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/agentfile"
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
			// empty). See Test_diagnose_classifies_host_agents's own doc
			// comment for the mutation this case is verified against (forcing
			// hostAgentsCheck's own `len(older) > 0` branch to false).
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

// homeAgentTree returns a roles test's own home field: an agentfile.Tree
// over an in-memory fstest.MapFS holding one agent file at relPath (rooted
// the way agentfile.DirTree roots a real "~/.claude/agents" directory, so
// relPath is always ".claude/agents/…") whose contents are body.
func homeAgentTree(relPath, body string) func(t *testing.T) agentfile.Tree {
	return func(t *testing.T) agentfile.Tree {
		t.Helper()

		return agentfile.Tree{
			Dir: fsAbs("home"),
			FS:  fstest.MapFS{relPath: &fstest.MapFile{Data: []byte(body)}},
		}
	}
}

// setRolesConfig sets "repo/.brief.yaml" in fsys with role bindings for all
// three positions — a bare "" leaves that position unbound.
func setRolesConfig(fsys fstest.MapFS, planner, implementer, reviewer string) {
	body := "progress-heading: \"## Progress\"\n" +
		"roles:\n" +
		fmt.Sprintf("  planner: %q\n", planner) +
		fmt.Sprintf("  implementer: %q\n", implementer) +
		fmt.Sprintf("  reviewer: %q\n", reviewer)

	setHostFile(fsys, ".brief.yaml", []byte(body))
}

// homeTreeOption builds the doctor.WithHomeTree option a roles/roles-skill
// case's own home field selects: home(t) when set, else the zero Tree,
// searched by nothing — the same "nothing here" a real, empty home
// directory would produce.
func homeTreeOption(t *testing.T, home func(t *testing.T) agentfile.Tree) doctor.Option {
	t.Helper()

	return doctor.WithHomeTree(func() agentfile.Tree {
		if home != nil {
			return home(t)
		}

		return agentfile.Tree{}
	})
}

// rolesCase is one row of Test_diagnose_classifies_roles: setup mutates
// newHostFixtureFS's own bare baseline, home overrides WithHomeTree, and the
// roles row must carry wantSeverity, with wantDetail a substring of Detail
// and wantFix the exact Fix.
type rolesCase struct {
	name         string
	setup        func(fsys fstest.MapFS, h host.Host)
	home         func(t *testing.T) agentfile.Tree
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
	h := claudeCodeHost(t)
	cases := []rolesCase{
		{
			name:         "no config anywhere",
			setup:        func(fsys fstest.MapFS, _ host.Host) { delete(fsys, "repo/.brief.yaml") },
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no roles bound",
			wantFix:      new(runInitWithAgents),
		},
		{
			name:         "every binding is empty",
			setup:        func(fstest.MapFS, host.Host) {},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no roles bound",
			wantFix:      new(runInitWithAgents),
		},
		{
			name: "the config is unparseable",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, ".brief.yaml", []byte("progress-heading: [not a scalar\n"))
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   ".brief.yaml did not parse",
			wantFix:      nil,
		},
		{
			name: "the planner role is unbound",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "", "brief:implementer", "brief:reviewer")
				setPluginAgent(fsys, h, "implementer")
				setPluginAgent(fsys, h, "reviewer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "planner unbound",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "brief:reviewer is bound but its plugin agent file was removed",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "brief:reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: brief:reviewer not found",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "brief:reviewer's plugin file is gone but a project override exists",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "brief:reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setHostFile(fsys, ".claude/agents/reviewer.md", []byte("custom reviewer\n"))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves via the project's own .claude/agents",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setAgentFrontmatter(fsys, ".claude/agents/my-reviewer.md", "my-reviewer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves only via the injected home",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			home:         homeAgentTree(".claude/agents/my-reviewer.md", agentBody("my-reviewer")),
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a bare role name resolves nowhere",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: my-reviewer not found",
			wantFix:      new("bind each role to an existing agent in .brief.yaml, or run 'brief init --with-agents'"),
		},
		{
			name: "a binding for a different plugin counts as bound but unverified",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "other:reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "not verified: reviewer",
			wantFix:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newHostFixtureFS()
			c.setup(fsys, h)

			srv := doctor.NewServer(doctor.WithRootFS(fsys), homeTreeOption(t, c.home))
			report := srv.Diagnose(t.Context(), fsAbs("repo"))

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
// literal wording of the user-level/not-verified OK suffixes rather than
// just the presence of a substring.
type rolesFrontmatterCase struct {
	name         string
	setup        func(fsys fstest.MapFS, h host.Host)
	home         func(t *testing.T) agentfile.Tree
	wantSeverity doctor.Severity
	wantDetail   string
	wantFix      *string
}

// rolesUnresolvedFix is the WARN fix roles reports whenever any binding is
// unbound, unresolved or duplicated (host.go's own rolesCheck).
const rolesUnresolvedFix = "bind each role to an existing agent in .brief.yaml, or " + runInitWithAgents

// Test_diagnose_roles_resolves_by_frontmatter_name pins Rule 5: a bare
// binding matches frontmatter "name:" anywhere under ".claude/agents/"
// (nested layout, any filename), a project definition shadows a
// same-named "~/.claude/agents" one, and a user-level-only resolution adds
// "; user-level: <role>". Test_diagnose_roles_warns_once_when_a_bare_name_has_two_project_definitions
// pins the duplicate-definition WARN, via (*Server).projectTree.
func Test_diagnose_roles_resolves_by_frontmatter_name(t *testing.T) {
	h := claudeCodeHost(t)
	cases := []rolesFrontmatterCase{
		{
			name: "a nested project agent resolves by frontmatter name, not filename",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "developer", "brief:reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "reviewer")
				setAgentFrontmatter(fsys, ".claude/agents/developer/Agent.md", "developer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "a filename match whose frontmatter name differs is not found",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setAgentFrontmatter(fsys, ".claude/agents/my-reviewer.md", "someone-else")
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "reviewer: my-reviewer not found",
			wantFix:      new(rolesUnresolvedFix),
		},
		{
			name: "a project definition shadows a same-named user definition, no suffix",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setAgentFrontmatter(fsys, ".claude/agents/team/y.md", "my-reviewer")
			},
			home:         homeAgentTree(".claude/agents/team/x.md", agentBody("my-reviewer")),
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound",
			wantFix:      nil,
		},
		{
			name: "control: the same user definition with no project file adds the user-level suffix",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			home:         homeAgentTree(".claude/agents/team/x.md", agentBody("my-reviewer")),
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound; user-level: reviewer",
			wantFix:      nil,
		},
		{
			name: "user-level and not-verified suffixes follow RoleBindings order",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "my-planner", "brief:implementer", "other:reviewer")
				setPluginAgent(fsys, h, "implementer")
			},
			home:         homeAgentTree(".claude/agents/my-planner.md", agentBody("my-planner")),
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer, reviewer bound; user-level: planner; not verified: reviewer",
			wantFix:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newHostFixtureFS()
			c.setup(fsys, h)

			srv := doctor.NewServer(doctor.WithRootFS(fsys), homeTreeOption(t, c.home))
			report := srv.Diagnose(t.Context(), fsAbs("repo"))

			check := findCheck(t, report, "roles")
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Equal(t, c.wantDetail, check.Detail)
			assert.Equal(t, c.wantFix, check.Fix)
		})
	}
}

// rolesSkillMissingFix is the WARN fix every roles-skill lacking-role case
// shares.
const rolesSkillMissingFix = `add "brief-workflow" to the "skills:" list of each agent named, or run 'brief init --edit-agents' for those in the repository`

// blockSkillsFragment, flowSkillsFragment and scalarSkillsFragment are the
// literal "skills:" YAML fragments a rolesSkillCase setup embeds in an
// agent file's own frontmatter, each ending in its own trailing newline so
// a caller can simply concatenate.
const (
	blockSkillsFragment  = "skills:\n  - brief-workflow\n"
	flowSkillsFragment   = "skills: [brief-workflow]\n"
	scalarSkillsFragment = "skills: brief-workflow\n"
	omitClaudeMdTrue     = "omitClaudeMd: true\n"
	omitClaudeMdLoose    = "omitClaudeMd: yes please\n"
)

// agentBody renders a minimal agent file's own frontmatter: "name:" plus
// whatever literal "skills:"/"omitClaudeMd:" fragments the caller passes
// (each already newline-terminated, or "" to omit the key entirely).
func agentBody(name string, fragments ...string) string {
	parts := append([]string{"---\nname: " + name + "\n"}, fragments...)
	parts = append(parts, "---\n\nbody\n")

	return strings.Join(parts, "")
}

// rolesSkillCase is one row of Test_diagnose_classifies_roles_skill: setup
// mutates newHostFixtureFS's own bare baseline, home overrides WithHomeTree,
// and the roles-skill row must carry wantSeverity, wantDetail (exact) and
// wantFix (exact). wantRolesDetail, when non-empty, also asserts the
// sibling "roles" row's own exact Detail — the scalar-skills guard's own
// control, proving a loose decode never drops the agent out of Rule 5
// resolution.
type rolesSkillCase struct {
	name            string
	setup           func(fsys fstest.MapFS, h host.Host)
	home            func(t *testing.T) agentfile.Tree
	noConfig        bool
	wantSeverity    doctor.Severity
	wantDetail      string
	wantFix         *string
	wantRolesDetail string
}

// Test_diagnose_classifies_roles_skill pins roles-skill's own resolution
// rules (S05): only the planner and implementer bindings are considered
// (product verdict item 1 — reviewer is excluded); no config, an
// unparseable config, or neither role bound and resolved is SKIP "no
// bound planner or implementer brief can check", Fix nil; any resolved
// role whose agent does not preload "brief-workflow" is WARN, one entry
// per lacking role joined "; ", in planner-then-implementer order, with
// an omitClaudeMd suffix only on an entry whose own agent sets it;
// otherwise OK, naming only the roles actually checked (resolved, not
// bound to another plugin) — "planner, implementer preload brief-workflow"
// when both, "<role> preloads brief-workflow" when one — plus "; not
// verified: <role>" per role bound to another plugin: a role the row
// never verified never appears in the leading clause, whether it is
// unresolved (the roles row already WARNs it) or bound elsewhere.
func Test_diagnose_classifies_roles_skill(t *testing.T) {
	cases := []rolesSkillCase{
		{
			name:         "no config anywhere",
			setup:        func(fsys fstest.MapFS, _ host.Host) { delete(fsys, "repo/.brief.yaml") },
			noConfig:     true,
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no bound planner or implementer brief can check",
			wantFix:      nil,
		},
		{
			name: "the config is unparseable",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setHostFile(fsys, ".brief.yaml", []byte("progress-heading: [not a scalar\n"))
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no bound planner or implementer brief can check",
			wantFix:      nil,
		},
		{
			name: "planner and implementer unbound, reviewer bound",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "", "", "brief:reviewer")
				setPluginAgent(fsys, h, "reviewer")
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no bound planner or implementer brief can check",
			wantFix:      nil,
		},
		{
			name: "planner and implementer bound but not found",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "my-planner", "my-implementer", "")
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no bound planner or implementer brief can check",
			wantFix:      nil,
		},
		{
			name: "planner and implementer are both other-plugin bindings",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "acme:planner", "acme:implementer", "")
			},
			wantSeverity: doctor.SeveritySkip,
			wantDetail:   "no bound planner or implementer brief can check",
			wantFix:      nil,
		},
		{
			name: "bare names, block-list skills",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "my-planner", "my-implementer", "")
				setHostFile(fsys, ".claude/agents/my-planner.md", []byte(agentBody("my-planner", blockSkillsFragment)))
				setHostFile(fsys, ".claude/agents/my-implementer.md", []byte(agentBody("my-implementer", blockSkillsFragment)))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer preload brief-workflow",
			wantFix:      nil,
		},
		{
			name: "bare names, flow-list skills",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "my-planner", "my-implementer", "")
				setHostFile(fsys, ".claude/agents/my-planner.md", []byte(agentBody("my-planner", flowSkillsFragment)))
				setHostFile(fsys, ".claude/agents/my-implementer.md", []byte(agentBody("my-implementer", flowSkillsFragment)))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer preload brief-workflow",
			wantFix:      nil,
		},
		{
			name: "brief:planner/brief:implementer against rendered plugin agents",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer preload brief-workflow",
			wantFix:      nil,
		},
		{
			name: "brief:implementer overridden by a project file with no frontmatter",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setHostFile(fsys, ".claude/agents/implementer.md", []byte("not a claude code agent file\n"))
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "implementer: brief:implementer does not preload brief-workflow",
			wantFix:      new(rolesSkillMissingFix),
		},
	}

	runRolesSkillCases(t, cases)
}

// Test_diagnose_classifies_roles_skill_verified_and_duplicate_cases
// continues Test_diagnose_classifies_roles_skill's own table — split into a
// second function only to keep golangci-lint's maintidx metric, driven by
// the table literal's own size, under threshold; the two functions pin one
// rule set (roles-skill's own resolution rules, S05) and share
// runRolesSkillCases. This half covers the "not verified" suffix (both role
// names — a hardcoded role literal in rolesSkillOKText must fail here even
// if it passes the sibling "planner" case), the omitClaudeMd suffix, the
// reviewer exclusion, user-level resolution, the loose-decode and
// duplicate-definition guards, and a lone unbound role.
func Test_diagnose_classifies_roles_skill_verified_and_duplicate_cases(t *testing.T) {
	cases := []rolesSkillCase{
		{
			name: "planner resolved with the skill, implementer is an other-plugin binding",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "acme:impl", "")
				setPluginAgent(fsys, h, "planner")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner preloads brief-workflow; not verified: implementer",
			wantFix:      nil,
		},
		{
			// Mirrors the "planner preloads…; not verified: implementer"
			// case above with the roles swapped, so the singular branch of
			// rolesSkillOKText is pinned against both role names, not just
			// "planner" — a hardcoded "planner" literal would still pass
			// the sibling case above but fail here.
			name: "implementer resolved with the skill, planner is an other-plugin binding",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "acme:planner", "brief:implementer", "")
				setPluginAgent(fsys, h, "implementer")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "implementer preloads brief-workflow; not verified: planner",
			wantFix:      nil,
		},
		{
			name: "planner and implementer both lack the skill, implementer omits CLAUDE.md",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "my-planner", "my-implementer", "")
				setHostFile(fsys, ".claude/agents/my-planner.md", []byte(agentBody("my-planner")))
				setHostFile(fsys, ".claude/agents/my-implementer.md", []byte(agentBody("my-implementer", omitClaudeMdTrue)))
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail: "planner: my-planner does not preload brief-workflow; " +
				"implementer: my-implementer does not preload brief-workflow and omits CLAUDE.md, so it never sees brief's instructions",
			wantFix: new(rolesSkillMissingFix),
		},
		{
			name: "a non-bool omitClaudeMd never adds the suffix",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "", "my-implementer", "")
				setHostFile(fsys, ".claude/agents/my-implementer.md", []byte(agentBody("my-implementer", omitClaudeMdLoose)))
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "implementer: my-implementer does not preload brief-workflow",
			wantFix:      new(rolesSkillMissingFix),
		},
		{
			name: "reviewer lacks the skill but is excluded",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "brief:implementer", "my-reviewer")
				setPluginAgent(fsys, h, "planner")
				setPluginAgent(fsys, h, "implementer")
				setHostFile(fsys, ".claude/agents/my-reviewer.md", []byte(agentBody("my-reviewer")))
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner, implementer preload brief-workflow",
			wantFix:      nil,
		},
		{
			name: "a user-level-only planner carries the skill",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "my-planner", "", "")
			},
			home:         homeAgentTree(".claude/agents/my-planner.md", agentBody("my-planner", blockSkillsFragment)),
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner preloads brief-workflow",
			wantFix:      nil,
		},
		{
			// Control for the loose-decode contract: a scalar "skills:"
			// value must not fail findIn's own whole-file decode — if it
			// did, the implementer would resolve as "not found" and the
			// roles row's own detail would gain an "implementer: … not
			// found" problem instead of staying silent about it.
			name: "scalar skills guards the loose decode",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "", "my-implementer", "")
				setHostFile(fsys, ".claude/agents/my-implementer.md", []byte(agentBody("my-implementer", scalarSkillsFragment)))
			},
			wantSeverity:    doctor.SeverityWarn,
			wantDetail:      "implementer: my-implementer does not preload brief-workflow",
			wantFix:         new(rolesSkillMissingFix),
			wantRolesDetail: "planner unbound; reviewer unbound",
		},
		{
			// Duplicate guard: two project definitions share the same
			// frontmatter name, sorted second (path order) lacks the
			// skill — the WARN must appear once for implementer, not
			// twice.
			name: "a duplicate definition WARNs once",
			setup: func(fsys fstest.MapFS, _ host.Host) {
				setRolesConfig(fsys, "", "dup-implementer", "")
				setHostFile(fsys, ".claude/agents/aaa.md", []byte(agentBody("dup-implementer", blockSkillsFragment)))
				setHostFile(fsys, ".claude/agents/zzz.md", []byte(agentBody("dup-implementer")))
			},
			wantSeverity: doctor.SeverityWarn,
			wantDetail:   "implementer: dup-implementer does not preload brief-workflow",
			wantFix:      new(rolesSkillMissingFix),
		},
		{
			name: "planner resolved with the skill, implementer unbound",
			setup: func(fsys fstest.MapFS, h host.Host) {
				setRolesConfig(fsys, "brief:planner", "", "")
				setPluginAgent(fsys, h, "planner")
			},
			wantSeverity: doctor.SeverityOK,
			wantDetail:   "planner preloads brief-workflow",
			wantFix:      nil,
		},
	}

	runRolesSkillCases(t, cases)
}

// runRolesSkillCases runs each rolesSkillCase in cases as its own subtest,
// diagnosing a fresh newHostFixtureFS and asserting roles-skill's own row
// (and, where wantRolesDetail is set, the sibling roles row) against it —
// the execution loop Test_diagnose_classifies_roles_skill and
// Test_diagnose_classifies_roles_skill_verified_and_duplicate_cases both
// share.
func runRolesSkillCases(t *testing.T, cases []rolesSkillCase) {
	t.Helper()

	h := claudeCodeHost(t)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newHostFixtureFS()
			c.setup(fsys, h)

			srv := doctor.NewServer(doctor.WithRootFS(fsys), homeTreeOption(t, c.home))
			report := srv.Diagnose(t.Context(), fsAbs("repo"))

			check := findCheck(t, report, "roles-skill")
			assert.Equal(t, c.wantSeverity, check.Severity)
			assert.Equal(t, c.wantDetail, check.Detail)
			assert.Equal(t, c.wantFix, check.Fix)

			if c.noConfig {
				assert.Empty(t, check.Path)
			} else {
				assert.Equal(t, fsAbs("repo", ".brief.yaml"), check.Path)
			}

			if c.wantRolesDetail != "" {
				roles := findCheck(t, report, "roles")
				assert.Equal(t, c.wantRolesDetail, roles.Detail)
			}
		})
	}
}
