package stepfile_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseFrontmatter_parses_id_status_and_depends_on(t *testing.T) {
	body := []byte("---\nid: STEP-03\nstatus: open\ndepends-on: [STEP-01, STEP-02]\n---\n\n# STEP-03\n")

	fm, rest, err := stepfile.ParseFrontmatter(body)

	require.NoError(t, err)
	assert.Equal(t, stepfile.Frontmatter{ID: "STEP-03", Status: "open", DependsOn: []string{"STEP-01", "STEP-02"}}, fm)
	assert.Equal(t, "\n# STEP-03\n", string(rest))
}

func Test_ParseFrontmatter_returns_no_frontmatter_error_when_the_body_has_no_leading_delimiter(t *testing.T) {
	body := []byte("# STEP-03\n\nno frontmatter here\n")

	_, _, err := stepfile.ParseFrontmatter(body)

	require.ErrorIs(t, err, stepfile.ErrNoFrontmatter)
}

func Test_ParseFrontmatter_returns_an_error_for_malformed_yaml(t *testing.T) {
	body := []byte("---\nid: [this is not: valid\n---\n\n# STEP-03\n")

	_, _, err := stepfile.ParseFrontmatter(body)

	require.Error(t, err)
}

func Test_Frontmatter_Done(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "done", status: "done", want: true},
		// Whitespace and case are kept as separate cases so each is pinned on its own.
		{name: "done with surrounding whitespace", status: " done ", want: true},
		{name: "done in mixed case", status: "Done", want: true},
		{name: "open", status: "open", want: false},
		{name: "blocked", status: "blocked", want: false},
		{name: "an empty status", status: "", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := stepfile.Frontmatter{Status: c.status}

			assert.Equal(t, c.want, fm.Done())
		})
	}
}

func Test_SetStatus_replaces_the_status_line_and_leaves_every_other_byte_identical(t *testing.T) {
	body := []byte("---\nid: STEP-02\nstatus: open\ndepends-on: [STEP-01]\nowner: planner\n---\n\n# STEP-02\n")

	got, err := stepfile.SetStatus(body, "done")

	require.NoError(t, err)
	want := []byte("---\nid: STEP-02\nstatus: done\ndepends-on: [STEP-01]\nowner: planner\n---\n\n# STEP-02\n")
	assert.Equal(t, want, got)
}

func Test_SetStatus_does_not_touch_a_status_line_after_the_closing_delimiter(t *testing.T) {
	body := []byte("---\nid: STEP-02\nstatus: open\n---\n\nstatus: not-yaml\n")

	got, err := stepfile.SetStatus(body, "done")

	require.NoError(t, err)
	want := []byte("---\nid: STEP-02\nstatus: done\n---\n\nstatus: not-yaml\n")
	assert.Equal(t, want, got)
}

func Test_SetStatus_returns_an_error_when_the_frontmatter_has_no_status_key(t *testing.T) {
	body := []byte("---\nid: STEP-02\ndepends-on: []\n---\n\n# STEP-02\n")

	_, err := stepfile.SetStatus(body, "done")

	require.ErrorIs(t, err, stepfile.ErrNoStatusField)
}

func Test_ParseFrontmatter_parses_a_CRLF_step_file(t *testing.T) {
	body := []byte("---\r\nid: STEP-03\r\nstatus: open\r\ndepends-on: []\r\n---\r\n# STEP-03\r\n")

	fm, rest, err := stepfile.ParseFrontmatter(body)

	require.NoError(t, err)
	assert.Equal(t, stepfile.Frontmatter{ID: "STEP-03", Status: "open", DependsOn: []string{}}, fm)
	// rest keeps a leading "\r"; readers trim it per line.
	assert.Equal(t, "\r\n# STEP-03\r\n", string(rest))
}

func Test_SetStatus_replaces_the_status_line_in_a_CRLF_step_file(t *testing.T) {
	body := []byte("---\r\nid: STEP-02\r\nstatus: open\r\n---\r\n\r\n# STEP-02\r\n")

	got, err := stepfile.SetStatus(body, "done")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(got), "---\r\n"))
	assert.Contains(t, string(got), "status: done\r\n")
}
