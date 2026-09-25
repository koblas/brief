package rwfstest

// Option declares one documented divergence between an adapter and
// rwfs.FS's own contract. Each constructor takes reason, which Contract
// logs (and reports via t.Skip when the divergence skips a whole row).
type Option func(*options)

// options collects every Option given to Contract or ReadContract; a field
// left "" means no divergence was declared for that row.
type options struct {
	skipInvalidNames           string
	pathIsAbsolute             string
	noOpenRoot                 string
	mkdirAllLeafSentinel       string
	writeFileAncestorSentinel  string
	openRootNotDirUnclassified string
}

// SkipInvalidNames declares that the adapter never checks fs.ValidPath, so
// Contract skips the whole invalid-name row rather than asserting it.
func SkipInvalidNames(reason string) Option {
	return func(o *options) { o.skipInvalidNames = reason }
}

// PathIsAbsolute declares that the adapter's *fs.PathError.Path is an
// absolute OS path rather than the caller's own name. Affected rows assert
// only that Path is a non-empty absolute path with a non-empty Op, instead
// of asserting Path equals name.
func PathIsAbsolute(reason string) Option {
	return func(o *options) { o.pathIsAbsolute = reason }
}

// NoOpenRoot declares that the adapter does not implement OpenRoot.
// Contract skips the OpenRoot row and the OpenRoot assertion inside the
// invalid-name row, while still checking that row's other five methods.
func NoOpenRoot(reason string) Option {
	return func(o *options) { o.noOpenRoot = reason }
}

// MkdirAllLeafSentinel declares that the adapter's MkdirAll reports
// syscall.ENOTDIR, not fs.ErrExist, when name already exists as a
// non-directory file. The row still asserts a matching *fs.PathError;
// only the sentinel checked differs.
func MkdirAllLeafSentinel(reason string) Option {
	return func(o *options) { o.mkdirAllLeafSentinel = reason }
}

// WriteFileAncestorSentinel declares that the adapter's WriteFile, when an
// ancestor of name is a non-directory file, reports an error with no
// sentinel a caller can check via errors.Is. The row still asserts a
// *fs.PathError; only the ENOTDIR check is dropped.
func WriteFileAncestorSentinel(reason string) Option {
	return func(o *options) { o.writeFileAncestorSentinel = reason }
}

// OpenRootNotDirUnclassified declares that the DirOpener's OpenRoot, when
// name exists as a file, reports an error that does not match
// syscall.ENOTDIR. ReadOpenRootContract still asserts a non-nil error that
// is not fs.ErrNotExist; only the ENOTDIR check is dropped.
func OpenRootNotDirUnclassified(reason string) Option {
	return func(o *options) { o.openRootNotDirUnclassified = reason }
}

// resolve folds opts into one options value.
func resolve(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// logExceptions logs every non-default field in o up front.
func logExceptions(t testingT, o options) {
	t.Helper()

	if o.skipInvalidNames != "" {
		t.Logf("rwfstest: SkipInvalidNames: %s", o.skipInvalidNames)
	}
	if o.pathIsAbsolute != "" {
		t.Logf("rwfstest: PathIsAbsolute: %s", o.pathIsAbsolute)
	}
	if o.noOpenRoot != "" {
		t.Logf("rwfstest: NoOpenRoot: %s", o.noOpenRoot)
	}
	if o.mkdirAllLeafSentinel != "" {
		t.Logf("rwfstest: MkdirAllLeafSentinel: %s", o.mkdirAllLeafSentinel)
	}
	if o.writeFileAncestorSentinel != "" {
		t.Logf("rwfstest: WriteFileAncestorSentinel: %s", o.writeFileAncestorSentinel)
	}
	if o.openRootNotDirUnclassified != "" {
		t.Logf("rwfstest: OpenRootNotDirUnclassified: %s", o.openRootNotDirUnclassified)
	}
}

// testingT is the *testing.T subset logExceptions needs, named so it reads
// as documentation rather than a stray *testing.T parameter.
type testingT interface {
	Helper()
	Logf(format string, args ...any)
}
