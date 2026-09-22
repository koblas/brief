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
	})
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
// directory or a symlink — is WARN, naming which, unless the other
// candidate still holds a real block, in which case that block wins exactly
// as it would if both candidates were regular files.
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

// Test_diagnose_classifies_roles pins roles' own resolution rules (R7):
// no config, or every binding empty, is SKIP "no roles bound"; an
// unparseable config is SKIP naming why; a bound "brief:<name>" resolves
// through the plugin agent file or a project ".claude/agents/<name>.md"
// override; a bound bare "<name>" resolves through the project or an
// injected home's own ".claude/agents/<name>.md"; any other "<plugin>:<name>"
// counts as bound but unverified; any unbound or unresolved position is
// WARN, never ERROR; everything bound and resolved is OK.
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
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "agents", "my-reviewer.md"), []byte("custom reviewer\n"), 0o600))
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
				require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "agents", "my-reviewer.md"), []byte("custom reviewer\n"), 0o600))
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
