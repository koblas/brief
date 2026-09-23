package artifact

// White-box package: classify is digestsFor's own unexported per-Kind
// decision extracted for combinatorial reasons — the current/older/edited
// precedence and the in-both-lists rule are impractical to drive through
// the public Recognize surface for a Kind whose older…Digests list is still
// empty (every Kind but KindAgentPlanner/KindAgentImplementer today; those
// two are covered directly, through Recognize, by agents_test.go's
// Test_previous_release_agent_renders_classify_as_older). This file drives
// the extracted logic directly with a synthetic older list; snippet_test.go
// and artifact_test.go cover the public surface.

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_classify_reports_older_for_a_digest_only_in_the_older_list pins
// classify's own precedence: a digest in current is OriginCurrent even when
// the same digest also appears in older; a digest only in older is
// OriginOlder; a digest in neither is OriginEdited.
func Test_classify_reports_older_for_a_digest_only_in_the_older_list(t *testing.T) {
	currentBody := []byte("current render")
	olderBody := []byte("older render")
	editedBody := []byte("something else entirely")
	bothBody := []byte("in both lists")

	current := [][32]byte{sha256.Sum256(currentBody), sha256.Sum256(bothBody)}
	older := [][32]byte{sha256.Sum256(olderBody), sha256.Sum256(bothBody)}

	cases := []struct {
		name string
		body []byte
		want Origin
	}{
		{name: "current digest", body: currentBody, want: OriginCurrent},
		{name: "older-only digest", body: olderBody, want: OriginOlder},
		{name: "digest in neither list", body: editedBody, want: OriginEdited},
		{name: "digest in both lists is current", body: bothBody, want: OriginCurrent},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, classify(current, older, c.body))
		})
	}
}
