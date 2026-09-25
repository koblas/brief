package stepfile

// DependencyIndex records each step's doneness, keyed by id, so a
// dependant's Frontmatter can be checked against it.
type DependencyIndex struct {
	recorded map[string]bool
}

// NewDependencyIndex returns an empty DependencyIndex: no id is Known
// until Record is called for it.
func NewDependencyIndex() *DependencyIndex {
	return &DependencyIndex{recorded: make(map[string]bool)}
}

// Record stores id's doneness as fm.Done() reports it, whether or not it
// is done. A step file that could not be read or parsed is still
// recorded, as a zero Frontmatter (never done), never skipped.
func (idx *DependencyIndex) Record(id string, fm Frontmatter) {
	idx.recorded[id] = fm.Done()
}

// Known reports whether id has been Recorded at all, distinguishing a
// recorded-but-open dependency from one whose id names no step file.
func (idx *DependencyIndex) Known(id string) bool {
	_, ok := idx.recorded[id]

	return ok
}

// FirstUnmet returns the first id in fm.DependsOn, in declaration order,
// that is not a Recorded done step, and true. It returns ("", false) when
// fm.Done() is true or every declared dependency is Recorded and done.
// Known distinguishes an id that was never Recorded from one Recorded
// not-done.
func (idx *DependencyIndex) FirstUnmet(fm Frontmatter) (string, bool) {
	if fm.Done() {
		return "", false
	}

	for _, dep := range fm.DependsOn {
		if !idx.recorded[dep] {
			return dep, true
		}
	}

	return "", false
}
