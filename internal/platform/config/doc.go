// Package config resolves brief's configuration: the shipped default profile,
// and the repository's own ".brief.yaml" when one is present.
//
// Every feature package (new, start, finish, status, check) consumes a
// Config; this package holds no feature knowledge of its own, only the
// schema and the resolution rule. Locate walks upward from a start
// directory to the filesystem root and finds the nearest ".brief.yaml",
// naming every farther ancestor's own config file as shadowed — a
// repository with no config file at all is not an error, it is the shipped
// profile in effect. Inspect decodes a found file onto Default(), so an
// omitted key keeps its shipped value, an unknown key is refused rather
// than silently ignored, and reports every decoded value that fails its own
// rule (a cap is at least 1, a heading is non-empty and distinct from the
// others, a file name carries no path separator, and so on) alongside the
// decoded Config. Resolve is Locate plus Inspect's first violation: the one
// rule set and the one walk-up every caller shares, so a command refusing
// an invalid value and a tool reporting every value that fails (doctor) can
// never drift onto two different checks.
package config
