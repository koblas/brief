#!/usr/bin/env python3
"""The one agreed way to count tests in this repo.

Usage: .claude/scripts/test-stats.py [--run] [PKGDIR ...]
  PKGDIR defaults to every package under internal/ that has tests.
  --run also executes `go test -count=1 -v` per package and counts PASS/FAIL/SKIP leaves.

Columns (static, from the working tree -- untracked test files included):
  tests    top-level `func Test` functions
  tempdir  `t.TempDir()` call sites
  disk     top-level tests whose own body touches real disk: t.TempDir(, t.Chdir(, or an
           os.WriteFile/MkdirAll/Mkdir/Symlink/Chmod/Remove call. A test that only calls a
           disk-backed helper is not counted -- read "disk" as a floor, and quote it the
           same way before and after a change so the delta means something.
Columns with --run: pass, fail, skip (every leaf, subtests included).

Quote these numbers, from this script, in every report. Hand-rolled counts drifted by up
to nine tests between agents on the same commit.

Exit status: 0, or 1 when --run saw a failing test.
Standard library only; runs on the macOS system python3 (3.9+).
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

DISK = re.compile(r"t\.TempDir\(|t\.Chdir\(|os\.(WriteFile|MkdirAll|Mkdir|Symlink|Chmod|Remove)\(")

STATIC_ROW = "{:<34} {:>6} {:>8} {:>5}"
RUN_ROW = "{:<34} {:>6} {:>8} {:>5} {:>6} {:>5} {:>5}"


def test_packages() -> list[str]:
    """Every directory under internal/ holding a _test.go file, sorted."""
    return sorted({str(p.parent) for p in Path("internal").rglob("*_test.go")})


def static_counts(pkg: str) -> tuple[int, int, int] | None:
    files = sorted(Path(pkg).glob("*_test.go"))
    if not files:
        return None
    # The package's test files are read as one stream, in name order, so a function's
    # body is everything from its `func` line to the next `func` line anywhere after it.
    lines = [line for f in files for line in f.read_text().splitlines()]

    tests = sum(1 for line in lines if line.startswith("func Test"))
    tempdir = sum(1 for line in lines if "t.TempDir()" in line)

    disk = 0
    in_test = hit = False
    for line in lines:
        if line.startswith("func "):
            disk += in_test and hit
            in_test, hit = line.startswith("func Test"), False
        elif in_test and DISK.search(line):
            hit = True
    disk += in_test and hit

    return tests, tempdir, disk


def run_counts(pkg: str) -> tuple[int, int, int]:
    out = subprocess.run(
        ["go", "test", "-count=1", "-v", f"./{pkg}"],
        text=True, capture_output=True,
    )
    lines = (out.stdout + out.stderr).splitlines()
    return tuple(sum(1 for line in lines if f"--- {kind}" in line) for kind in ("PASS", "FAIL", "SKIP"))


def main(argv: list[str]) -> int:
    run = bool(argv) and argv[0] == "--run"
    if run:
        argv = argv[1:]

    root = subprocess.run(["git", "rev-parse", "--show-toplevel"], text=True, capture_output=True, check=True)
    os.chdir(root.stdout.strip())

    pkgs = argv or test_packages()

    if run:
        print(RUN_ROW.format("package", "tests", "tempdir", "disk", "pass", "fail", "skip"))
    else:
        print(STATIC_ROW.format("package", "tests", "tempdir", "disk"))

    totals = [0] * 6
    for pkg in pkgs:
        counts = static_counts(pkg)
        if counts is None:
            continue
        row = list(counts) + (list(run_counts(pkg)) if run else [])
        totals = [t + n for t, n in zip(totals, row)]
        print((RUN_ROW if run else STATIC_ROW).format(pkg, *row))

    if run:
        print(RUN_ROW.format("TOTAL", *totals))
        return 1 if totals[4] else 0

    print(STATIC_ROW.format("TOTAL", *totals[:3]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
