// Package config resolves brief's configuration: the shipped default profile,
// and the repository's own ".brief.yaml" when one is present.
//
// Every feature package (new, start, finish, status, check) consumes a
// Config; this package holds no feature knowledge of its own, only the
// schema and the resolution rule. Resolution walks upward from a start
// directory to the filesystem root, and the nearest ".brief.yaml" wins with
// no merging across levels — a repository with no config file at all is not
// an error, it is the shipped profile in effect. A found config file is
// decoded onto Default(), so an omitted key keeps its shipped value, and an
// unknown key is refused rather than silently ignored.
package config
