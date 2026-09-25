package artifact

// White-box package: classify is unexported decision logic extracted for
// combinatorial reasons. Most Kinds ship an empty older-digest list, so
// the current/older/edited precedence is impractical to drive through the
// public Recognize surface; this file drives classify directly instead.

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
