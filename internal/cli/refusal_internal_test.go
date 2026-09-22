// This file reaches classifyRefusal and reporter.refusal directly to pin
// R14a's tail rule at the unit level: a *setup.RefusalError's own "(no
// files changed)" tail is only ever true when the refusal it names is the
// whole story. Once apply has already landed an earlier write (setup.go's
// own markPartial wraps the same *RefusalError with setup.ErrPartialWrite),
// that promise is false, and both the text-mode line and the --json
// document's "message" must stop making it, even though "message" and
// "files_changed" are rendered through two independent code paths
// (classifyRefusal's tail and filesChangedFor's own errors.Is check) that
// must agree.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newConcurrentEditRefusal returns the *setup.RefusalError apply's own
// CLAUDE.md read-modify-write guard (verifySnippetUnchanged) produces.
func newConcurrentEditRefusal() *setup.RefusalError {
	return &setup.RefusalError{
		Path:    "/repo/CLAUDE.md",
		Problem: "changed since it was planned",
		Fix:     "re-run 'brief init'",
		Err:     setup.ErrConcurrentEdit,
	}
}

// newPartialConcurrentEditRefusal stands in for what setup.markPartial
// produces once an earlier artifact already landed: not markPartial's own
// unexported type, but an error chain matching it on the two predicates
// classifyRefusal and filesChangedFor actually branch on —
// errors.AsType[*setup.RefusalError] finds the inner refusal, and
// errors.Is(_, setup.ErrPartialWrite) is true. That real pairing — a
// *setup.RefusalError reaching classifyRefusal already wrapped in
// ErrPartialWrite — is proven constructible from setup's own write path
// (not merely type-equivalent) by
// Test_apply_wraps_ErrPartialWrite_when_an_earlier_write_already_landed in
// internal/setup/apply_internal_test.go; nothing black-box through cli.Run
// can reach it, since the race it reports (CLAUDE.md edited between
// planning and apply) has no seam this package can drive from outside.
func newPartialConcurrentEditRefusal() error {
	return fmt.Errorf("%w: %w", newConcurrentEditRefusal(), setup.ErrPartialWrite)
}

// Test_classifyRefusal_omits_the_tail_when_the_write_was_partial is the
// claim: wrapping a *setup.RefusalError in setup.ErrPartialWrite (apply's
// own markPartial) must drop the tail, since files did change.
func Test_classifyRefusal_omits_the_tail_when_the_write_was_partial(t *testing.T) {
	c := classifyRefusal(newPartialConcurrentEditRefusal())

	assert.Empty(t, c.tail)
}

// Test_classifyRefusal_keeps_the_tail_for_an_ordinary_refusal is the
// control this claim needs: the very same *setup.RefusalError, not wrapped
// in setup.ErrPartialWrite, still carries the tail — proving the branch
// above is reached because of ErrPartialWrite specifically, not because
// *setup.RefusalError stopped carrying a tail altogether.
func Test_classifyRefusal_keeps_the_tail_for_an_ordinary_refusal(t *testing.T) {
	c := classifyRefusal(newConcurrentEditRefusal())

	assert.Equal(t, noFilesChangedTail, c.tail)
}

// Test_reporter_refusal_is_consistent_for_a_partial_write proves the fix at
// the boundary both modes actually render through: text mode's stderr line
// never claims "(no files changed)" once ErrPartialWrite is in the chain,
// and --json's own document keeps "files_changed":true and a "message"
// with the same missing tail — the two independent code paths
// (classifyRefusal's tail, filesChangedFor's errors.Is check) agreeing
// rather than each telling a different story about the same run.
func Test_reporter_refusal_is_consistent_for_a_partial_write(t *testing.T) {
	cmd := &cobra.Command{Use: "init"}
	cmd.Annotations = map[string]string{writesFilesAnnotation: "true"}
	err := newPartialConcurrentEditRefusal()

	var stderr bytes.Buffer
	textReporter := reporter{stderr: &stderr, wd: "/repo", cmd: cmd}
	_ = textReporter.refusal(err)

	assert.NotContains(t, stderr.String(), noFilesChangedTail)
	assert.Contains(t, stderr.String(), "brief init: ")

	var stdout bytes.Buffer
	jsonReporter := reporter{stdout: &stdout, wd: "/repo", cmd: cmd, json: true}
	_ = jsonReporter.refusal(err)

	var doc struct {
		Error struct {
			Message      string `json:"message"`
			FilesChanged *bool  `json:"files_changed"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	require.NotNil(t, doc.Error.FilesChanged)
	assert.True(t, *doc.Error.FilesChanged)
	assert.NotContains(t, doc.Error.Message, noFilesChangedTail)
}
