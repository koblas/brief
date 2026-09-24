#!/usr/bin/env bash
# test-stats.sh — the one agreed way to count tests in this repo.
#
# Usage: .claude/scripts/test-stats.sh [--run] [PKGDIR...]
#   PKGDIR defaults to every package under internal/ that has tests.
#   --run also executes `go test -count=1 -v` per package and counts PASS/FAIL/SKIP leaves.
#
# Columns (static, from the working tree — untracked test files included):
#   tests    top-level `func Test` functions
#   tempdir  `t.TempDir()` call sites
#   disk     top-level tests whose own body touches real disk: t.TempDir(, t.Chdir(,
#            or an os.WriteFile/MkdirAll/Mkdir/Symlink/Chmod/Remove call. A test that only
#            calls a disk-backed helper is not counted — read "disk" as a floor, and quote
#            it the same way before and after a change so the delta means something.
# Columns with --run: pass, fail, skip (every leaf, subtests included).
#
# Quote these numbers, from this script, in every report. Hand-rolled counts drifted by
# up to nine tests between agents on the same commit.
set -euo pipefail

run=0
if [ "${1:-}" = "--run" ]; then
	run=1
	shift
fi

cd "$(git rev-parse --show-toplevel)"

pkgs=("$@")
if [ ${#pkgs[@]} -eq 0 ]; then
	while IFS= read -r d; do pkgs+=("$d"); done < <(find internal -name '*_test.go' -exec dirname {} \; | sort -u)
fi

if [ "$run" -eq 1 ]; then
	printf '%-34s %6s %8s %5s %6s %5s %5s\n' package tests tempdir disk pass fail skip
else
	printf '%-34s %6s %8s %5s\n' package tests tempdir disk
fi

tot_tests=0 tot_temp=0 tot_disk=0 tot_pass=0 tot_fail=0 tot_skip=0
for p in "${pkgs[@]}"; do
	files=("$p"/*_test.go)
	[ -e "${files[0]}" ] || continue
	tests=$(cat "${files[@]}" | grep -c '^func Test' || true)
	temp=$(cat "${files[@]}" | grep -c 't\.TempDir()' || true)
	disk=$(awk '
		/^func / { if (name != "" && hit) n++; name = ($0 ~ /^func Test/) ? $2 : ""; hit = 0; next }
		name != "" && /t\.TempDir\(|t\.Chdir\(|os\.(WriteFile|MkdirAll|Mkdir|Symlink|Chmod|Remove)\(/ { hit = 1 }
		END { if (name != "" && hit) n++; print n + 0 }
	' "${files[@]}")
	tot_tests=$((tot_tests + tests)) tot_temp=$((tot_temp + temp)) tot_disk=$((tot_disk + disk))
	if [ "$run" -eq 1 ]; then
		out=$(go test -count=1 -v "./$p" 2>&1 || true)
		pass=$(grep -c -- '--- PASS' <<<"$out" || true)
		fail=$(grep -c -- '--- FAIL' <<<"$out" || true)
		skip=$(grep -c -- '--- SKIP' <<<"$out" || true)
		tot_pass=$((tot_pass + pass)) tot_fail=$((tot_fail + fail)) tot_skip=$((tot_skip + skip))
		printf '%-34s %6d %8d %5d %6d %5d %5d\n' "$p" "$tests" "$temp" "$disk" "$pass" "$fail" "$skip"
	else
		printf '%-34s %6d %8d %5d\n' "$p" "$tests" "$temp" "$disk"
	fi
done

if [ "$run" -eq 1 ]; then
	printf '%-34s %6d %8d %5d %6d %5d %5d\n' TOTAL "$tot_tests" "$tot_temp" "$tot_disk" "$tot_pass" "$tot_fail" "$tot_skip"
	[ "$tot_fail" -eq 0 ]
else
	printf '%-34s %6d %8d %5d\n' TOTAL "$tot_tests" "$tot_temp" "$tot_disk"
fi
