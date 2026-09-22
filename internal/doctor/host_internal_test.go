package doctor

// White-box package: originRow is host.go's own unexported origin→row
// mapping. Every compiled-in older digest list (internal/platform/artifact)
// ships empty (STATE.md), so no real file's bytes ever classify as
// artifact.OriginOlder through artifact.Recognize or artifact.RecognizeSnippet
// today — the OriginOlder arm host-plugin, host-hook, host-snippet and
// host-agents all share is reachable only by calling originRow directly
// with a synthetic artifact.OriginOlder value, as this file does. Every
// other arm (present/missing, edited, current) is covered black-box in
// host_test.go through the real artifact.Recognize/RecognizeSnippet path.

import (
	"os"
	"testing"
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
