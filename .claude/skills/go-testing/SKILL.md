---
name: go-testing
description: Use whenever writing, modifying, or reviewing tests in this project. Defines the expected style for unit tests and handler (Connect-RPC / ogen HTTP slice) tests. Go-specific — examples use testify, table-free declarative style, hand-written fakes, minimock for generated Connect clients, httptest for ogen handlers, and synctest for time.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
paths: *.go
---

Go, `testing` + **testify** (`assert`/`require`; `suite` optional). Tests table-free, declarative. Project conventions enforced by `test-reviewer` agent.

## Black-box package by default (MANDATORY)

**Every test file MUST declare `package <name>_test`, not `package <name>`.**
Tests exercise component through _exported_ surface — same surface caller sees. Default. Reviewer **flags any `package <name>` (white-box) test file as violation** unless justified (below).

```go
// Good — black-box: external package, public surface only.
package oidcas_test

import "github.com/koblas/content_buddy/go/services/publicapi/oidcas"

// Bad — white-box: same package, reaches unexported funcs. Requires justification.
package oidcas
```

- **Why:** black-box tests can't bind to internals, so survive refactors and document contract. Test reaching unexported func couples to decision not in contract.
- **Narrow exception (must be justified in-file):** white-box `package <name>` permitted ONLY for **unexported decision logic extracted for combinatorial reasons** that can't be driven economically through public surface (e.g. oidcas `doAuthorize`, `verifyPKCE`, `parseAuthorizeRequest`). When taking exception:
  - Keep extracted func unexported — do **not** widen visibility for test.
  - Put white-box tests in own file (e.g. `authorize_internal_test.go`) with top-of-file comment naming unexported unit and why public surface won't do. Everything testable through public API stays black-box.
- **Don't** reach for white-box just to read private field or skip wiring — that's the violation default exists to stop. Restructure for testability instead.

## Test structure (mandatory)

Every test follows **Given-When-Then**, separated by blank lines, no `// Given` comments.

- **Given/When/Then must trace cleanly.** Every value When/Then references must be explicit in Given. When test queries or asserts by specific id, status code, etc., setup must put that value on seeded data — don't rely on factory default. If value matters to assertion, make it explicit arg to test-data builder; don't bury as default.

```go
// Bad — DOMAIN is silently the builder default; the test reads as if domain doesn't matter
seed(t, store, row(userID, withCrawler("gptbot")))
found := result.find(domain)

// Good — DOMAIN is explicit in both the seed and the assertion
seed(t, store, row(userID, domain, withCrawler("gptbot")))
found := result.find(domain)
```

- Black-box `package <name>_test` default (see **Black-box package by default** above) — exercise through exported surface.
- Use minimum fixture/input needed to prove behavior; remove records irrelevant to assertion.
- When behavior provable with 1-2 domain values, don't use larger reference datasets.
- Prefer shared constants for recurring domain values over ad-hoc literals.
- **Construct system-under-test via `newXxx(t *testing.T)` helper called at top of each test** — Go analog of before-each. Shared fakes / `Server` / stores _every test uses identically_ built here. Never package-level `var`s (shared mutable state across tests). With `suite`, that's `SetupTest`.
- **Seed test data inside test body, never in helper/SetupTest.** Data setup (`store.save(...)`, `fake.add(...)`, `srv.codes.Mint(...)`) lives in each test so test self-contained.
- **Prefer enriching fake over building bespoke inline mock.** When test needs port to fail for one scenario, give project's fake small field + method (e.g. `failNext(err)`); fake still built in helper, `failNext(...)` call is per-test data setup in test body.

```go
// In the fake (a hand-written fake implementing the Store port):
type fakeFooStore struct {
    foos    map[string]*Foo
    nextErr error // set by failNext for one scenario
}

func (f *fakeFooStore) failNext(err error) { f.nextErr = err }

func (f *fakeFooStore) Find(ctx context.Context, id string) (*Foo, error) {
    if f.nextErr != nil {
        return nil, f.nextErr
    }
    return f.foos[id], nil
}

// In the test:
func Test_returns_error_when_the_lookup_fails(t *testing.T) {
    srv := newFooServer(t)
    srv.store.(*fakeFooStore).failNext(errBoom)

    _, err := srv.Run(t.Context(), userID)

    require.ErrorIs(t, err, errBoom)
}
```

```go
func Test_returns_expected_occupancy_when_capacity_is_available(t *testing.T) {
    srv := newOccupancyServer(t, withGuests(lowGuest, highGuest))

    result, err := srv.Calculate(t.Context(), request)

    require.NoError(t, err)
    assert.Equal(t, 2, result.TotalAssignedRooms)
}
```

## Mandatory Review

**Every new or modified test must be reviewed by `test-reviewer` agent.**

## Naming

- Test function: `func Test_<behavior_in_camel_case>(t *testing.T)` or `func Test<Unit>_<behavior>(t *testing.T)` — must start with `Test`. Subtests (`t.Run("...")`) use natural-language string.
- Failure scenarios prefer `FailsWhen<condition>` / `ReturnsErrorWhen_…` over `PanicsWhen…`.
- Handler tests prefer `Returns<status>When<condition>` (HTTP) or `Returns<connect_code>When<condition>` (Connect).
- **Use plain business language, not invented jargon.** Avoid verbs domain doesn't use (`credits`, `honors`, `respects`). Prefer `ignores…`, `returns…`, `saves…`, `rejects…`. `IgnoresSightingsFromOtherUsers` reads better than `OnlyCreditsSightingsBelongingToTheRequestedUser`.
- **Name behavior, not mechanism.** Name should communicate observable outcome from caller's view, not internal implementation.
  - **Exception: when mechanism IS guarantee, name it.** Cache internal — but if "don't refetch on second call" is contract, name it (`DoesNotRefetchOnSecondCall`).
  - **Incidental → drop it:** `FailsWhenTheRtmapLookupErrors` leaks internal table; contract is `ReturnsInvalidGrantWhenTheRefreshTokenIsUnknown`.

## Logic in Tests (Forbidden)

**Never use `if`, `else`, `while`, `switch`, in test body.**
Tests stay declarative, linear. If branching seems necessary, split scenarios or redesign setup.

## One behavior per test

Each test verifies one behavior. If test name needs "and", split. Multiple `assert`/`require` calls each proving _different_ behavior (not different facets of same outcome) is same smell.

- **Watch seed shape, not just assertions.** Test can name one rule but exercise two if seed shaped for both. Test named "returns distinct user IDs" seeding three rows for user A and one for B exercises both _dedup_ and _multi-user enumeration_; single `Len(…, 2)` does work for both. Split. Heuristic: if removing one _type_ of seed variation still proves named rule, that variation was testing different rule.

```go
// Bad — one test, two rules. Len(ids, 2) does work for both dedup AND enumeration.
func Test_returns_distinct_user_ids_of_users_with_non_deleted_domains(t *testing.T) {
    seed(t, store,
        row(userA, withDeletedAt(nil)),
        row(userA, withDeletedAt(nil)),
        row(userA, withDeletedAt(nil)), // over-specified — 2 rows is enough for dedup
        row(userB, withDeletedAt(nil)),
    )

    ids, err := finder.FindAll(t.Context())

    require.NoError(t, err)
    assert.Len(t, ids, 2)
    assert.ElementsMatch(t, []string{userA, userB}, ids)
}
```

```go
// Good — split. Each test seeds the minimum for the single rule it names.

func TestReturnsAUserIdOnlyOnceWhenTheUserHasMultipleNonDeletedDomains(t *testing.T) {
    seed(t, store,
        row(userA, withDeletedAt(nil)),
        row(userA, withDeletedAt(nil)),
    )

    ids, err := finder.FindAll(t.Context())

    require.NoError(t, err)
    assert.Equal(t, []string{userA}, ids)
}

func Test_returns_one_user_id_for_each_user_with_at_least_one_non_deleted_domain(t *testing.T) {
    seed(t, store,
        row(userA, withDeletedAt(nil)),
        row(userB, withDeletedAt(nil)),
    )

    ids, err := finder.FindAll(t.Context())

    require.NoError(t, err)
    assert.ElementsMatch(t, []string{userA, userB}, ids)
}
```

## Designing the test list (ZOMBIES + mutation check)

Before writing tests, walk **ZOMBIES** categories explicitly — systematic, not "interesting":

- **Z**ero — empty / nil / no-result input
- **O**ne — exactly one item (catches inversions and missing filters)
- **M**any — N>1: all-same, all-different, mixed
- **B**oundary — min/max, off-by-one, time-window edges
- **I**nterface — contract shape (types, fields, optional/required)
- **E**xceptions — errors, infrastructure faults, Connect error codes
- **S**imple — keep each scenario minimal

For every test ask **mutation question**:

> "If I flip an operator (`>` ↔ `<`, `== 1` ↔ `== 0`, `&&` ↔ `||`) or drop a
> filter clause in production, would this test catch it?"

If can't name mutation test rules out, vacuous — redesign or delete.

### Asymmetric data for discriminating filters

Test on filtering/aggregating function must use data distinguishing correct implementation from likely mutants. Symmetric data (one matching + one non-matching, asserting `== 1`) is smell — correct predicate and inversion both yield 1. Pick counts that diverge.

```go
// Bad — 1 matching + 1 non-matching, predicate `flagged`: count is 1 either way.
seed(t, store, row(withFlagged(true)), row(withFlagged(false)))
assert.Equal(t, 1, query.Count(t.Context()))

// Good — 2 matching + 1 non-matching: correct predicate yields 2, mutation yields 1.
seed(t, store, row(withFlagged(true)), row(withFlagged(true)), row(withFlagged(false)))
assert.Equal(t, 2, query.Count(t.Context()))
```

Reference: James Grenning, ["TDD Guided by Zombies"](https://blog.wingman-sw.com/tdd-guided-by-zombies).

## Assertions

- **testify**: `require` for fatal precondition (stop test — e.g. `require.NoError(t, err)` before dereferencing result); `assert` for rest. Argument order: `assert.Equal(t, expected, actual)`.
- **Precision-sensitive values**: use `assert.InDelta` / `InEpsilon` ONLY for values production legitimately makes fuzzy (currency rounding, accumulated float math). For anything you control, pick fixtures whose math resolves to exact integers and use `assert.Equal`. Tolerance assertion over fixture you chose to be fractional is smell.
- Avoid magic numbers; use named constants where meaning matters.
- **No redundant intermediate assertions.** Don't assert precondition already implied by next assertion (e.g. asserting `ok` from map lookup when next line asserts value — test fails anyway if absent). Exception: `require` guards preventing nil-deref panic fine.
- **Subset matchers NOT equality.** `assert.Contains` / `assert.Subset` pass when items _included_ — extras slip through. For "exactly these in any order" use `assert.ElementsMatch`; for exact ordered equality use `assert.Equal`.

## Test data minimality

- Smallest input/fixture set proving behavior.
- Oversized datasets where fewer records assert same rule are violation.
- Repeated raw domain literals → shared constants when values recur across tests.

## Repeated construction = extract a helper

- When same construction (`Server`, fake, fixture) appears identically in 3+ tests or across 2+ files, extract `newXxx(t *testing.T, opts...)` helper or fixture builder. oidcas tests do this with `newAuthorizeServer`, `newLoginServer`, `newTokenServer`.
- Helper absorbs incidental params (fakes, KV) so each test specifies only what matters to its scenario; use functional options on helper for per-test knobs.
- **Prepare seam before changing signatures.** Before adding parameter to constructor called in many tests, extract/extend helper first — one edit, not shotgun surgery. "Make the change easy, then make the easy change." — Kent Beck

## Test data visibility

- **All test data referenced in assertions must be visible in test body.** Package-level `var`s building test data used implicitly by tests are violation — reader shouldn't scroll to package scope to understand assertion. Pass data via helper or use named constants.
- **Don't assert on internal-signal return types.** If return value only consumed internally, don't write test only asserting on it; behavioral tests already prove it.
- **"Unchanged" assertions must use distinct before/after values.** When asserting "data unchanged after operation," two sides must have visibly different identifiers, else test passes vacuously.

## Test behavior, not library boundaries

- **When library implements your product behavior, behavior is still yours to test.** Library is implementation detail, not excuse to skip.
- **If behavior hard to test, restructure for testability first** (e.g. split pure decision out of HTTP bridge, as oidcas does with `doAuthorize` vs ogen handler). Most "untestable" behaviors are design signal.
- **Delete vacuous tests.** Test passing regardless of correctness worse than none. If can't make it fail by removing behavior, delete it.

## Testing Strategy & Efficiency

**Prefer fast, economical, deterministic tests.** Before reaching for slow/non-deterministic dependencies (disk, env vars, containers, network, real HTTP), exhaust cheaper options:

- **Unit tests** of pure funcs (e.g. `doAuthorize`, `verifyPKCE`, `parseAuthorizeRequest`) when combinatorial complexity makes going through handler impractical. Otherwise test through handler.
- **In-memory adapters** — `memory.go` Store and `libs/fxkv/memorykv` — instead of real backend.
- **Narrow handler tests** for adapter boundary (Connect handler or ogen HTTP handler), mocking only downstream Connect clients.

## Fakes over mocks (default)

- **Hand-written fakes for persistence port.** `Store` interface gets in-package fake (or reuse real `memory.go` adapter). Fake commonly embeds generated interface and overrides few methods exercised, so unimplemented methods panic if called unexpectedly:

```go
// Fake for a downstream Connect client: embed the generated interface, override
// what the test exercises. An unexpected call to anything else panics (nil method).
type fakeClients struct {
    oauth_clientv1connect.OAuthClientServiceClient // embedded — unimplemented methods panic
    clients map[string]*oauthclientv1.Client
}

func (f *fakeClients) GetClient(_ context.Context, req *connect.Request[...]) (*connect.Response[...], error) {
    c, ok := f.clients[req.Msg.GetId()]
    if !ok {
        return nil, connect.NewError(connect.CodeNotFound, nil)
    }
    resp := &oauthclientv1.OAuthClientServiceGetClientResponse{}
    resp.SetClient(c)
    return connect.NewResponse(resp), nil
}
```

- **minimock for generated Connect clients in slice tests** acceptable and standard (`minimock -s _mock.go`, wired in Makefile; see `services/publicapi/todo/handler_test.go`). Use hand-written fake when want stateful behavior (seeding, sequencing); use minimock when only need to stub call or assert delegation.
- **Test `Server`/handler concretely**, not behind interface introduced only for tests. Don't widen visibility for tests, and don't drop to white-box `package foo` to reach internals — black-box `package foo_test` against exported surface is default (see **Black-box package by default**).
- **Fresh instances per test** via `newXxx(t)` helper. No `Reset()`/`Clear()` methods on fakes — construct new one. For real resources needing teardown, use `t.Cleanup` (e.g. `miniredis.RunT(t)` auto-cleans).

```go
type fakeGuestStore struct{ guests []Guest }

func (f *fakeGuestStore) FindAll(context.Context) ([]Guest, error) {
    return append([]Guest(nil), f.guests...), nil
}
```

## Cross-service dependencies in tests (MANDATORY)

**Never pass one service's `*Server` to another service's `WithX` / constructor that expects a
generated Connect client.** It compiles only because a Connect unary *handler* and a unary
*client* coincidentally share method signatures — it is not a real seam. The test then never
exercises `connect.Error` wrapping, error codes, or transport failure modes, and it couples the
caller's suite to the callee's internals.

```go
// WRONG — compiles, proves less than it looks like it proves.
accountSvc := account.NewServer(account.WithStore(store))
srv := user.NewUserServer(user.WithAccountService(accountSvc))
```

Pick by **what the test's own assertions are about**:

| The test needs | Use | Example |
|---|---|---|
| A specific response or failure from the dependency | `minimock` client mock, canned `.Return(...)` per test | `core/user/resolve_test.go`, `publicapi/session/exchange_http_test.go` |
| Stateful behavior across several calls | Hand-written fake implementing the client interface | see **Fakes over mocks** above |
| The callee's *real* emergent behavior (provisioning, claiming, a shared keyspace) | Real handler behind `httptest.NewServer`, hand out the **generated client** (socket-bound — cannot run inside a `synctest` bubble; use a mock there) | `core/user/provision_test.go`'s `newAccountClient`, `oauthuser/upsert_test.go`'s `newIdentityClient` |

```go
// RIGHT — real behavior, real client seam. libs/connecttest.MountConnect owns the
// mux + httptest.Server + t.Cleanup + client construction; the wrapper only names
// what is being mounted. Pass BOTH returns of the generated handler constructor —
// the path is what the mux routes on.
func newAccountClient(t *testing.T, store account.Store) accountv1connect.AccountServiceClient {
    t.Helper()

    path, handler := accountv1connect.NewAccountServiceHandler(account.NewServer(account.WithStore(store)))
    return connecttest.MountConnect(t, path, handler, accountv1connect.NewAccountServiceClient)
}
```

Do not hand-roll the mount. `connecttest.MountConnect` exists because this shape was duplicated
five times across three packages before it was extracted.

**Several consumers must share one dependency's state?** That is still not a reason for a bare
`*Server`. Mount it **once** and hand the **same** generated client to every consumer — shared
mutating state is preserved and the seam stays real.

```go
identityClient := newIdentityClient(t, identityStore) // one server, one client
userSrv := user.NewUserServer(user.WithIdentityService(identityClient))
oauthSrv := oauthuser.NewOauthUserServer(oauthuser.WithIdentityService(identityClient))
```

This rule also covers `libs/*` constructors that take a generated client
(e.g. `interceptors.NewResolvingUserIDExtractor`).

**Production wiring is not exempt — and the cost there is different.** The same shape in a
`cmd/*` binary (handing `account.NewServer(...)` to another service's `WithX`) does more than
weaken a test: interceptors are mounted at `NewXServiceHandler(svc, options...)`, so an
in-process call **skips the entire interceptor chain**. In this repo that chain is
`otelkit.ServerDefaultOptions()` + `interceptors.NewErrorInterceptor()` +
`validate.NewInterceptor()`, so the in-process caller loses **all three**: no protovalidate (and
protovalidate is the only thing in the Connect path that reads `buf.validate`, so those
constraints silently do not apply), no error-mapping interceptor, and no server spans/metrics —
the hop is invisible to tracing. The identical RPC over a real client gets all three. Two
regimes for one contract.

Before wiring one service's `*Server` into another inside a `cmd/*` binary, either give it a
client that runs the interceptors, or record the decision explicitly — naming which annotations
stop being enforced on that path. `go/cmd/rpc/chassis/shared.go` is a known, accepted instance;
see `docs/specifications/account-and-policy/REVIEW-04.md` MAJOR 1.

## Response sequencing for external-call fakes

- **One fake per port, with response sequencing** — slice of responses (or call counter) fake plays back in order. Do NOT create separate fake types for success / error / timeout (`throwingClient`, `failingClient`, `spyClient`) — unify into one configurable fake. Model variants with small response struct (`type resp struct { out *Foo; err error }`) and `[]resp` fake advances through.
- **Auto-advance**: each call consumes next response; last repeats once exhausted. No manual `advance()`.

## Handler tests (slice standard)

Two handler kinds in repo — pick matching pattern:

**Connect-RPC handler (core + some publicapi):** call handler method directly with typed `connect.Request`; mock/fake downstream Connect clients; assert Connect code first, then message.

```go
func TestReturnsNotFoundWhenTheClientIsUnknown(t *testing.T) {
    srv := oauthclient.NewServer(oauthclient.WithStore(newMemoryStore(t)))

    req := &oauthclientv1.OAuthClientServiceGetClientRequest{}
    req.SetId("missing")
    _, err := srv.GetClient(t.Context(), connect.NewRequest(req))

    assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

**ogen HTTP handler (publicapi edge / oidcas):** mount generated server under `httptest.Server` (`newOgenTestServer(t, srv)` helper) and issue real request; assert HTTP status first, then body/headers/cookies.

```go
func TestReturns302ToLoginWhenNoSession(t *testing.T) {
    srv := newAuthorizeServer(t, clients)
    ts := newOgenTestServer(t, srv)
    httpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
        return http.ErrUseLastResponse
    }}

    req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/authorize?"+validQuery().Encode(), nil)
    require.NoError(t, err)
    resp, err := httpClient.Do(req)
    require.NoError(t, err)
    t.Cleanup(func() { _ = resp.Body.Close() })

    require.Equal(t, http.StatusFound, resp.StatusCode)
    assert.Contains(t, resp.Header.Get("Location"), "/auth/login?login_tx=")
}
```

For pure decision logic, prefer calling extracted pure func directly (no `httptest`) — faster, clearer (oidcas: `runAuthorize` → `doAuthorize`).

## API validation matrix (what to cover)

For create/update endpoints (HTTP) or mutating RPCs (Connect):

- Happy path (`200`/`201`/`204` HTTP, or `OK` Connect).
- Malformed input / parse error -> `400` (HTTP) / `InvalidArgument` (Connect).
- Missing required field -> `400` / `InvalidArgument`.
- Domain invariant violation -> `400` / `InvalidArgument` / `FailedPrecondition`.
- Resource not found on update/delete/get -> `404` / `NotFound`.
- Unexpected infra/runtime failure where applicable -> `500` / `Internal`.

For ogen handlers, malformed/typed-decode failures rejected by generated request decoder before handler runs — test that boundary too (missing required param returns 4xx from ogen, not your code).

## Async / time / concurrency tests

- **No real `time.Sleep` for waiting, no injected clock.** Production calls `time.Now()`; time-dependent tests run inside Go 1.26 `testing/synctest` bubble and advance fake clock with `time.Sleep` + `synctest.Wait`.
- **Allocate backends with background goroutines OUTSIDE bubble.** `memorykv` wraps otter (background eviction goroutines) and `miniredis` spawns server goroutine — construct in outer test body and pass pointer in, else bubble won't reach "all durably blocked".

```go
func TestSessionExpiresAfterMaxAge(t *testing.T) {
    mem, err := memorykv.New(1024)   // built OUTSIDE the bubble (otter goroutines)
    require.NoError(t, err)

    synctest.Test(t, func(t *testing.T) {
        srv := newServerWith(mem)
        sess := mustCreateSession(t, srv)

        time.Sleep(2 * time.Hour) // fake clock — instant
        synctest.Wait()

        _, ok := srv.activeSession(t.Context(), sess.ID, /*maxAge*/ 3600)
        assert.False(t, ok)
    })
}
```

- Library that itself reads wall-clock (e.g. TOTP via `pquerna/otp`) must stay on real time — do NOT bubble it, else generated code desyncs from library's window. httptest-based dispatch tests also stay real-time (real sockets can't live in bubble); they assert routing, not time progression.

## Test file size & grouping

- Files over ~300-400 lines covering unrelated behavior should be split by behavior/endpoint.
- Tests live at package's public boundary in `package <name>_test` (default — see **Black-box package by default**). White-box `package <name>` is justified exception, only for extracted unexported decision logic.

## Backend integration tests (real DB / Redis, contract-style)

- Use project's local-backend harness: `awsutil.LocalDynamoClient` + `dynokit` for DynamoDB (`dynamo_test.go`), `miniredis.RunT(t)` for Redis.
- Verify CRUD contract: save/read, list-empty/list-populated, update, delete-existing, delete-missing idempotence.
- Slower; gate breadth — prove storage contract here, prove business rules against in-memory adapter.

## Contract tests for the Store port

- `Store` with both `memory` adapter and real adapter (`dynamo`) should share one contract test: `func testStoreContract(t *testing.T, mk func() Store)` exercised once per adapter. Both must pass identically — that's what lets tests trust in-memory fake stands in for production.

## What to test

For **handlers / Server methods**: happy path, empty results, validation + edge cases, error handling (correct Connect code / HTTP status), delegation to downstream clients, persisted-object shape.

For **pure funcs** (decision logic, parsers, PKCE/verify): full ZOMBIES set.

For **mappers / proto↔domain conversion**: field-mapping + value-conversion correctness.

## What NOT to test

- Unexported funcs _directly_ when already covered through handler/pure entry point (extract + test directly only for combinatorial explosion; keep extracted thing unexported).
- Trivial getters/`GetX`/`SetX` with no logic.
- Generated code (`go/gen`) and framework/library internals.
- `Server`/handler behind test-only interface — test concretely.
