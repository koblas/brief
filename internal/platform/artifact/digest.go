package artifact

import (
	"crypto/sha256"
	"slices"
)

// Kind names one of the file shapes this package renders — currently only
// the ".brief.yaml" config; a host's plugin manifest, hook, snippet and
// agent files each add their own Kind and digest list as init grows to
// write them.
type Kind string

// KindConfig is ConfigFile's own Kind: the digest list Recognize checks
// when kind is KindConfig holds every release's own ConfigFile digest.
const KindConfig Kind = "config"

// Origin classifies an existing file's bytes against a Kind's compiled-in
// digest list: OriginCurrent for a byte-for-byte match against today's own
// render, OriginEdited for anything else. A future release's digest list
// grows an OriginOlder arm once a second digest exists to distinguish "an
// earlier release wrote this, unedited" from "a person edited this" —
// today every digest list holds only the current render, so that
// distinction has nothing to select between yet.
type Origin string

const (
	// OriginCurrent marks bytes equal to this binary's own current render.
	OriginCurrent Origin = "current"
	// OriginEdited marks bytes that match no compiled-in digest for the
	// given Kind.
	OriginEdited Origin = "edited"
)

// configDigests holds the sha256 digest of every release's own ConfigFile
// render. It holds exactly one entry today; a future ConfigFile change
// appends its new digest here rather than replacing the old one, so an
// earlier release's file is still recognized rather than misclassified as
// edited.
var configDigests = [][32]byte{
	sha256.Sum256(ConfigFile()),
}

// Recognize reports body's Origin against kind's own compiled-in digest
// list: OriginCurrent when body's sha256 digest matches this binary's own
// current render for kind, OriginEdited otherwise. kind is KindConfig
// today; an unrecognized Kind (none exist yet) reports OriginEdited, since
// there is no digest list to match against.
func Recognize(kind Kind, body []byte) Origin {
	sum := sha256.Sum256(body)

	if slices.Contains(digestsFor(kind), sum) {
		return OriginCurrent
	}

	return OriginEdited
}

// digestsFor returns kind's own compiled-in digest list, nil for a Kind
// this package does not render.
func digestsFor(kind Kind) [][32]byte {
	switch kind {
	case KindConfig:
		return configDigests
	default:
		return nil
	}
}
