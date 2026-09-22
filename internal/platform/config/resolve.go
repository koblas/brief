package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/repo"
	"gopkg.in/yaml.v3"
)

// configFileName is the one name Resolve looks for. init writes exactly
// this name, and discovery must not drift from it.
const configFileName = ".brief.yaml"

// Locate walks upward from startDir to the filesystem root looking for a
// ".brief.yaml" file: nearest is the first one found (empty when none
// exists anywhere above startDir), and shadowed names every farther
// ancestor's own config file, nearest-first, that Resolve and Inspect
// never read because the nearest one already won. It refuses, as
// ErrInvalidConfig, a startDir that does not exist — filepath.Abs alone
// does not stat the path, so without this guard a mistyped path would
// silently walk from the nearest existing ancestor and report as if
// nothing were wrong. It is LocateWithin(startDir, "") — unbounded.
func Locate(startDir string) (string, []string, error) {
	return LocateWithin(startDir, "")
}

// LocateWithin is Locate's own walk, stopping at boundary rather than the
// filesystem root: boundary itself is still checked; its parent never is,
// so a config above boundary is never found. An empty boundary is
// unbounded, identical to Locate. It refuses, as ErrInvalidConfig, a
// startDir that does not exist, the same guard Locate's own doc describes —
// LocateWithin is where that check actually runs. boundary is expected to
// name an ancestor of startDir, or startDir itself: that is the only shape
// where the walk ever reaches a directory equal to it. A boundary outside
// startDir's own ancestor chain is silently inert rather than an error, and
// a boundary filepath.Abs cannot resolve is treated the same way: both fall
// back to unbounded, identical to Locate.
func LocateWithin(startDir, boundary string) (string, []string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve config: %w", err)
	}

	if _, statErr := os.Stat(abs); statErr != nil {
		return "", nil, fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: abs, Err: statErr})
	}

	boundaryAbs := resolveBoundary(boundary)

	var nearest string

	var shadowed []string

	for dir := abs; ; {
		path := filepath.Join(dir, configFileName)

		if _, statErr := os.Stat(path); statErr == nil {
			if nearest == "" {
				nearest = path
			} else {
				shadowed = append(shadowed, path)
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

// Inspect reads path and decodes it onto Default(), so a key the file
// omits keeps its shipped value, then reports every decoded value that
// fails an R1 rule (violations), in Config's own field-declaration order,
// alongside the decoded Config — doctor's config-values check renders one
// row per element, where Resolve reports only the first. A decode failure
// (malformed YAML, an unknown key) is reported as *InvalidConfigError,
// "resolve config:" prefixed, the zero Config and nil violations; a
// zero-byte file decodes as io.EOF, which Inspect treats as an empty file
// rather than a failure — Default() stands, with no violations.
func Inspect(path string) (Config, []*ValueError, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, nil, fmt.Errorf("resolve config: %s: %w", path, err)
	}
	defer f.Close()

	cfg := Default()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return cfg, nil, nil
		}

		return Config{}, nil, fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: path, Err: err})
	}

	return cfg, violations(cfg), nil
}
