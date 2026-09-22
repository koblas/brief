package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// configFileName is the one name Resolve looks for. init writes exactly
// this name, and discovery must not drift from it.
const configFileName = ".brief.yaml"

// Resolve walks upward from startDir to the filesystem root looking for a
// ".brief.yaml" file. The nearest one found wins outright — its values are
// decoded onto Default() so an omitted key keeps its shipped value, and no
// value from a farther file is merged in. A repository with no config file
// anywhere is not an error: Default() is returned and the reported source
// is empty, meaning the shipped profile is in effect.
//
// Resolve refuses, as ErrInvalidConfig, a startDir that does not exist —
// filepath.Abs alone does not stat the path, so without this guard a
// mistyped path would silently walk from the nearest existing ancestor and
// return the shipped profile as if nothing were wrong.
//
// A found config file's decoded values are checked against R1's rules —
// caps at least 1, headings non-empty and pairwise distinct, file names
// with no path separator, a step-file-pattern with exactly one integer
// verb, a handoff-file-suffix that collides with nothing — before Resolve
// returns it. The first violation, in Config's own field-declaration
// order, is reported as *InvalidConfigError wrapping a *ValueError;
// nothing from the file is used. A repository with no config file is
// exempt: Default() is never run back through this check.
func Resolve(startDir string) (Config, string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve config: %w", err)
	}

	if _, statErr := os.Stat(abs); statErr != nil {
		return Config{}, "", fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: abs, Err: statErr})
	}

	for dir := abs; ; {
		path := filepath.Join(dir, configFileName)

		if _, statErr := os.Stat(path); statErr == nil {
			cfg, decodeErr := decodeConfig(path)
			if decodeErr != nil {
				return Config{}, "", decodeErr
			}

			return cfg, path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return Default(), "", nil
}

// decodeConfig reads path and decodes it onto Default(), so a key the file
// omits keeps its shipped value, then validates every decoded value
// against R1's rules before returning it.
func decodeConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("resolve config: %s: %w", path, err)
	}
	defer f.Close()

	cfg := Default()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		// yaml.Decoder.Decode reports a zero-byte file as io.EOF, where
		// yaml.Unmarshal would have reported nil. An empty file sets no
		// key, so it is treated the same as a file that is merely silent
		// on every key: the shipped defaults already loaded into cfg above
		// stand.
		if errors.Is(err, io.EOF) {
			return cfg, nil
		}

		return Config{}, fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: path, Err: err})
	}

	if err := validate(cfg); err != nil {
		return Config{}, fmt.Errorf("resolve config: %w", &InvalidConfigError{Path: path, Err: err})
	}

	return cfg, nil
}
