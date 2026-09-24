// Package assemble reads a feature directory and computes the brief an
// implementer needs to start its next step: the next open step's id,
// title, acceptance criteria and checklist, plus the decisions and
// constraints inherited from the feature's state file. It also computes,
// across every feature, the summary "brief status" prints: steps done over
// total, the next open step, and how many steps are blocked on an
// unfinished dependency.
//
// Start, Status and Features are the entry points. Start refuses a
// feature that does not exist with ErrNoSuchFeature. It refuses, with a
// *RefusalError
// wrapping ErrMalformedFeature, a feature whose structure it cannot
// assemble around rather than return a brief that silently omits or
// misreports part of it: a specification that is missing, unreadable,
// carries an unterminated fenced code block, or has no configured progress
// heading; a state file that is missing, unreadable, or carries an
// unterminated fence; or a briefed step whose frontmatter carries no "id:"
// or whose checklist heading is absent. A step file whose frontmatter is
// absent or does not parse also refuses, wrapping whatever sentinel
// readSteps produced, including stepfile.ErrNoFrontmatter. An absent
// acceptance heading in the briefed step, or an absent heading in the
// state file, does not refuse: Start reports each as a Shortfall in
// Brief.Shortfalls and still returns a Brief carrying every other section.
// A Shortfall fires on a heading's absence, never on an empty body — a
// heading present with nothing under it stays conforming. A step's
// frontmatter is parsed before any markdown
// extraction runs, so a "#" character inside a YAML value is never mistaken
// for a heading.
//
// Status takes the opposite stance on the same failure: a feature
// directory or step file that cannot be read or parsed becomes that row's
// Problem rather than failing the whole call, because one malformed
// feature must not blind "brief status" to every other feature in the
// repository. Start's refusal and Status's degradation both read the same
// step files through readSteps, which stays intolerant of a parse failure
// either way — the difference is which caller turns that failure into a
// refusal (Start) and which caller catches it and marks a row (Status).
//
// Features lists the names of a repository's known feature directories,
// the layout-reading step cli's not-found copy builds its "known: …"
// suggestion from — cli never lists a directory itself.
//
// Check is the backstop R18 assigns the read path: it reports every fault
// in a feature's on-disk layout that scaffold.Finish would now refuse to
// write over, so a tree that predates a cap or a rule is still surfaced
// rather than silently grandfathered in. Unlike Start and Status, Check
// walks a feature's step files itself rather than through readSteps, so
// one step file whose frontmatter cannot be read or parsed becomes its own
// finding rather than collapsing every other step's findings behind it.
// Every predicate Check shares with Finish lives in
// internal/platform/conform, the one place each is defined, since
// assemble and scaffold cannot import each other.
//
// assemble imports internal/platform/config, internal/platform/stepfile
// and internal/platform/markdown, and the standard library only. It never
// imports internal/scaffold or internal/cli: assemble owns reading a
// feature, scaffold owns writing one, and the two packages never import
// each other.
//
// FeatureFS pairs one feature's own filesystem with the absolute OS
// directory it is rooted at, and StartFS, CheckFS and StatusFS are Start's,
// Check's and Status's own content-reading cores: each takes a FeatureFS
// (CheckFS and StatusFS also take the compiled step-file pattern their
// caller shares across every feature) and reads through its FS with
// io/fs — fs.ReadFile, fs.ReadDir — never through an *os.Root directly, so
// any fs.FS, real or in-memory, drives them. Start, Check and Status stay
// the adapters around this seam: each opens the configured feature
// directory through Server's own openFeatureDir, then one feature's own
// subdirectory nested inside it — so a step file symlinked outside the
// feature directory is never reachable in production — computes
// FeatureFS{FS: root.FS(), Path: <the feature's own absolute directory>},
// and delegates. openFeatureDir returns a dirFS (fs.go): production wraps a
// real, nested *os.Root in osRoot, reproducing every error byte-for-byte,
// including the *os.Root-typed openRoot test-injection seam Start, Check
// and Status all share (export_test.go's SetOpenRootForTest) for
// simulating a permission-denied open without depending on OS permission
// bits or effective uid; NewServer's own WithFS Option substitutes a memDirFS
// wrapping an rwfs.Mem fixture instead, read by a command-level test
// (internal/cli) exactly as scaffold's own WithFS is — the symlink guard
// and the openRoot seam apply only to the production, os.Root-backed
// adapter. FeaturesFS is Features' own core, listing fsys directly with no
// nested open, so it takes a bare fs.FS.
package assemble
