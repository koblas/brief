package stepfile_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
)

// Test_FirstUnmet_reports_nothing_when_a_dependency_is_recorded_done covers
// the satisfied case: a dependant declaring a dependency that was Recorded
// with a done Frontmatter is never unmet.
func Test_FirstUnmet_reports_nothing_when_a_dependency_is_recorded_done(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "done"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

// Test_FirstUnmet_reports_a_recorded_not_done_dependency_as_unmet covers the
// "exists but open" case: Known must be true and FirstUnmet must still
// report the id, distinguishing it from an id nobody recorded at all.
func Test_FirstUnmet_reports_a_recorded_not_done_dependency_as_unmet(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "open"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-01", dep)
	assert.True(t, idx.Known("STEP-01"))
}

// Test_FirstUnmet_reports_an_id_nobody_recorded_as_unmet_and_not_known
// covers the "names no step file" case: an id FirstUnmet reports must be
// distinguishable, via Known, from a recorded-but-open one, since the two
// cases render different refusal copy.
func Test_FirstUnmet_reports_an_id_nobody_recorded_as_unmet_and_not_known(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-99"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-99", dep)
	assert.False(t, idx.Known("STEP-99"))
}

// Test_FirstUnmet_treats_a_step_recorded_from_a_zero_Frontmatter_as_known_and_unmet
// covers a depended-on sibling whose file could not be read or parsed: it
// must be Recorded (a zero Frontmatter is never done), not skipped, so the
// dependant blocks with the "is not finished" copy rather than the
// "names no step file" copy.
func Test_FirstUnmet_treats_a_step_recorded_from_a_zero_Frontmatter_as_known_and_unmet(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-01", dep)
	assert.True(t, idx.Known("STEP-01"))
}

// Test_FirstUnmet_reports_nothing_for_a_done_dependant_with_an_unmet_dependency
// pins the done-step exemption: FirstUnmet short-circuits on fm.Done()
// before ever consulting DependsOn, whatever the recorded index says.
func Test_FirstUnmet_reports_nothing_for_a_done_dependant_with_an_unmet_dependency(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-01", stepfile.Frontmatter{Status: "open"})
	fm := stepfile.Frontmatter{Status: "done", DependsOn: []string{"STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

// Test_FirstUnmet_reports_nothing_for_an_empty_DependsOn covers the
// scaffolded shape: a Frontmatter with no declared dependencies is never
// unmet.
func Test_FirstUnmet_reports_nothing_for_an_empty_DependsOn(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open"}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}

// Test_FirstUnmet_reports_the_first_of_two_unmet_dependencies_in_declaration_order
// pins that FirstUnmet walks DependsOn in the order it was declared, not
// map order or some other reordering.
func Test_FirstUnmet_reports_the_first_of_two_unmet_dependencies_in_declaration_order(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-02", "STEP-01"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.True(t, unmet)
	assert.Equal(t, "STEP-02", dep)
}

// Test_the_index_keys_on_the_recorded_id_not_the_frontmatter_s_own_id pins
// that DependencyIndex is keyed by whatever id Record is called with —
// pattern.ID(n) at every real call site — never fm.ID: a step file whose
// frontmatter id disagrees with its filename must still resolve the same
// way finish's own step lookup does.
func Test_the_index_keys_on_the_recorded_id_not_the_frontmatter_s_own_id(t *testing.T) {
	idx := stepfile.NewDependencyIndex()
	idx.Record("STEP-02", stepfile.Frontmatter{ID: "WRONG-02", Status: "done"})
	fm := stepfile.Frontmatter{Status: "open", DependsOn: []string{"STEP-02"}}

	dep, unmet := idx.FirstUnmet(fm)

	assert.False(t, unmet)
	assert.Empty(t, dep)
}
