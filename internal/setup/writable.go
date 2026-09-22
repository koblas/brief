package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// writableTargets lists every path a real (non-DryRun, non-Print) Init
// would write to, in apply's own write order: the feature root when
// ActionCreated, then every writeArts entry reporting ActionCreated, then
// the snippet's own path when hasSnippet and its Action is ActionCreated or
// ActionMerged, then the config file's own path when ActionCreated —
// checkWritable's own input.
func writableTargets(featureArt Artifact, writeArts []pluginArtifact, snippetArt snippetArtifact, hasSnippet bool, configArt Artifact) []string {
	targets := make([]string, 0, 2+len(writeArts))

	if featureArt.Action == ActionCreated {
		targets = append(targets, featureArt.Path)
	}

	for _, w := range writeArts {
		if w.Action == ActionCreated {
			targets = append(targets, w.Path)
		}
	}

	if hasSnippet && (snippetArt.Action == ActionCreated || snippetArt.Action == ActionMerged) {
		targets = append(targets, snippetArt.Path)
	}

	if configArt.Action == ActionCreated {
		targets = append(targets, configArt.Path)
	}

	return targets
}

// checkWritable reports R10's refusal for the first target whose nearest
// existing ancestor cannot be written to, nil when every target's own
// ancestor exists as a writable directory. Each target's own leaf path is
// never inspected — R10 never refuses on the leaf itself, only on an
// ancestor: a target that already exists (a merge) is exactly the "create
// vs merge" distinction this never has to make. Ancestors already checked
// (two targets sharing one parent directory) are checked only once.
func checkWritable(targets []string) error {
	checked := make(map[string]bool, len(targets))

	for _, target := range targets {
		anc, info, err := nearestExistingAncestor(target)
		if err != nil {
			return err
		}

		if checked[anc] {
			continue
		}

		checked[anc] = true

		if !info.IsDir() {
			return &RefusalError{Path: anc, Problem: "not a directory", Fix: "apply the output below by hand", Err: ErrUnwritable}
		}

		if !probeWritable(anc) {
			return &RefusalError{Path: anc, Problem: "not writable", Fix: "apply the output below by hand", Err: ErrUnwritable}
		}
	}

	return nil
}

// nearestExistingAncestor walks up from target's own parent directory —
// never target itself — until it finds a path that exists, returning that
// path and its os.Lstat info. Both ENOENT (os.IsNotExist) and ENOTDIR mean
// "keep walking up": a non-directory ancestor further down the same chain
// (say, ".claude/skills" a regular file) makes every deeper Lstat report
// ENOTDIR rather than ENOENT, which os.IsNotExist never matches — the walk
// has to keep going until it reaches that file itself, the actual blocking
// ancestor. It always terminates: the filesystem root always exists.
func nearestExistingAncestor(target string) (string, os.FileInfo, error) {
	dir := filepath.Dir(target)

	for {
		info, err := os.Lstat(dir)
		if err == nil {
			return dir, info, nil
		}

		if !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTDIR) {
			return "", nil, fmt.Errorf("setup: lstat %s: %w", dir, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, fmt.Errorf("setup: lstat %s: %w", dir, err)
		}

		dir = parent
	}
}

// probeWritable reports whether dir can be written to: it creates a
// temporary file inside dir, closes it, and removes it immediately — a
// deliberate copy of internal/doctor's own probe. setup may not import
// doctor (neither sits above the other in the dependency rule), so this is
// duplicated rather than shared sideways.
func probeWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".brief-init-probe-*")
	if err != nil {
		return false
	}

	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)

	return true
}
