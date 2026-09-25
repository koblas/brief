package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
)

func Test_FirstUnmet_reports_nothing_when_a_dependency_is_recorded_done(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "done"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

func Test_FirstUnmet_reports_a_recorded_not_done_dependency_as_unmet(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "open"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-01", dep)
	assert.True(t, idx.Known("STEP-01"))
}

func Test_FirstUnmet_reports_an_id_nobody_recorded_as_unmet_and_not_known(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-99"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-99", dep)
	assert.False(t, idx.Known("STEP-99"))
}

func Test_FirstUnmet_treats_a_step_recorded_from_a_zero_Frontmatter_as_known_and_unmet(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-01", dep)
	assert.True(t, idx.Known("STEP-01"))
}

func Test_FirstUnmet_reports_nothing_for_a_done_dependant_with_an_unmet_dependency(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "open"})
	fm := stepfile.Frontmatter{Status: "done", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

func Test_FirstUnmet_reports_nothing_for_an_empty_DependsOn(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open"}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

func Test_FirstUnmet_reports_the_first_of_two_unmet_dependencies_in_declaration_order(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-02", "STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-02", dep)
}

// Pins that the index is keyed by the id passed to Record, never fm.ID.
func Test_the_index_keys_on_the_recorded_id_not_the_frontmatter_s_own_id(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-02", stepfile.Frontmatter{ID: "WRONG-02", Status: "done"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-02"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}
