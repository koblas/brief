package artifact_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A render whose bytes drift reclassifies every existing install as
// "edited locally". A deliberate change must move the old digest into
// that Kind's older list, not edit this table.
func Test_render_matches_the_digest_every_install_already_carries(t *testing.T) {
	tests := []struct {
		kind artifact.Kind
		want string
	}{
		{artifact.KindPluginManifest, "44b297b9c877f85db9b11df8f1a96d4c71a6409da04fbd5cc33617df78ef05f0"},
		{artifact.KindClaudeHooks, "f1bf824585e8783ff2edf31a0d02b81057a3384dfbb00e604b874a08b972bea4"},
		{artifact.KindSkillStart, "78dbf86361329f31c652f66218ff1da3583a2378527bd0412430981f21dda20e"},
		{artifact.KindSkillFinish, "c95f4760cb759eea9ab42719e786bd19750438f733d448de9ea169fbfbb60b52"},
		{artifact.KindSkillWorkflow, "2b02c4755e92155b637e17a81d9a27ca526562c47066c59bb424f2b7f640de1b"},
		{artifact.KindAgentPlanner, "635850759919c6b1d425557b7fd2658a94a51c01a3222e9ad96c786951861750"},
		{artifact.KindAgentImplementer, "0203af01bd856716a3c63499ecc074e4a4a5b2ded90dce8f5244eb27606ee415"},
		{artifact.KindAgentReviewer, "78e69ede6e3f19b59762fbff9a9cae530f38213fc58e9704cfa38f0c6e600364"},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			sum := sha256.Sum256(artifact.Render(tt.kind))

			assert.Equal(t, tt.want, hex.EncodeToString(sum[:]))
		})
	}
}

// Pins the earlier planner and implementer renders to their shipped
// digests, so init upgrades an old install in place rather than refusing it.
func Test_an_earlier_release_render_is_recognized_as_older(t *testing.T) {
	tests := []struct {
		kind artifact.Kind
		path string
		want string
	}{
		{artifact.KindAgentPlanner, "files/older/agents/planner.md", "ff048e14d5a249126f3eced2bef629e2becb685ac4b7047992e3dcf4e61da9dc"},
		{artifact.KindAgentImplementer, "files/older/agents/implementer.md", "3a63027cf01bf42e97e30a8dad337a5166b406febab6886fe0b009252395aace"},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			body, err := os.ReadFile(tt.path)
			require.NoError(t, err)

			sum := sha256.Sum256(body)

			assert.Equal(t, tt.want, hex.EncodeToString(sum[:]))
			assert.Equal(t, artifact.OriginOlder, artifact.Recognize(tt.kind, body))
		})
	}
}
