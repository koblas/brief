package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCheckJSONFixture writes two features under wd, named so fs.ReadDir's
// byte order is also Check's and the golden's own feature order: "alpha"
// is still in flight (an open step) with an over-cap state file — one
// ERROR finding carrying a line — and "beta" is fully done, with its
// state file missing entirely — one WARN, whole-file finding (line 0,
// "in_flight":false, the "missing STATE" shape rather than a feature-level
// producer, which always hard-codes in_flight true).
func newCheckJSONFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, alphaDir, "SCENARIO-01", "open", nil)

	betaDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "specification.md"), []byte(conformingSpec), 0o600))
	writeCheckStep(t, betaDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	return wd
}

// Test_check_json_document_golden is the exact-bytes golden pinning
// checkDocument's key order: an in-flight feature's ERROR finding
// carrying a line ("alpha") and a done feature's WARN, whole-file finding
// ("beta", "line":null). detail and path are captured from
// assemble.NewServer(...).Check against the same fixture, never a
// production literal.
func Test_check_json_document_golden(t *testing.T) {
	wd := newCheckJSONFixture(t)

	findings, checkErr := assemble.NewServer(config.Default(), wd).Check(t.Context(), "")
	require.NoError(t, checkErr)
	alphaFinding := findFinding(t, findings, "alpha")
	betaFinding := findFinding(t, findings, "beta")

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	betaDir := filepath.Join(wd, "docs", "specifications", "beta")

	want := `{"schema":1,"command":"check","ok":false,"exit_code":1,` +
		`"counts":{"error":1,"warn":1},` +
		`"features":[` +
		`{"name":"alpha","path":` + jsonString(t, alphaDir) + `,"in_flight":true,"findings":[` +
		`{"severity":"ERROR","rule":"state-cap","path":` + jsonString(t, alphaFinding.Path) + `,"line":81,"detail":` + jsonString(t, alphaFinding.Detail) + `}]},` +
		`{"name":"beta","path":` + jsonString(t, betaDir) + `,"in_flight":false,"findings":[` +
		`{"severity":"WARN","rule":"state-missing","path":` + jsonString(t, betaFinding.Path) + `,"line":null,"detail":` + jsonString(t, betaFinding.Detail) + `}]}` +
		`]}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_check_json_with_no_findings covers both zero-findings causes — a
// conforming repository and a repository with no features at all — each
// rendering "counts":{"error":0,"warn":0} and "features":[], never null,
// ok true, exit 0, zero stderr bytes.
func Test_check_json_with_no_findings(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name: "conforming repository",
			setup: func(t *testing.T) string {
				t.Helper()

				wd := t.TempDir()
				featureDir := filepath.Join(wd, "docs", "specifications", "demo")
				require.NoError(t, os.MkdirAll(featureDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
				writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

				return wd
			},
		},
		{
			name: "no features",
			setup: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := c.setup(t)
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{"check", "--json"}, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))

			want := `{"schema":1,"command":"check","ok":true,"exit_code":0,"counts":{"error":0,"warn":0},"features":[]}` + "\n"
			assert.Equal(t, want, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}

	wd := t.TempDir()
	var textStdout, textStderr bytes.Buffer

	textErr := cli.Run(t.Context(), wd, []string{"check"}, nil, &textStdout, &textStderr)

	require.NoError(t, textErr)
	assert.Equal(t, "brief check: no findings\n", textStderr.String())
}

// Test_check_json_warn_only_is_ok is R18's exit rule in JSON form: a
// fully-done feature carrying only a WARN finding renders ok true,
// exit_code 0, counts.error 0, counts.warn greater than zero, exit 0,
// zero stderr bytes.
func Test_check_json_warn_only_is_ok(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc struct {
		OK       bool `json:"ok"`
		ExitCode int  `json:"exit_code"`
		Counts   struct {
			Error int `json:"error"`
			Warn  int `json:"warn"`
		} `json:"counts"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.True(t, doc.OK)
	assert.Equal(t, 0, doc.ExitCode)
	assert.Equal(t, 0, doc.Counts.Error)
	assert.Positive(t, doc.Counts.Warn)
}

// Test_check_json_scopes_to_the_named_feature pins that naming a feature
// scopes both "counts" and "features" to it alone: "alpha" carries the
// two-feature fixture's only ERROR finding, "beta" the only WARN one, so
// this proves scoping rather than an empty run happening to match.
func Test_check_json_scopes_to_the_named_feature(t *testing.T) {
	wd := newCheckJSONFixture(t)

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "alpha", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc struct {
		Counts struct {
			Error int `json:"error"`
			Warn  int `json:"warn"`
		} `json:"counts"`
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.Equal(t, 1, doc.Counts.Error)
	assert.Equal(t, 0, doc.Counts.Warn)
	require.Len(t, doc.Features, 1)
	assert.Equal(t, "alpha", doc.Features[0].Name)
}
