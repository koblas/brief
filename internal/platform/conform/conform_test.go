package conform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/conform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodyOfLines returns a body of exactly n distinct lines, with a trailing
// newline, mirroring scaffold_test's own helper of the same shape.
func bodyOfLines(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

// fixtureHeadings is a StateHeadings whose four entries differ from
// config.Default's, so a test built against it cannot pass on a hardcoded
// default.
func fixtureHeadings() config.StateHeadings {
	return config.StateHeadings{
		BindingDecisions: "## Decisions Fixture",
		LeftUnbuilt:      "## Left Fixture",
		Traps:            "## Gotchas",
		OpenDebts:        "## Debts Fixture",
	}
}

func Test_OverCap_accepts_a_body_of_exactly_the_configured_cap(t *testing.T) {
	v := conform.OverCap(bodyOfLines(10), "state", 10)

	assert.Nil(t, v)
}

func Test_OverCap_refuses_a_body_one_line_over_the_configured_cap(t *testing.T) {
	v := conform.OverCap(bodyOfLines(11), "state", 10)

	require.NotNil(t, v)
	require.ErrorIs(t, v.Err, conform.ErrOverCap)
	assert.Equal(t, 0, v.Line)
	assert.Equal(t, "state is 11 lines, over the cap of 10", v.Problem)
	assert.Equal(t,
		"cut the state to 10 lines or fewer, or raise state-cap-lines in .brief.yaml, and retry",
		v.Fix)
}

func Test_UnterminatedFence_reports_the_line_and_delimiter_of_an_unclosed_fence(t *testing.T) {
	body := "line one\n```\nunterminated\n"

	v := conform.UnterminatedFence([]byte(body), "state")

	require.NotNil(t, v)
	require.ErrorIs(t, v.Err, conform.ErrUnterminatedFence)
	assert.Equal(t, 2, v.Line)
	assert.Equal(t, "state has an unclosed ``` fence", v.Problem)
}

func Test_UnterminatedFence_returns_nil_when_every_fence_closes(t *testing.T) {
	body := "line one\n```\nclosed\n```\n"

	v := conform.UnterminatedFence([]byte(body), "state")

	assert.Nil(t, v)
}

func Test_MissingHeading_reports_only_the_first_missing_heading(t *testing.T) {
	headings := fixtureHeadings()
	body := headings.LeftUnbuilt + "\n\ncontent\n\n" +
		headings.OpenDebts + "\n\ncontent\n"

	v := conform.MissingHeading([]byte(body), "state", headings)

	require.NotNil(t, v)
	require.ErrorIs(t, v.Err, conform.ErrMissingStateHeading)
	assert.Equal(t, 0, v.Line)
	assert.Equal(t, `state is missing the "## Decisions Fixture" section`, v.Problem)
}

func Test_MissingHeading_returns_nil_when_every_heading_is_present(t *testing.T) {
	headings := fixtureHeadings()

	var body strings.Builder
	for _, h := range headings.Ordered() {
		body.WriteString(h + "\n\n")
	}

	v := conform.MissingHeading([]byte(body.String()), "state", headings)

	assert.Nil(t, v)
}

func Test_OpenChecklistItem_returns_nil_when_the_checklist_heading_is_absent(t *testing.T) {
	body := "# Step\n\nno checklist heading here\n\n- [ ] stray item\n"

	v := conform.OpenChecklistItem([]byte(body), "## Implementation Plan")

	assert.Nil(t, v)
}

func Test_OpenChecklistItem_returns_nil_when_the_checklist_has_no_items(t *testing.T) {
	body := "# Step\n\n## Implementation Plan\n\nnothing here yet\n"

	v := conform.OpenChecklistItem([]byte(body), "## Implementation Plan")

	assert.Nil(t, v)
}

func Test_OpenChecklistItem_reports_the_first_unticked_item(t *testing.T) {
	body := "# Step\n\n## Implementation Plan\n\n- [x] done\n- [ ] second thing\n"

	v := conform.OpenChecklistItem([]byte(body), "## Implementation Plan")

	require.NotNil(t, v)
	require.ErrorIs(t, v.Err, conform.ErrOpenChecklistItem)
	assert.Equal(t, 6, v.Line)
	assert.Equal(t, `checklist item "second thing" is not ticked`, v.Problem)
}

func Test_OpenChecklistItem_omits_the_quoted_text_for_a_bare_item(t *testing.T) {
	body := "# Step\n\n## Implementation Plan\n\n- [ ]\n"

	v := conform.OpenChecklistItem([]byte(body), "## Implementation Plan")

	require.NotNil(t, v)
	assert.Equal(t, "checklist item is not ticked", v.Problem)
}
