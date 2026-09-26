# Mutation verification

For `developer` (runs mutations) and `test-reviewer` (checks mutation claims). Other reviewers: only *Reviewers never mutate worktree* applies.

Guard, test, or "absence" claim proven by breaking thing and seeing specific test go red — not by suite being green.

Mutation is second proof, not first. Red-first test already went red once, against missing behaviour; mutation shows it still guards finished code. Test that never went red before its production code does not become TDD by passing mutation check afterwards.

**Copy the file aside so a crash cannot leave the mutation behind:**

```bash
B="$TMPDIR/mutation-$(basename <file>).$$"   # unique per run; take FRESH copy before each mutation
cp <file> "$B" && test -f "$B" || exit 1     # a shared name may be a DIRECTORY
# apply the mutation, run the targeted test, observe RED
cp "$B" <file>                               # restore
diff "$B" <file>                             # prove byte-identical
```

Interrupted run can die holding gutted guard, and tree then looks merely "failing" not "deliberately broken". Copy make that recoverable.

**Unique backup name, checked to be file.** Fixed path like `$TMPDIR/mutation-backup` shared by every agent in session: if earlier one left *directory* there, `cp <file> "$TMPDIR/mutation-backup"` silently copies INTO it, restore then fails with mutation still live. Only mandated `diff` reveals it.

**Never use `git stash` for this.** Pipeline work runs in git worktrees, and every worktree shares one stash stack with main checkout and any other session: bare `git stash pop` can apply someone else entry. Never reuse old `$TMPDIR` copy either — stale copy silently reverts file to older contents.

Rules:

- **Mutate only guards plan names.** Architect picks which guards matter; developer add no mutation checks of own. Mutation per step = how scenario double its tool calls without proving anything named ones do not.
- **Verify guards INDIVIDUALLY.** Two guards that only go red when BOTH disabled mean either can be deleted silently. Disable one at a time.
- Mutation that breaks compilation **not** evidence. If every test fails, you proved file parses, nothing more. Make mutation surgical and still-valid.
- Say which mutation you ran and which test it reddened. "Mutation-verified" alone not claim anyone can check.
- Mutation results go in the report and STATE.md, never in a test comment (`go-testing` → *Test comments*).
- **Run affected package with `-run`, not whole suite.** Mutation targets one file; full-suite run per check = most repeated waste in long scenario.
- **Two reddened tests not two behaviours.** Pair sharing Given, When and Then is one case named twice; mutation report counting both overstates coverage. Check each cited test discriminates something others do not.
- **Reviewers never mutate worktree.** Reviewers run parallel; mutation in shared tree poisons every concurrent run. Mutate `git archive <sha>` export under `$TMPDIR`. Only developer (runs alone) mutates in place.
