package doctor_test

// Host-integration classification tests: every case here builds its
// subject file tree in an in-memory fstest.MapFS rather than on disk.
// Cases whose own subject is an OS error shape live in host_disk_test.go
// instead, where the OS itself produces the error.

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
// target, resolved relative to relPath's own directory.
func setHostSymlink(fsys fstest.MapFS, relPath, target string) {
	fsys["repo/"+relPath] = &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(target)}
}

// hostFSCheckCase is one row of an in-memory host-check classification
// table: setup mutates newHostFixtureFS's own bare baseline, and
// Diagnose's report must carry checkID at wantSeverity, with wantDetail a
// substring of Detail, wantFix the exact Fix text, and wantRel the row's
// own Path exactly, relative to fsAbs("repo").
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
// Diagnose over the resulting fstest.MapFS rooted at fsAbs("repo"), and
// asserts c.checkID's own row.
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

// The ENOTDIR "ancestor is a regular file" arm lives in
// host_disk_test.go: a fabricated fstest.MapFS cannot produce it.
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

// writeHostPluginWithoutHookFS sets every h.Plugin(true) file except the
// trailing hook entry.
func writeHostPluginWithoutHookFS(fsys fstest.MapFS, h host.Host) {
	for _, f := range h.Plugin(true) {
		if f.Hook {
			continue
		}

		setHostArtifact(fsys, f)
	}
}

// The ENOTDIR and unreadable arms live in host_disk_test.go.
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

// A missing agent file is WARN, never ERROR: an unbound agent is not
// itself a fault. The ENOTDIR and unreadable arms live in
// host_disk_test.go.
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
			// The pre-change planner render, captured mechanically — the
			// one fixture that reaches host-agents' own OriginOlder arm
			// today.
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

// The skill alone, with no plugin or agent file present, still
// classifies itself OK "installed" rather than SKIP — only an absent
// skill defers to whether anything else is installed. The unreadable arm
// lives in host_disk_test.go.
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

// The skill is never an install signal on its own, so host-plugin must
// stay SKIP "not installed" even though the skill file is genuinely
// present and OK.
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

// A block whose line endings are CRLF can never exactly match the LF
// marker, so it is SKIP "not installed". The unreadable-vs-absent split
// lives in host_disk_test.go.
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
			// A dangling symlink target ("elsewhere.md" is never created
			// here) still resolves under Lstat, since Lstat never follows
			// it, but resolves as absent under Stat.
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
			// Root CLAUDE.md is absent here, so firstPresent must reach
			// past states[0] to ".claude/CLAUDE.md".
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
			// planSnippet would choose root CLAUDE.md here and merge into
			// it, never touching the directory at ".claude/CLAUDE.md" —
			// doctor must agree, not WARN about a candidate init would
			// never look at.
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
			// The SKIP row's Path must name the regular, blockless
			// ".claude/CLAUDE.md" here, not root's missing "CLAUDE.md".
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

// Two project agent files under ".claude/agents" both declare frontmatter
// "name: my-reviewer", so the WARN fires once, naming both paths, never
// twice and never silently picking one. Exercised through
// (*Server).projectTree's own fs.Sub-backed project tree.
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
// over an in-memory fstest.MapFS holding one agent file at relPath (always
// ".claude/agents/…") whose contents are body.
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
// case's home field selects: home(t) when set, else the zero Tree.
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

// Test_diagnose_roles_resolves_by_frontmatter_name covers the frontmatter
// rule's own nested-layout, shadowing, duplicate and user-level cases;
// this test's own bare-name fixtures stay flat, named after the bound
// role.
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

// A bare binding matches frontmatter "name:" anywhere under
// ".claude/agents/", a project definition shadows a same-named
// "~/.claude/agents" one, and a user-level-only resolution adds
// "; user-level: <role>".
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
// sibling "roles" row's own exact Detail.
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

// Only the planner and implementer bindings are considered; reviewer is
// excluded.
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

// Continues Test_diagnose_classifies_roles_skill's own table — split into
// a second function only to keep golangci-lint's maintidx metric under
// threshold.
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
			// Roles swapped from the case above, so rolesSkillOKText's
			// singular branch is pinned against both role names.
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
			// A scalar "skills:" value must not fail findIn's own
			// whole-file decode.
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
			// Two project definitions share the same frontmatter name;
			// the WARN must appear once for implementer, not twice.
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
// asserting roles-skill's own row (and, where wantRolesDetail is set, the
// sibling roles row) against it.
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
