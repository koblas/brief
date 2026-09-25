// This file reaches classifyRefusal and reporter.refusal directly to pin
// the "(no files changed)" tail rule: it drops once an earlier write has
// landed, in both text mode and --json's "message".

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
		Fix:     "rerun 'brief init'",
		Err:     setup.ErrConcurrentEdit,
	}
}

// newPartialConcurrentEditRefusal stands in for setup.markPartial's own
// output: a *setup.RefusalError wrapped in setup.ErrPartialWrite.
func newPartialConcurrentEditRefusal() error {
	return fmt.Errorf("%w: %w", newConcurrentEditRefusal(), setup.ErrPartialWrite)
}

func Test_classifyRefusal_omits_the_tail_when_the_write_was_partial(t *testing.T) {
	c := classifyRefusal(newPartialConcurrentEditRefusal())

	assert.Empty(t, c.tail)
}

// Control for the test above: the same *setup.RefusalError, not wrapped
// in setup.ErrPartialWrite, still carries the tail.
func Test_classifyRefusal_keeps_the_tail_for_an_ordinary_refusal(t *testing.T) {
	c := classifyRefusal(newConcurrentEditRefusal())

	assert.Equal(t, noFilesChangedTail, c.tail)
}

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
