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
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
