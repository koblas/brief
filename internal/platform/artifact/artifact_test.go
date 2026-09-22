package artifact_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_config_file_resolves_to_the_shipped_defaults guards the trap an
// all-comment ".brief.yaml" is exposed to: config.Inspect treats a
// zero-live-line file the same as a zero-byte one (io.EOF-as-empty), so
// ConfigFile's own bytes — as written, every line commented — must decode
// to config.Default() with no violations. A render that ever emits one
// live line (a "---" marker, say) would break this without touching the
// uncommented form at all.
func Test_config_file_resolves_to_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, artifact.ConfigFile(), 0o600))

	cfg, violations, err := config.Inspect(path)

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, config.Default(), cfg)
}

// Test_uncommenting_the_config_file_yields_the_shipped_defaults strips
// exactly one leading "#" from every line not beginning "# " — the
// uncomment rule R2's copy documents — and proves the result is valid YAML
// that decodes to config.Default() with no violations: the real round trip
// a repository owner performs by hand when they want to change a value.
func Test_uncommenting_the_config_file_yields_the_shipped_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, uncomment(artifact.ConfigFile()), 0o600))

	cfg, violations, err := config.Inspect(path)

	// optional-conventions round-trips through YAML's empty flow sequence
	// "[]" as a non-nil empty slice, never the nil Default() itself holds;
	// the two are the same "nothing set" value, so want is normalized to
	// match rather than treated as a mismatch.
	want := config.Default()
	want.OptionalConventions = []string{}

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, want, cfg)
}

// Test_every_config_key_is_documented enforces the doc/value line pairing
// structurally: every line of ConfigFile() is commented (R2's "no live
// line at all"), and every commented value line ("#<yaml>", no space) is
// immediately preceded by a doc prose line ("# ", hash space) — so no
// value line documents itself only by accident of being readable YAML.
func Test_every_config_key_is_documented(t *testing.T) {
	body := strings.TrimRight(string(artifact.ConfigFile()), "\n")
	lines := strings.Split(body, "\n")
	require.NotEmpty(t, lines)

	for i, line := range lines {
		require.Truef(t, strings.HasPrefix(line, "#"), "line %d (%q) must be commented", i, line)

		if strings.HasPrefix(line, "# ") {
			continue
		}

		require.Positivef(t, i, "value line %d (%q) has no preceding doc line", i, line)
		assert.Truef(t, strings.HasPrefix(lines[i-1], "# "), "value line %d (%q) must be preceded by a doc line, got %q", i, line, lines[i-1])
	}
}

// Test_the_current_config_render_is_a_known_digest pins that ConfigFile's
// own bytes, as shipped today, are recognized by Recognize as the current
// render — the compiled-in registry Recognize checks must always contain
// today's own digest.
func Test_the_current_config_render_is_a_known_digest(t *testing.T) {
	origin := artifact.Recognize(artifact.KindConfig, artifact.ConfigFile())

	assert.Equal(t, artifact.OriginCurrent, origin)
}

// Test_Recognize reports current for bytes matching a compiled-in digest
// and edited for anything else, by table: the current render on one side,
// an arbitrary edited body on the other.
func Test_Recognize(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want artifact.Origin
	}{
		{name: "current render", body: artifact.ConfigFile(), want: artifact.OriginCurrent},
		{name: "locally edited bytes", body: []byte("feature-directory: elsewhere\n"), want: artifact.OriginEdited},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, artifact.Recognize(artifact.KindConfig, c.body))
		})
	}
}

// uncomment strips exactly one leading "#" from every line not beginning
// "# " (hash, space) — the inverse of what ConfigFile writes: a "# " line
// is doc prose and stays a comment forever; every other line is a
// commented YAML line and becomes live.
func uncomment(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			continue
		}

		lines[i] = strings.TrimPrefix(line, "#")
	}

	return []byte(strings.Join(lines, "\n"))
}
