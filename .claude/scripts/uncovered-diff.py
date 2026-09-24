#!/usr/bin/env python3
"""List production Go lines added since BASE that no test executes.

Usage: .claude/scripts/uncovered-diff.py [BASE] [PKG ...]
  BASE defaults to origin/main; PKG defaults to ./... (the packages whose tests run).

Runs `go test -coverprofile -coverpkg=./...` once, then intersects the profile with the
lines `git diff -U0 BASE` added to non-test .go files (committed and uncommitted). A line
counts as uncovered when every coverage block containing it has count 0. Lines that are
not statements (comments, declarations) have no block and are skipped, as are lone
closing braces, which share their block with the statement above them.

Output: one "path:line: <source>" row per uncovered added line on stdout, a summary on
stderr. Exit status: 0 when nothing added is uncovered, 1 when something is, 2 on a tool
failure. An uncovered line is not automatically a defect -- an unreachable defensive
branch may be legitimate -- but every one must be either tested or named in the report
with its reason.

Standard library only; runs on the macOS system python3 (3.9+).
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
import tempfile
from collections import defaultdict

HUNK = re.compile(r"^@@ -\S+ \+(\d+)(?:,(\d+))? @@")
# module/path/file.go:startLine.startCol,endLine.endCol numStmts count
BLOCK = re.compile(r"^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$")


def run(args: list[str], **kw) -> subprocess.CompletedProcess:
    return subprocess.run(args, check=True, text=True, capture_output=True, **kw)


def added_lines(base: str) -> dict[str, set[int]]:
    """Map each non-test .go file to the line numbers `git diff -U0 base` added to it."""
    diff = run(["git", "diff", "-U0", base, "--", "*.go", ":(exclude)*_test.go"]).stdout
    added: dict[str, set[int]] = defaultdict(set)
    path = None
    for line in diff.splitlines():
        if line.startswith("+++ "):
            target = line[4:]
            path = target[2:] if target.startswith("b/") else None  # "/dev/null" on delete
            continue
        m = HUNK.match(line)
        if m and path:
            start = int(m.group(1))
            count = 1 if m.group(2) is None else int(m.group(2))
            added[path].update(range(start, start + count))
    return added


def coverage(profile: str, module: str) -> tuple[set[tuple[str, int]], set[tuple[str, int]]]:
    """Return (lines inside any block, lines inside a block with count > 0)."""
    seen: set[tuple[str, int]] = set()
    covered: set[tuple[str, int]] = set()
    prefix = module + "/"
    with open(profile) as f:
        for raw in f:
            m = BLOCK.match(raw.strip())
            if not m:
                continue  # the "mode:" header
            path = m.group(1)
            if path.startswith(prefix):
                path = path[len(prefix):]
            start, end, count = int(m.group(2)), int(m.group(3)), int(m.group(4))
            for n in range(start, end + 1):
                seen.add((path, n))
                if count > 0:
                    covered.add((path, n))
    return seen, covered


def main(argv: list[str]) -> int:
    base = argv[0] if argv else "origin/main"
    pkgs = argv[1:] or ["./..."]

    try:
        root = run(["git", "rev-parse", "--show-toplevel"]).stdout.strip()
        os.chdir(root)
        module = run(["go", "list", "-m"]).stdout.strip()
        added = added_lines(base)
    except subprocess.CalledProcessError as e:
        print(f"uncovered-diff: {' '.join(e.cmd)} failed: {e.stderr.strip()}", file=sys.stderr)
        return 2

    if not added:
        print(f"uncovered-diff: no production Go lines added since {base}", file=sys.stderr)
        return 0

    fd, profile = tempfile.mkstemp(prefix="uncovered.", dir=os.environ.get("TMPDIR"))
    os.close(fd)
    try:
        test = subprocess.run(
            ["go", "test", "-count=1", "-covermode=set", "-coverpkg=./...",
             f"-coverprofile={profile}", *pkgs],
            text=True, capture_output=True,
        )
        if test.returncode != 0:
            print("uncovered-diff: go test failed; run it directly to see why", file=sys.stderr)
            return 2
        seen, covered = coverage(profile, module)
    finally:
        os.remove(profile)

    rows = []
    for path in sorted(added):
        try:
            with open(path) as f:
                source = f.read().splitlines()
        except OSError:
            continue  # added then deleted in the working tree
        for n in sorted(added[path]):
            key = (path, n)
            if key not in seen or key in covered or n > len(source):
                continue
            text = source[n - 1].strip()
            if text == "}":
                continue
            rows.append(f"{path}:{n}: {text}")

    for row in rows:
        print(row)
    print(f"uncovered-diff: {len(rows)} uncovered added line(s) since {base}", file=sys.stderr)
    return 1 if rows else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
