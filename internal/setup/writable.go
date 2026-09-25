package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/koblas/brief/internal/platform/writable"
)

// writableTargets lists every path a real Init would write to, in apply's
// write order, for checkWritable to check.
func writableTargets(featureArt Artifact, writeArts []pluginArtifact, boundAgentArts []boundAgentArtifact, snippetArt snippetArtifact, hasSnippet bool, configArt Artifact) []string {
	targets := make([]string, 0, 2+len(writeArts)+len(boundAgentArts))

	if featureArt.Action == ActionCreated {
		targets = append(targets, featureArt.Path)
	}

	for _, w := range writeArts {
		if w.Action == ActionCreated || w.Action == ActionMerged {
			targets = append(targets, w.Path)
		}
	}

	for _, ba := range boundAgentArts {
		if ba.Action == ActionMerged {
			targets = append(targets, ba.Path)
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

// checkWritable reports a refusal for the first target whose nearest
// existing ancestor cannot be written to, nil if every one can. Each
// target's own leaf path is never inspected, only its ancestor.
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

		if !writable.Probe(anc) {
			return &RefusalError{Path: anc, Problem: "not writable", Fix: "apply the output below by hand", Err: ErrUnwritable}
		}
	}

	return nil
}

// nearestExistingAncestor walks up from target's parent directory until it
// finds a path that exists, returning that path and its os.Lstat info. It
// always terminates: the filesystem root always exists.
func nearestExistingAncestor(target string) (string, os.FileInfo, error) {
	dir := filepath.Dir(target)

	for {
		info, err := os.Lstat(dir)
		if err == nil {
			return dir, info, nil
		}

		// ENOTDIR means a non-directory ancestor further down this chain
		// blocks every deeper Lstat; keep walking up past it too.
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
