package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStatusJSONFixture writes four features under wd's default layout, named
// so fs.ReadDir's byte order is also the golden order: "alpha" (1/4 done,
// next SCENARIO-02, one step blocked on its own unfinished SCENARIO-02),
// "beta" (2/2 done, complete), "delta" (malformed — no frontmatter) and
// "epsilon" (a bare feature directory with no step files at all).
func newStatusJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()

	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-03.md", "SCENARIO-03", "open", []string{"SCENARIO-02"})
	writeStatusStep(t, wd, "alpha", "SCENARIO-04.md", "SCENARIO-04", "open", nil)

	writeStatusStep(t, wd, "beta", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "beta", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	writeMalformedStatusFeature(t, wd, "delta")

	writeConformingFeatureFiles(t, filepath.Join(wd, "docs", "specifications", "epsilon"))

	return wd
}

// captureMalformedDetailFix runs "brief status" (text mode) against wd and
// pulls detail/fix out of the one stderr line naming feature's step file —
// the same way a --json golden must be built from a captured value rather
// than a literal copied out of newProblem. relStepPath is the step file's
// path relative to wd, exactly as the text-mode line names it.
func captureMalformedDetailFix(t *testing.T, wd, feature, relStepPath string) (string, string) {
	t.Helper()

	var textStdout, textStderr bytes.Buffer
	textErr := cli.Run(t.Context(), wd, []string{"status"}, nil, &textStdout, &textStderr)
	require.NoError(t, textErr)

	lines := strings.Split(strings.TrimSuffix(textStderr.String(), "\n"), "\n")
	require.NotEmpty(t, lines)

	prefix := "brief status: " + feature + ": " + relStepPath + ": "
	require.True(t, strings.HasPrefix(lines[0], prefix), "line %q missing prefix %q", lines[0], prefix)

	rest := strings.TrimPrefix(lines[0], prefix)
	parts := strings.SplitN(rest, "; ", 2)
	require.Len(t, parts, 2)

	return parts[0], parts[1]
}

// Test_status_json_document_golden is the exact-bytes golden pinning
// statusDocument's key order: an in-progress feature with a blocked step
// and a non-null next, a complete feature (next null), a malformed feature
// (counts null, a problem object, line null) and a well-formed zero-step
// feature (0/0/0, complete false, next null).
func Test_status_json_document_golden(t *testing.T) {
	wd := newStatusJSONFixture(t)

	deltaRelStep := filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")
	detail, fix := captureMalformedDetailFix(t, wd, "delta", deltaRelStep)

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	featureDirAlpha := filepath.Join(wd, "docs", "specifications", "alpha")
	featureDirBeta := filepath.Join(wd, "docs", "specifications", "beta")
	featureDirDelta := filepath.Join(wd, "docs", "specifications", "delta")
	featureDirEpsilon := filepath.Join(wd, "docs", "specifications", "epsilon")
	nextPathAlpha := filepath.Join(featureDirAlpha, "SCENARIO-02.md")
	problemPathDelta := filepath.Join(featureDirDelta, "SCENARIO-01.md")

	want := `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[` +
		`{"name":"alpha","path":` + jsonString(t, featureDirAlpha) + `,"done":1,"total":4,"blocked":1,"complete":false,` +
		`"next":{"id":"SCENARIO-02","title":` + jsonString(t, stepTitle("SCENARIO-02")) + `,"path":` + jsonString(t, nextPathAlpha) + `},"problem":null},` +
		`{"name":"beta","path":` + jsonString(t, featureDirBeta) + `,"done":2,"total":2,"blocked":0,"complete":true,"next":null,"problem":null},` +
		`{"name":"delta","path":` + jsonString(t, featureDirDelta) + `,"done":null,"total":null,"blocked":null,"complete":false,"next":null,` +
		`"problem":{"path":` + jsonString(t, problemPathDelta) + `,"line":null,"detail":` + jsonString(t, detail) + `,"fix":` + jsonString(t, fix) + `}},` +
		`{"name":"epsilon","path":` + jsonString(t, featureDirEpsilon) + `,"done":0,"total":0,"blocked":0,"complete":false,"next":null,"problem":null}` +
		`]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_status_json_with_no_features covers R9's empty discriminator in
// JSON form for both zero-row causes: the feature root missing entirely and
// the feature root present but empty — both render "features":[], never
// null, with zero stderr bytes. The control arm proves the empty stderr is
// the --json branch, not the zero-rows notice going missing for some other
// reason: the same fixture's text-mode run still writes the notice.
func Test_status_json_with_no_features(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name: "feature root absent",
			setup: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
		},
		{
			name: "feature root empty",
			setup: func(t *testing.T) string {
				t.Helper()

				wd := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

				return wd
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := c.setup(t)
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))

			want := `{"schema":1,"command":"status","ok":true,"exit_code":0,"features":[]}` + "\n"
			assert.Equal(t, want, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}

	wd := t.TempDir()
	var textStdout, textStderr bytes.Buffer

	textErr := cli.Run(t.Context(), wd, []string{"status"}, nil, &textStdout, &textStderr)

	require.NoError(t, textErr)
	assert.NotEmpty(t, textStderr.String())
}

// decodedStatusNext is the "next" shape decoded out of a status --json
// document; only the fields a given test needs to inspect.
type decodedStatusNext struct {
	Title string `json:"title"`
}

// decodedStatusFeature is one "features[]" row decoded out of a status
// --json document; only the fields a given test needs to inspect.
type decodedStatusFeature struct {
	Next *decodedStatusNext `json:"next"`
}

// decodedStatusDocument is a status --json document decoded down to the
// fields Test_status_json_keeps_the_next_title_raw inspects.
type decodedStatusDocument struct {
	Features []decodedStatusFeature `json:"features"`
}

// Test_status_json_keeps_the_next_title_raw pins that --json's next.title
// carries the step heading exactly as markdown.Title returns it, interior
// tab included, while the text-mode NEXT column flattens that same tab to a
// space (flattenTabwriterField) — the control arm proving the two render
// differently for the identical fixture, only --json differing.
func Test_status_json_keeps_the_next_title_raw(t *testing.T) {
	wd := t.TempDir()
	rawTitle := "Implement\tSCENARIO-01"
	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	writeConformingFeatureFiles(t, featureDir)
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# " + rawTitle + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	var textStdout, textStderr bytes.Buffer
	textErr := cli.Run(t.Context(), wd, []string{"status"}, nil, &textStdout, &textStderr)

	require.NoError(t, textErr)
	assert.Contains(t, textStdout.String(), "Implement SCENARIO-01")
	assert.NotContains(t, textStdout.String(), rawTitle)

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	var doc decodedStatusDocument
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	require.Len(t, doc.Features, 1)
	require.NotNil(t, doc.Features[0].Next)
	assert.Equal(t, rawTitle, doc.Features[0].Next.Title)
}

// Test_status_json_counts_are_null_only_on_a_malformed_row decodes each row
// into a map so "null" is distinguishable from the number 0: a malformed
// row's done/total/blocked are literally null and its path is still the
// absolute feature directory (never the step file newProblem names); a
// well-formed zero-step row's are literally 0, the one variable — malformed
// or not — the two rows differ on.
func Test_status_json_counts_are_null_only_on_a_malformed_row(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")
	writeConformingFeatureFiles(t, filepath.Join(wd, "docs", "specifications", "epsilon"))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"status", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	var doc struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	require.Len(t, doc.Features, 2)

	delta := doc.Features[0]
	assert.Equal(t, "null", string(delta["done"]))
	assert.Equal(t, "null", string(delta["total"]))
	assert.Equal(t, "null", string(delta["blocked"]))
	assert.Equal(t, jsonString(t, filepath.Join(wd, "docs", "specifications", "delta")), string(delta["path"]))

	epsilon := doc.Features[1]
	assert.Equal(t, "0", string(epsilon["done"]))
	assert.Equal(t, "0", string(epsilon["total"]))
	assert.Equal(t, "0", string(epsilon["blocked"]))
}
