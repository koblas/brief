package rwfstest

// Option declares one documented divergence between an adapter and rwfs.FS's
// own contract. Every constructor below takes reason, the citation a caller
// gives for why the divergence exists — Contract logs it via t.Logf, and,
// for an Option that skips a whole row rather than relaxing one assertion,
// via t.Skip on that row's own subtest, so a reader always sees which rows
// were not run at full strength and why.
type Option func(*options)

// options collects every Option Contract or ReadContract was given, each
// field defaulting to "" ("no divergence declared, run this row at full
// strength").
type options struct {
	skipInvalidNames           string
	pathIsAbsolute             string
	noOpenRoot                 string
	mkdirAllLeafSentinel       string
	writeFileAncestorSentinel  string
	openRootNotDirUnclassified string
}

// SkipInvalidNames declares that the adapter under test never checks
// fs.ValidPath itself, so the contract's whole invalid-name row — every
// write method's response to an empty, ".."-carrying, absolute, or
// doubled-separator name — cannot be asserted against it. Contract skips
// that row (t.Skip(reason)) rather than silently omitting it.
func SkipInvalidNames(reason string) Option {
	return func(o *options) { o.skipInvalidNames = reason }
}

// PathIsAbsolute declares that the adapter's own *fs.PathError.Path is an
// absolute OS path it built from the caller's name, not the name itself —
// diverging from rwfs.FS's own contract (fs.go), which promises Path is
// always the caller's own name unchanged. Every row that would otherwise
// assert pe.Path == name instead asserts only that pe.Path is a non-empty
// absolute path, still requiring a *fs.PathError with a non-empty Op.
func PathIsAbsolute(reason string) Option {
	return func(o *options) { o.pathIsAbsolute = reason }
}

// NoOpenRoot declares that the adapter under test does not implement
// OpenRoot at all. Contract skips the whole OpenRoot row (t.Skip(reason))
// and the OpenRoot assertion inside the invalid-name row — the latter
// gated independently of SkipInvalidNames, so an adapter that validates
// names but has no OpenRoot still gets the other five write methods'
// invalid-name rows checked.
func NoOpenRoot(reason string) Option {
	return func(o *options) { o.noOpenRoot = reason }
}

// MkdirAllLeafSentinel declares that the adapter's MkdirAll, when name
// already exists as a non-directory file, reports a sentinel other than
// fs.ErrExist — rwfs.OS and rwfs.Mem both report fs.ErrExist for this case,
// but an adapter built directly over os.MkdirAll (rather than
// os.Root.MkdirAll) reports syscall.ENOTDIR instead, since os.MkdirAll's
// own implementation treats the leaf like any other path segment. The row
// still asserts a *fs.PathError naming the same failure; only the sentinel
// checked differs.
func MkdirAllLeafSentinel(reason string) Option {
	return func(o *options) { o.mkdirAllLeafSentinel = reason }
}

// WriteFileAncestorSentinel declares that the adapter's WriteFile, when a
// proper ancestor of name exists as a non-directory file, reports an error
// that does not match syscall.ENOTDIR at all — rather than a different
// sentinel, no sentinel a caller can check with errors.Is. This is narrower
// than a raw os.Mkdir/os.Open/os.Stat failure through the same ancestor,
// which the kernel itself reports as ENOTDIR and every os.* wrapper
// preserves: it applies only to an adapter whose WriteFile opens the
// parent directory through a call that classifies the failure itself
// (os.OpenRoot, confirmed on go1.27.1/darwin to report a bare, unwrapped
// "not a directory" string here) rather than propagating the raw errno.
// The row still asserts a *fs.PathError; only the ENOTDIR check is
// dropped.
func WriteFileAncestorSentinel(reason string) Option {
	return func(o *options) { o.writeFileAncestorSentinel = reason }
}

// OpenRootNotDirUnclassified declares that the DirOpener's OpenRoot, when
// name exists as a file, reports an error that does not match
// syscall.ENOTDIR — a raw *os.Root.OpenRoot failure, confirmed on
// go1.27.1/darwin to report a bare, unwrapped "not a directory" string
// rather than the classified sentinel rwfs.OS's own classifyOpenRootErr
// produces for the identical case. ReadOpenRootContract still asserts a
// non-nil error and that it is not fs.ErrNotExist; only the ENOTDIR check
// is dropped.
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

// logExceptions logs every non-default field in o, so a reader of `go test
// -v` output sees every declared divergence up front, not only at the
// specific row it relaxes or skips.
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
