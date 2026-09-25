package doctor

// White-box package. This file pins three unexported units host_test.go's
// own black-box table cannot reach economically:
//
//   - originRow: every compiled-in older-digest list but the planner and
//     implementer agents' own ships empty, so host-plugin's, host-hook's
//     and host-snippet's own OriginOlder arm is reachable only by a
//     synthetic call.
//   - hostSkillRow: same reason, for host-skill's own OriginOlder arm.
//   - nonRegularKind's default arm needs a mode (a named pipe) a
//     black-box fixture cannot portably construct; a fake os.FileInfo
//     reaches the same branch without that platform dependency.

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFIFOInfo is a minimal os.FileInfo reporting a named-pipe mode, the
// one non-symlink, non-directory shape nonRegularKind's default arm falls
// through to.
type fakeFIFOInfo struct{}

func (fakeFIFOInfo) Name() string       { return "CLAUDE.md" }
func (fakeFIFOInfo) Size() int64        { return 0 }
func (fakeFIFOInfo) Mode() os.FileMode  { return os.ModeNamedPipe }
func (fakeFIFOInfo) ModTime() time.Time { return time.Time{} }
func (fakeFIFOInfo) IsDir() bool        { return false }
func (fakeFIFOInfo) Sys() any           { return nil }

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

// The empty fstest.MapFS passed as fsys is never read: OriginOlder
// resolves through originRow, never reaching the unreadable branch.
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

func Test_nonRegularKind_reports_no_kind_for_a_fifo(t *testing.T) {
	got := nonRegularKind(fakeFIFOInfo{})

	assert.Empty(t, got)
}

func Test_notRegularDetail_omits_the_parenthetical_when_kind_is_empty(t *testing.T) {
	got := notRegularDetail("")

	assert.Equal(t, "not a regular file; brief block not installed", got)
}
