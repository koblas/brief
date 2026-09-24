package doctor

// White-box package. This file pins three unexported units host_test.go's
// own black-box table cannot reach economically:
//
//   - originRow is host.go's own unexported origin→row mapping. Every
//     compiled-in older digest list but the planner and implementer agents'
//     own (internal/platform/artifact) still ships empty, so no real
//     host-plugin, host-hook or host-snippet fixture's bytes ever classify
//     as artifact.OriginOlder through artifact.Recognize or
//     artifact.RecognizeSnippet — those three rows' own OriginOlder arm is
//     reachable only by calling originRow directly with a synthetic
//     artifact.OriginOlder value, as this file does. host-agents' own
//     OriginOlder arm is covered black-box instead, in host_test.go's
//     Test_diagnose_classifies_host_agents ("an older planner render"),
//     through the real artifact.Recognize path. Every other arm
//     (present/missing, edited, current) is covered black-box in
//     host_test.go the same way.
//   - hostSkillRow is host.go's own unexported state→row mapping for
//     host-skill. olderSkillWorkflowDigests ships empty, the same as every
//     other Kind but the planner and implementer agents', so its own
//     OriginOlder arm is reachable only by calling hostSkillRow directly
//     with a synthetic integrationFileState carrying artifact.OriginOlder,
//     mirroring originRow's own case above. Every other arm is covered
//     black-box, in host_test.go's Test_diagnose_classifies_host_skill.
//   - nonRegularKind's own default (neither-symlink-nor-directory) arm needs
//     a mode a black-box fixture cannot portably construct: os.Symlink and
//     os.Mkdir work on every platform this project targets, but a named
//     pipe requires syscall.Mkfifo, which is POSIX-only and would make
//     host_test.go itself platform-conditional. A fake os.FileInfo reaches
//     the same branch without that dependency.
//   - notRegularDetail's own empty-kind fold depends directly on
//     nonRegularKind's default arm reporting "" — calling it with "" here
//     pins the fold itself without needing the fifo fixture nonRegularKind's
//     own test already stands in for.

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFIFOInfo is a minimal os.FileInfo reporting a named-pipe mode — the
// one non-symlink, non-directory shape nonRegularKind's own default arm
// falls through to, without depending on syscall.Mkfifo's platform support.
type fakeFIFOInfo struct{}

func (fakeFIFOInfo) Name() string       { return "CLAUDE.md" }
func (fakeFIFOInfo) Size() int64        { return 0 }
func (fakeFIFOInfo) Mode() os.FileMode  { return os.ModeNamedPipe }
func (fakeFIFOInfo) ModTime() time.Time { return time.Time{} }
func (fakeFIFOInfo) IsDir() bool        { return false }
func (fakeFIFOInfo) Sys() any           { return nil }

// Test_originRow_reports_older_as_warn_with_the_init_fix pins the one arm
// no black-box test can reach: OriginOlder is WARN, "installed by an older
// brief release" with suffix appended, and the caller-supplied olderFix —
// exercised once per shape a real row calls it with: host-hook/host-snippet's
// own single-subject call (no suffix, "run 'brief init'") and host-plugin/
// host-agents' own multi-file call (a joined-relpath suffix, each with its
// own fix).
func Test_originRow_reports_older_as_warn_with_the_init_fix(t *testing.T) {
	cases := []struct {
		name       string
		olderFix   string
		suffix     string
		wantDetail string
	}{
		{
			name:       "single-subject row (host-hook, host-snippet)",
			olderFix:   runInit,
			suffix:     "",
			wantDetail: "installed by an older brief release",
		},
		{
			name:       "multi-file row naming brief init (host-plugin)",
			olderFix:   runInit,
			suffix:     ": .claude/skills/brief/.claude-plugin/plugin.json",
			wantDetail: "installed by an older brief release: .claude/skills/brief/.claude-plugin/plugin.json",
		},
		{
			name:       "multi-file row naming --with-agents (host-agents)",
			olderFix:   runInitWithAgents,
			suffix:     ": .claude/skills/brief/agents/reviewer.md",
			wantDetail: "installed by an older brief release: .claude/skills/brief/agents/reviewer.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sev, detail, fix := originRow(artifact.OriginOlder, c.olderFix, c.suffix)

			assert.Equal(t, SeverityWarn, sev)
			assert.Equal(t, c.wantDetail, detail)
			require.NotNil(t, fix)
			assert.Equal(t, c.olderFix, *fix)
		})
	}
}

// Test_hostSkillRow_reports_an_older_render_as_warn pins host-skill's own
// OriginOlder arm, reachable only through this synthetic call. The empty
// fstest.MapFS passed as fsys is never read: OriginOlder resolves through
// originRow, never reaching the unreadable branch that would consult it.
func Test_hostSkillRow_reports_an_older_render_as_warn(t *testing.T) {
	state := integrationFileState{
		path:    "/repo/.claude/skills/brief-workflow/SKILL.md",
		present: true,
		regular: true,
		origin:  artifact.OriginOlder,
	}

	check := hostSkillRow(fstest.MapFS{}, "/repo", "/repo", state, true)

	assert.Equal(t, "host-skill", check.ID)
	assert.Equal(t, SeverityWarn, check.Severity)
	assert.Contains(t, check.Detail, "installed by an older brief release")
	require.NotNil(t, check.Fix)
	assert.Equal(t, runInit, *check.Fix)
}

// Test_nonRegularKind_reports_no_kind_for_a_fifo pins nonRegularKind's own
// default arm: a mode that is neither a symlink nor a directory (a fifo,
// standing in for the one shape a real CLAUDE.md candidate could take that
// is neither) reports "", the value notRegularDetail's own bare-sentence
// arm depends on for reachability. Mutation-verified: changing the default
// case to return a non-empty string (e.g. "fifo") reddens this test alone,
// restored after.
func Test_nonRegularKind_reports_no_kind_for_a_fifo(t *testing.T) {
	got := nonRegularKind(fakeFIFOInfo{})

	assert.Empty(t, got)
}

// Test_notRegularDetail_omits_the_parenthetical_when_kind_is_empty pins the
// fold notRegularKind's fifo case (above) reaches in practice: an empty
// kind renders the bare sentence, never a doubled parenthetical ("not a
// regular file (); brief block not installed"). Mutation-verified:
// unconditionally formatting with kind (dropping the `if kind == ""`
// guard) reddens this test alone, restored after.
func Test_notRegularDetail_omits_the_parenthetical_when_kind_is_empty(t *testing.T) {
	got := notRegularDetail("")

	assert.Equal(t, "not a regular file; brief block not installed", got)
}
