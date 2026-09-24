#!/usr/bin/env bash
# uncovered-diff.sh — list production Go lines added since BASE that no test executes.
#
# Usage: .claude/scripts/uncovered-diff.sh [BASE] [PKG...]
#   BASE defaults to origin/main; PKG defaults to ./... (the packages whose tests run).
#
# Runs `go test -coverprofile -coverpkg=./...` once, then intersects the profile with the
# lines `git diff -U0 BASE` added to non-test .go files (committed and uncommitted).
# A line counts as uncovered when every coverage block containing it has count 0. Lines
# that are not statements (comments, braces, declarations) have no block and are skipped.
#
# Output: one "path:line: <source>" row per uncovered added line, then a summary on stderr.
# Exit status: 0 when nothing added is uncovered, 1 when something is, 2 on a tool failure.
# An uncovered line is not automatically a defect — an unreachable defensive branch may be
# legitimate — but every one must be either tested or named in the report with its reason.
set -euo pipefail

base="${1:-origin/main}"
shift || true
pkgs=("$@")
[ ${#pkgs[@]} -eq 0 ] && pkgs=(./...)

root="$(git rev-parse --show-toplevel)"
cd "$root"
module="$(go list -m)"

prof="$(mktemp "${TMPDIR:-/tmp}/uncovered.XXXXXX")"
added="$(mktemp "${TMPDIR:-/tmp}/uncovered-added.XXXXXX")"
trap 'rm -f "$prof" "$added"' EXIT

if ! go test -count=1 -covermode=set -coverpkg=./... -coverprofile="$prof" "${pkgs[@]}" >/dev/null 2>&1; then
	echo "uncovered-diff: go test failed; run it directly to see why" >&2
	exit 2
fi

# Added lines in non-test Go files, as "path<TAB>line".
git diff -U0 "$base" -- '*.go' ':(exclude)*_test.go' | awk '
	/^\+\+\+ / { file = substr($2, 3); next }
	/^@@ / {
		split($3, a, ",")
		start = substr(a[1], 2) + 0
		count = (a[2] == "" ? 1 : a[2] + 0)
		for (i = 0; i < count; i++) print file "\t" start + i
	}
' >"$added"

if [ ! -s "$added" ]; then
	echo "uncovered-diff: no production Go lines added since $base" >&2
	exit 0
fi

rows="$(awk -v module="$module/" '
	FNR == NR { want[$1 "\t" $2] = 1; next }
	FNR == 1 && /^mode:/ { next }
	{
		# module/path/file.go:startLine.startCol,endLine.endCol numStmts count
		split($1, loc, ":")
		file = loc[1]; sub("^" module, "", file)
		split(loc[2], span, ",")
		split(span[1], s, "."); split(span[2], e, ".")
		for (l = s[1] + 0; l <= e[1] + 0; l++) {
			key = file "\t" l
			if (!(key in want)) continue
			seen[key] = 1
			if ($3 + 0 > 0) covered[key] = 1
		}
	}
	END { for (key in seen) if (!(key in covered)) print key }
' "$added" "$prof" | sort -t"$(printf '\t')" -k1,1 -k2,2n)"

if [ -z "$rows" ]; then
	echo "uncovered-diff: 0 uncovered added lines since $base" >&2
	exit 0
fi

n=0
while IFS="$(printf '\t')" read -r file line; do
	src="$(sed -n "${line}p" "$file" | sed 's/^[[:space:]]*//')"
	# A lone closing brace shares its block with the statement above it; skip the noise.
	[ "$src" = "}" ] && continue
	printf '%s:%s: %s\n' "$file" "$line" "$src"
	n=$((n + 1))
done <<<"$rows"

if [ "$n" -eq 0 ]; then
	echo "uncovered-diff: 0 uncovered added lines since $base" >&2
	exit 0
fi

echo "uncovered-diff: $n uncovered added line(s) since $base" >&2
exit 1
