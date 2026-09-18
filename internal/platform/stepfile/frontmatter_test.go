package stepfile_test

import (
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

func Test_Frontmatter_Done_is_true_for_done(t *testing.T) {
	fm := stepfile.Frontmatter{Status: "done"}

	assert.True(t, fm.Done())
}

func Test_Frontmatter_Done_is_true_for_done_with_surrounding_whitespace_and_case(t *testing.T) {
	fm := stepfile.Frontmatter{Status: " Done "}

	assert.True(t, fm.Done())
}

func Test_Frontmatter_Done_is_false_for_open(t *testing.T) {
	fm := stepfile.Frontmatter{Status: "open"}

	assert.False(t, fm.Done())
}

func Test_Frontmatter_Done_is_false_for_blocked(t *testing.T) {
	fm := stepfile.Frontmatter{Status: "blocked"}

	assert.False(t, fm.Done())
}

func Test_Frontmatter_Done_is_false_for_empty_status(t *testing.T) {
	fm := stepfile.Frontmatter{Status: ""}

	assert.False(t, fm.Done())
}
