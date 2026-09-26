# Evidence that proves nothing

For anyone making or judging a claim: `architect` (plans control arms), `developer`, every reviewer.

## Assertions

Assertions that look like proof and are not recur in few shapes:

- Asserting against constant fixture set, or value copied out of production code being tested. Pin derived by reading code pins nothing.
- Negative assertions satisfied by nothing happening at all — dominant shape. Absence claim needs **control arm** showing thing DOES happen when guard removed, and control must differ from claim in exactly one variable.
- Observables that cannot fire on path under test.
- Asserting store empty without first proving it non-empty and same probe would have seen it.
- Comments overclaiming what test below them covers.
- Test that was never red. Written after its production code and green on first run, it has not shown it can fail — `tdd` skill.

When refactor removes call site, **every existing "was never called" assertion on that fake become unfalsifiable.** Repoint them at new reachable observable, or they pass with guard deleted.

## Greps

Grep is evidence only if it can see what it looks for.

- **Comment sweeps must be multiline-aware.** `//` blocks wrap, so phrase splits across lines and line-based `grep` silently reports zero. Flatten continuations first (strip leading `//`) before matching. "0 hits" from line-based grep over prose = untested claim, not clean sweep.
- **Run positive control before believing zero.** Grep for symbol you KNOW is present with same flags and scope. Control not found → sweep cannot see target, its zero means nothing.
- State what grep would MISS, not just what it found.
