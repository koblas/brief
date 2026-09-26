# Fix passes

For orchestrator (writing fix-pass brief) and `developer` in fix mode.

## Red first, per finding

CLAUDE.md TDD rule covers fix passes: every finding whose fix changes production behaviour starts with failing test. Write test reproducing finding's `Failure:`, run it, see it fail **at its assertion, for that reason**, then fix. Finding already covered by test that fails today → cite it, no new one needed.

Exempt: findings changing no behaviour — doc comments, renames, test-only additions, extractions. Missing-test finding ("untested change") *is* test: write it, and if it passes on arrival against correct code, say so (green on arrival, not manufactured red).

Can't make it red (Failure not reproducible in test) → say so in report with what you tried; do not fix blind and backfill test.

## No new behaviour

Fix pass applies findings. Cheap MINOR/NIT folds = docs, renames, test additions, extractions keeping behaviour identical. **Fold adding runtime behaviour not cheap:** new branch folded in from MINOR and left untested becomes MAJOR forcing another fix pass and re-gate. New behaviour goes to STATE.md `## Open debts`, or folded with its tests planned in same brief, named per branch.

**Applies to MAJOR's fix too, not only folds.** When fix for finding adds runtime behaviour (timeout, retry, fallback, new branch), fix-pass brief names, before dispatch:

- **positive assertion** for each new branch — what DID happen, not only what did not — with control arm differing in one variable;
- for every new numeric bound, **in-bound and just-outside-bound tests** (`planning.md` → *Coverage* applies to fix passes unchanged);
- **mutation expected to redden each test**.

Those tests go red before fix code lands, same as above.

Test reading `ctx.Err()` from value defaulting to nil, or bound test that cannot see bound's value, passes whether or not new branch works — each such gap found at gate costs whole extra fix pass and re-gate.
