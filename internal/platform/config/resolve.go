package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/repo"
	"gopkg.in/yaml.v3"
)

// configFileName is the one name Resolve looks for. init writes exactly
// this name, and discovery must not drift from it.
const configFileName = ".brief.yaml"

// rootFS returns the production root FS: the whole namespace LocateWithinFS
// walks and InspectFS reads a file from, rooted at "/". This assumes a
// single-rooted, forward-slash
// namespace — true for brief's darwin/linux target (no Windows evidence
// anywhere in the tree: no CI workflow, devenv.nix names only a linux
// Buildkite agent) — and is not evaluated on a Windows volume path
// ("C:\..."), which fsName below cannot represent.
func rootFS() fs.FS {
	return os.DirFS("/")
}

// fsName maps abs, an absolute OS path, onto the name rootFS (or a test's
// own fstest.MapFS standing in for it) expects: the leading path separator
// stripped, forward-slash separated, "." for the root itself.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// rewritePathError swaps a *fs.PathError's own Path back to abs when it
// came from a rootFS call through fsName's relative mapping, so a stat or
// open failure's error text — embedded verbatim in a command's own refusal
// line — reads exactly as os.Stat(abs) or os.Open(abs) itself would have
// produced. Any other error shape passes through unchanged.
func rewritePathError(err error, abs string) error {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return &fs.PathError{Op: pe.Op, Path: abs, Err: pe.Err}
	}

	return err
}

// Locate walks upward from startDir to the filesystem root looking for a
// ".brief.yaml" file: nearest is the first one found (empty when none
// exists anywhere above startDir), and shadowed names every farther
// ancestor's own config file, nearest-first, that Resolve and Inspect
// never read because the nearest one already won. It refuses, as
// *InvalidConfigError (errors.Is(err, ErrInvalidConfig) holds too), a
// startDir that does not exist — filepath.Abs alone does not stat the
// path, so without this guard a mistyped path would silently walk from
// the nearest existing ancestor and report as if nothing were wrong. It
// is LocateWithin(startDir, "") — unbounded.
func Locate(startDir string) (string, []string, error) {
	return LocateWithin(startDir, "")
}

// LocateWithin is Locate's own walk, stopping at boundary rather than the
// filesystem root: boundary itself is still checked; its parent never is,
// so a config above boundary is never found. An empty boundary is
// unbounded, identical to Locate. It refuses, as *InvalidConfigError
// (errors.Is(err, ErrInvalidConfig) holds too — cli/refusal.go and
// doctor/checks.go both type-assert the concrete type to reach Path and
// Err), a startDir that does not exist, the same guard Locate's own doc
// describes — LocateWithinFS is where that check actually runs. boundary is
// expected to name an ancestor of startDir, or startDir itself: that is the only shape
// where the walk ever reaches a directory equal to it. A boundary outside
// startDir's own ancestor chain is silently inert rather than an error, and
// a boundary filepath.Abs cannot resolve is treated the same way: both fall
// back to unbounded, identical to Locate.
func LocateWithin(startDir, boundary string) (string, []string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve config: %w", err)
	}

	nearest, shadowed, err := LocateWithinFS(rootFS(), abs, resolveBoundary(boundary))
	if err != nil {
		return "", nil, fmt.Errorf("resolve config: %w", err)
	}

	return nearest, shadowed, nil
}

// LocateWithinFS is LocateWithin's own core: fsys is the whole filesystem
// namespace the walk runs against — production passes rootFS(), a test a
// fstest.MapFS holding just the ancestors in play — and startAbs,
// boundaryAbs are already absolute, slash-separated OS paths;
// filepath.Abs's own cwd-dependent resolution stays in LocateWithin, never
// here. The walk and the boundary comparison run entirely on startAbs's
// own string form (filepath.Join / filepath.Dir), identical regardless of
// fsys; only the existence checks go through it, by way of fsName.
func LocateWithinFS(fsys fs.FS, startAbs, boundaryAbs string) (string, []string, error) {
	if _, statErr := fs.Stat(fsys, fsName(startAbs)); statErr != nil {
		return "", nil, &InvalidConfigError{Path: startAbs, Err: rewritePathError(statErr, startAbs)}
	}

	var nearest string

	var shadowed []string

	for dir := startAbs; ; {
		candidate := filepath.Join(dir, configFileName)

		if _, statErr := fs.Stat(fsys, fsName(candidate)); statErr == nil {
			if nearest == "" {
				nearest = candidate
			} else {
				shadowed = append(shadowed, candidate)
			}
		}

		if boundaryAbs != "" && dir == boundaryAbs {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return nearest, shadowed, nil
}

// resolveBoundary returns boundary's own absolute path, or "" when boundary
// is empty or filepath.Abs cannot resolve it — both of which LocateWithin
// treats as unbounded rather than as an error.
func resolveBoundary(boundary string) string {
	if boundary == "" {
		return ""
	}

	abs, err := filepath.Abs(boundary)
	if err != nil {
		return ""
	}

	return abs
}

// LocateInRepo is Locate, bounded to the nearest git repository enclosing
// startDir (repo.Root): a ".brief.yaml" found above that repository's own
// root is never adopted — reported exactly as if none existed, empty
// nearest, no shadowed ancestors — since init, uninstall and doctor's own
// install root must never leave the repository startDir is inside. When no
// enclosing git repository exists anywhere above startDir, this is
// identical to Locate: there is no repository boundary to enforce, so
// today's unbounded ancestor walk stands.
func LocateInRepo(startDir string) (string, []string, error) {
	boundary := ""
	if root, ok := repo.Root(startDir); ok {
		boundary = root
	}

	return LocateWithin(startDir, boundary)
}

// Resolve walks upward from startDir to the filesystem root looking for a
// ".brief.yaml" file (Locate). The nearest one found wins outright — its
// values are decoded onto Default() so an omitted key keeps its shipped
// value, and no value from a farther, shadowed file is merged in. A
// repository with no config file anywhere is not an error: Default() is
// returned and the reported source is empty, meaning the shipped profile
// is in effect.
//
// A found config file's decoded values are checked against R1's rules
// (Inspect) — caps at least 1, headings non-empty and pairwise distinct,
// file names with no path separator, a step-file-pattern with exactly one
// integer verb, a handoff-file-suffix that collides with nothing — before
// Resolve returns it. The first violation, in Config's own
// field-declaration order, is reported as *InvalidConfigError wrapping
// that *ValueError; nothing from the file is used. A repository with no
// config file is exempt: Default() is never run back through this check.
func Resolve(startDir string) (Config, string, error) {
	nearest, _, err := Locate(startDir)
	if err != nil {
		return Config{}, "", err
	}

	if nearest == "" {
		return Default(), "", nil
	}

	cfg, violations, err := Inspect(nearest)
	if err != nil {
		return Config{}, "", err
	}

	if len(violations) > 0 {
		return Config{}, "", fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return cfg, nearest, nil
}

// InspectFS is Inspect's own core: fsys is the whole filesystem namespace
// the file at abs is read from — production passes rootFS(), a test a
// fstest.MapFS holding just that one file — and abs is the file's own
// already-absolute, slash-separated OS path; Inspect owns filepath.Abs's
// own cwd-dependent resolution, never seen here. It opens fsName(abs)
// within fsys and decodes it onto Default(), so a key the file omits keeps
// its shipped value, then reports every decoded value that fails an R1
// rule (violations), in Config's own field-declaration order, alongside
// the decoded Config — doctor's config-values check renders one row per
// element, where Resolve reports only the first. A decode failure
// (malformed YAML, an unknown key, or a directory sitting where the file
// is expected — opening it succeeds, decoding it does not) is reported as
// *InvalidConfigError, Path set to abs, neither "resolve config:" prefixed
// nor otherwise wrapped — that prefix is Inspect's own. A zero-byte file
// decodes as io.EOF, which InspectFS treats as an empty file rather than a
// failure — Default() stands, with no violations. A file fsys cannot open
// (missing, or permission denied) reports fsys's own open error, Path
// rewritten to abs, unwrapped — never *InvalidConfigError, matching
// Inspect's own long-standing contract that an unreadable file is a plain
// failure, not an invalid one.
func InspectFS(fsys fs.FS, abs string) (Config, []*ValueError, error) {
	f, err := fsys.Open(fsName(abs))
	if err != nil {
		return Config{}, nil, rewritePathError(err, abs)
	}
	defer f.Close()

	cfg := Default()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return cfg, nil, nil
		}

		return Config{}, nil, &InvalidConfigError{Path: abs, Err: err}
	}

	return cfg, violations(cfg), nil
}

// Inspect is InspectFS's own thin OS adapter: path is resolved to an
// absolute path the same way LocateWithin resolves startDir, then read
// through rootFS(). Every error InspectFS returns is wrapped, once, in
// Inspect's own "resolve config:" prefix — an *InvalidConfigError as
// *InvalidConfigError still (errors.Is(err, ErrInvalidConfig) and
// errors.As both still reach it), anything else as plain text ahead of it,
// path.Abs's own failure the same way.
func Inspect(path string) (Config, []*ValueError, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, nil, fmt.Errorf("resolve config: %s: %w", path, err)
	}

	cfg, viol, err := InspectFS(rootFS(), abs)
	if err != nil {
		if invalidCfg, ok := errors.AsType[*InvalidConfigError](err); ok {
			return Config{}, nil, fmt.Errorf("resolve config: %w", invalidCfg)
		}

		return Config{}, nil, fmt.Errorf("resolve config: %s: %w", abs, err)
	}

	return cfg, viol, nil
}
