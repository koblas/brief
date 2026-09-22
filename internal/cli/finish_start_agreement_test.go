package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is SCENARIO-11's agreement pin between scaffold.Finish's next
// and assemble.Start's own next-open-step rule, the two independent
// definitions the "Design decision: where next is computed" section of
// SCENARIO-11.md accepts as a necessary duplication (scaffold and assemble
// may not import each other). It follows the check_drift_test.go
// precedent: capture both values from cli.Run rather than pinning either
// as a literal, so a change to one rule without the other reddens this
// test instead of silently drifting.

// Test_finish_next_agrees_with_start finishes one step, in --json mode,
// then briefs the same feature, also in --json mode, and asserts that
// finish's own "next" (id and title) equals what start would brief next —
// on a tree where the next step is blocked by an unmet depends-on, and on
// a tree where the next step is simply lower-numbered than the one just
// finished. It also briefs status --json for the same feature and asserts
// finish's own "next.path" agrees with status's own "next.path" — the
// third field MAJOR 2 adds to finish's object, sourced from a third,
// independent definition (assemble.Status's own featureStatus loop) that
// scaffold and assemble may not share either.
func Test_finish_next_agrees_with_start(t *testing.T) {
	tests := []struct {
		name          string
		build         func(t *testing.T) string
		finishFeature string
		finishStep    string
	}{
		{
			name: "a blocked step is still named next",
			build: func(t *testing.T) string {
				t.Helper()

				return newFinishCLIFixtureWithSteps(t,
					finishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
					finishStep{id: "SCENARIO-02", status: "open", dependsOn: "[SCENARIO-03]"},
					finishStep{id: "SCENARIO-03", status: "open", dependsOn: "[]"},
				)
			},
			finishFeature: "demo",
			finishStep:    "SCENARIO-01",
		},
		{
			name: "a lower-numbered open step is named next",
			build: func(t *testing.T) string {
				t.Helper()

				return newFinishCLIFixtureWithSteps(t,
					finishStep{id: "SCENARIO-01", status: "open", dependsOn: "[]"},
					finishStep{id: "SCENARIO-02", status: "open", dependsOn: "[]"},
				)
			},
			finishFeature: "demo",
			finishStep:    "SCENARIO-02",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := tc.build(t)
			handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
			statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")

			var finishStdout, finishStderr bytes.Buffer
			finishErr := cli.Run(t.Context(), wd,
				[]string{"finish", tc.finishFeature, tc.finishStep, "--handoff", handoffPath, "--state", statePath, "--json"},
				nil, &finishStdout, &finishStderr)
			require.NoError(t, finishErr)

			var finishDoc struct {
				Next *struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Path  string `json:"path"`
				} `json:"next"`
			}
			require.NoError(t, json.Unmarshal(finishStdout.Bytes(), &finishDoc))
			require.NotNil(t, finishDoc.Next, "the fixture always leaves another step open")

			var startStdout, startStderr bytes.Buffer
			startErr := cli.Run(t.Context(), wd, []string{"start", tc.finishFeature, "--json"}, nil, &startStdout, &startStderr)
			require.NoError(t, startErr)

			var startDoc struct {
				Step *struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"step"`
			}
			require.NoError(t, json.Unmarshal(startStdout.Bytes(), &startDoc))
			require.NotNil(t, startDoc.Step, "the fixture always leaves another step open")

			assert.Equal(t, startDoc.Step.ID, finishDoc.Next.ID)
			assert.Equal(t, startDoc.Step.Title, finishDoc.Next.Title)

			var statusStdout, statusStderr bytes.Buffer
			statusErr := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &statusStdout, &statusStderr)
			require.NoError(t, statusErr)

			var statusDoc struct {
				Features []struct {
					Name string `json:"name"`
					Next *struct {
						Path string `json:"path"`
					} `json:"next"`
				} `json:"features"`
			}
			require.NoError(t, json.Unmarshal(statusStdout.Bytes(), &statusDoc))

			var statusNextPath string

			for _, f := range statusDoc.Features {
				if f.Name == tc.finishFeature {
					require.NotNil(t, f.Next, "the fixture always leaves another step open")
					statusNextPath = f.Next.Path
				}
			}

			require.NotEmpty(t, statusNextPath, "status must report the same feature finish just closed a step in")
			assert.Equal(t, statusNextPath, finishDoc.Next.Path)
		})
	}
}
