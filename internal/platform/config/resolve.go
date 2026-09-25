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

// configFileName is the one name Resolve looks for.
const configFileName = ".brief.yaml"

// rootFS returns the production root FS that LocateWithinFS walks and
// InspectFS reads from, rooted at "/". Assumes a single-rooted,
// forward-slash namespace (brief's darwin/linux target); not evaluated on a
// Windows volume path.
func rootFS() fs.FS {
	return os.DirFS("/")
}

// fsName maps abs, an absolute OS path, onto the name rootFS (or a test
// fstest.MapFS standing in for it) expects: the leading path separator
// stripped, forward-slash separated, "." for the root itself.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// rewritePathError swaps a *fs.PathError's Path back to abs when it came
// from a rootFS call through fsName's relative mapping, so the error text
// reads exactly as os.Stat(abs) or os.Open(abs) would have produced.
func rewritePathError(err error, abs string) error {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return &fs.PathError{Op: pe.Op, Path: abs, Err: pe.Err}
	}

	return err
}

// Locate walks upward from startDir to the filesystem root looking for a
// ".brief.yaml" file: nearest is the first one found (empty when none
// exists), and shadowed names every farther ancestor's config file,
// nearest-first. It refuses, as *InvalidConfigError, a startDir that does
// not exist. It is LocateWithin(startDir, "") — unbounded.
func Locate(startDir string) (string, []string, error) {
	return LocateWithin(startDir, "")
}

// LocateWithin is Locate's walk, stopping at boundary rather than the
// filesystem root: boundary itself is still checked, its parent never is,
// so a config above boundary is never found. An empty boundary, or one
// outside startDir's ancestor chain, is unbounded, identical to Locate. It
// refuses, as *InvalidConfigError, a startDir that does not exist.
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

// LocateWithinFS is LocateWithin's core: fsys is the filesystem namespace
// the walk runs against (production: rootFS(), a test: a fstest.MapFS
// holding just the ancestors in play), and startAbs, boundaryAbs are
// already absolute, slash-separated OS paths — filepath.Abs's cwd-dependent
// resolution stays in LocateWithin, never here.
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

// resolveBoundary returns boundary's absolute path, or "" when boundary is
// empty or filepath.Abs cannot resolve it — both unbounded to LocateWithin.
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
// startDir (repo.Root): a ".brief.yaml" found above that repository's root
// is never adopted — reported exactly as if none existed. With no
// enclosing git repository, this is identical to Locate.
func LocateInRepo(startDir string) (string, []string, error) {
	boundary := ""
	if root, ok := repo.Root(startDir); ok {
		boundary = root
	}

	return LocateWithin(startDir, boundary)
}

// Resolve walks upward from startDir for the nearest ".brief.yaml"
// (Locate). No config file anywhere is not an error: Default() is returned
// with an empty source. A found file's decoded values are checked (Inspect)
// — caps at least 1, headings non-empty and pairwise distinct, file names
// with no path separator, a step-file-pattern with exactly one integer
// verb, a handoff-file-suffix that collides with nothing. The first
// violation, in field-declaration order, is reported as *InvalidConfigError
// wrapping that *ValueError; nothing from the file is used.
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

// InspectFS decodes the file at abs, within fsys, onto Default() — an
// omitted key keeps its shipped value — and reports every decoded value
// that fails its own rule, in field-declaration order. A decode failure is
// *InvalidConfigError; a zero-byte file is Default() with no violations; a
// file fsys cannot open reports the open error, Path rewritten to abs,
// never *InvalidConfigError.
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

// Inspect is InspectFS's thin OS adapter: path is resolved to an absolute
// path, then read through rootFS(). Every error InspectFS returns is
// wrapped, once, in a "resolve config:" prefix — an *InvalidConfigError
// stays reachable via errors.Is/errors.As, anything else becomes plain text
// ahead of it.
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
