package stepfile

// DependencyIndex is the one definition of "blocked" a feature's step files
// share: a set of recorded steps, keyed by id, each carrying its own
// doneness, that a dependant's Frontmatter is checked against. Its two
// callers are assemble's status blocked count and scaffold's finish
// refusal (see doc.go) — both need the same answer to "is this dependency
// met" on the same tree, so the rule lives here rather than in either
// feature package, which cannot import the other.
type DependencyIndex struct {
	recorded map[string]bool
}

// NewDependencyIndex returns an empty DependencyIndex: no id is Known
// until Record is called for it.
func NewDependencyIndex() *DependencyIndex {
	return &DependencyIndex{recorded: make(map[string]bool)}
}

// Record stores id's doneness as fm.Done() reports it. Record stores every
// step it is given, whether or not it is done — Known is what lets a
// caller tell "recorded but open" apart from "no such step file", and
// recording only done steps would collapse that distinction. A step file
// that could not be read or whose frontmatter did not parse is still
// Recorded, from a zero Frontmatter (never done), rather than skipped: a
// caller checking Known must see it as an existing, not-done step, not as
// an unknown id.
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
// fm.Done() is true — a done step is never dependency-blocked, whatever
// its dependencies say, the same exemption assemble.Status's blocked count
// already applies — or when every declared dependency is Recorded and
// done. An id that was never Recorded and one that was Recorded not-done
// are both unmet here; Known is how a caller tells them apart to choose
// the right refusal copy.
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
