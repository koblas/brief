---
name: clean-architecture
description: Go service architecture for this monorepo — the cmd/services/libs/gen layout, thin-binary + service-package split, functional-options DI, the Store-interface + adapters port pattern, Connect-RPC / ogen handler shape, the dependency rule (what may import what), and Go code + test conventions. Invoke when deciding where code lives, what a package may import, how to structure a new service or handler, or how to wire dependencies.
---

Codebase **Go**, service-oriented, on **Connect-RPC** (internal) + **ogen / OpenAPI** (edge), protobuf-generated. NOT layered Vaughn-Vernon / IDDD way — no aggregates, no `UseCase` classes, no `domain/application/infrastructure/api` folders, no `Repository`-per-aggregate, no `.contract` files. Carryover from Clean Architecture = *spirit*: strict dependency direction, ports-and-adapters at persistence boundary, thin delivery handlers, deps injected (never reached for). Below = how looks here.

## Repository layout

```
go/
  cmd/<group>/<service>/        # THIN binary — wiring + deployment entrypoints only
    shared.go                   #   Config struct + buildMux (parse config, build deps, construct service)
    main_compose.go             #   //go:build !lambda — local/compose entrypoint
    main_lambda.go              #   //go:build lambda  — AWS Lambda entrypoint
    Dockerfile, Dockerfile.tilt
    groups: core/fleet · publicapi/{edge,oidcas,websocket} · trigger/* · websocket/* · workers/*
  services/<group>/<service>/   # the service package — all business logic + handler + stores
    core/{authn,oauthclient,oauthuser,sendemail,todo,user}   # internal Connect-RPC services (behind the fleet)
    publicapi/{auth,feed,file,gpt,oidcas,todo,user,websocket} # edge-facing handlers (ogen or Connect)
    publicapi/{bootstrap.go,server.go,bearer.go}             # composes the pubapi ogen Handler from sub-handlers
  libs/<lib>/                   # shared, generic infrastructure (no service deps)
    fxkv · fxpubsub · keymanager · oidckeys · interceptors · bufcutil · reqaudit ·
    attempts · oapiutil · otelkit · confmgr · manager · redisutil · dynokit · ...
  workers/<worker>/             # event-bus consumers (authnrevoke, feed, usersecurity, bucketfile)
  gen/                          # GENERATED — Connect stubs, ogen servers, protobuf. NEVER hand-edit.
protos/                         # proto sources → gen/ via `buf generate` (see proto-regen-loop skill)
```

Two service families:
- **core/** — internal Connect-RPC services. Aggregated into one binary, `cmd/core/fleet`, mounts each via service's `Bootstrap(...)`. Reached only over Connect by other services.
- **publicapi/** — public HTTP edge. `cmd/publicapi/edge` composes pubapi handlers via `publicapi.NewHandler(WithXHandler(...))` + `publicapi.Boostrap`. `cmd/publicapi/oidcas` = standalone OIDC IdP front (ogen).

## The thin-binary / service-package split

`cmd/<group>/<service>` = **wiring only** (mirror `cmd/publicapi/edge`): `shared.go` holds `Config` struct + `buildMux` — parses config, builds concrete deps (KV via `fxkv.New`, Connect clients, Redis, KMS, publishers), constructs service with functional options, mounts it (+ healthz, reqaudit middleware). `main_compose.go` / `main_lambda.go` = build-tagged entrypoints supplying deployment-specific defaults. **No business logic under `cmd/`.**

`services/<group>/<service>` = service package (`package <name>`) holding handler, business logic, stores.

## Service package shape (functional-options DI)

Every service package follows shape (see `services/core/user`, `services/publicapi/auth`, `services/publicapi/oidcas`):

```go
package user

type Server struct {        // private fields = injected dependencies
    store   Store
    authn   authnv1connect.AuthNServiceClient
    pub     fxpubsub.BytesPublisher
}

type Option func(*Server)                                    // functional options
func WithStore(s Store) Option        { return func(x *Server) { x.store = s } }
func WithAuthnService(c …) Option      { return func(x *Server) { x.authn = c } }

func NewUserServer(opts ...Option) *Server {                 // constructor
    s := &Server{ /* defaults */ }
    for _, o := range opts { o(s) }
    // panic on a missing required dep, or default it
    return s
}

var _ userv1connect.UserServiceHandler = (*Server)(nil)      // compile-time guard

// Bootstrap returns the Connect mount path + handler for the fleet to mount.
func Bootstrap(config []Option, options []connect.HandlerOption) (string, http.Handler) {
    return userv1connect.NewUserServiceHandler(NewUserServer(config...), options...)
}
```

Rules:
- **Functional options, not constructor-arg lists or DI framework.** Every dep gets `WithX(...) Option`.
- **Compile-time interface guard** (`var _ Iface = (*Server)(nil)`) for generated Connect/ogen handler service implements.
- **Handlers thin.** Handler method decodes typed request, calls business logic / stores / downstream Connect clients, returns typed response. No business rules in `buildMux`; no transport in business funcs.
- For ogen edge handlers, keep HTTP-shaped bridge (response types, cookies, redirects) separate from pure decision logic, so logic testable without HTTP (see `services/publicapi/oidcas` — `doAuthorize` vs `APIV1…` bridge method).

## Ports & adapters (the persistence boundary)

Where "ports and adapters" actually lives in Go here:

- **`Store` = persistence port**, interface defined **in service package that consumes it** (`services/core/user/store.go`, `services/core/authn/store.go`). Not in separate `domain/` tree.
- **Adapters sit beside it in same package**: production adapter (`dynamo.go`, or `fxkv`-backed `kvStore`) + in-memory adapter (`memory.go` / `memorykv`) for dev + tests. Convert domain↔persistence at adapter boundary.
- **`libs/fxkv.KV` = key-value port** — deliberately minimal (`Get/Exists/Set/SetNX/Delete`). Do **not** extend; build higher-level behavior in service's Store, or use richer backend behind Store interface.
- **Downstream services = ports too**, but port = *generated Connect client interface* (`…v1connect.XServiceClient`); adapter = generated client. Mock with **minimock** (`-s _mock.go`, wired in Makefile) for tests.
- Name `Store` (+ `dynamo`/`memory` adapters). No `Repository`, `Finder`, `Reader`, or aggregate package here.

## Dependency rule (strict — load-bearing part)

Deps point one way:

```
cmd/*  ──►  services/*  ──►  libs/*  +  gen/*
                │
                └─► other services ONLY via generated Connect clients (gen/…v1connect), never their package
```

- `cmd/*` may import `services/*` + `libs/*` (wiring). Nothing imports `cmd`.
- `services/*` may import `libs/*` + `gen/*`. **Service never imports another service's package** — cross-service calls go over Connect via generated client. (core services reached through `cmd/core/fleet`.)
- `libs/*` import only other `libs/*` + `gen/*` — generic, no service knowledge.
- `gen/*` = leaf. Generated; **never hand-edited** — change `.proto` + regenerate (see `proto-regen-loop` + `pubapi-gen-cwd` skills).

Two services need shared types → types live in proto (shared message) or `libs/*` package — never imported across service boundaries.

## Cross-service wiring in `cmd/*` (MANDATORY)

Never hand one service's `*Server` to another service's `WithX` that expects a generated Connect
client. It compiles — a Connect unary handler and unary client share method signatures — but the
interceptor chain is mounted at `NewXServiceHandler(svc, options...)`, so the in-process hop skips
**all of it**: protovalidate (the only reader of `buf.validate` annotations), the error-mapping
interceptor, and otel spans/metrics. The same RPC over a real client enforces all three, so one
proto contract ends up with two regimes.

Give it a client that runs the interceptors, or record the decision explicitly — naming the
specific RPCs and annotations that stop being enforced on that path.

**Be aware there is no worked production pattern for this in the repo today.** The options in
`go-testing`'s decision table (minimock, hand-written fake, `httptest.NewServer` + generated
client) are all test-side constructs; none is a wiring pattern for a single-binary chassis. So
today the compliant choices are: run the two services as separate binaries and use a real network
client (what `cmd/edge/*` does), or accept the risk with the record above. If you need a third —
an in-process transport that still runs the handler's interceptor chain — that is unbuilt, and
building it would close the accepted risk rather than work around it. Do not read the absence of a
pattern as permission to skip the record.

`go/cmd/rpc/chassis/shared.go` is a known, accepted instance
(`docs/specifications/account-and-policy/REVIEW-04.md` MAJOR 1). Full rule, and the test-side
equivalent with its decision table, in the `go-testing` skill under "Cross-service dependencies
in tests".

## Error handling at boundaries

- **Connect handlers**: return `bufcutil.InvalidArgumentError / NotFoundError /
  FailedPreconditionError / InternalError(...)` so right Connect status code surfaces.
- **ogen / HTTP handlers**: return generated typed error response, or map sentinel through `NewError` (see `publicapi.Handler.NewError` → `oapiutil.SharedNewError`, + oidcas' `tokenErr` → `NewError`). Keep wire error shape in generated envelope, not ad-hoc JSON.

## Go code conventions

- **Functional options** for construction; **interfaces defined at point of consumption** (the `Store` in service, the `KV` in `libs/fxkv`), kept small.
- **No `I` prefix** on interfaces (matches rest of doc's lineage).
- Prefer **explicit, intention-revealing names**; extract repeated literals into **named constants** (TTLs, prefixes, limits).
- Protobuf messages use generated `GetX`/`SetX`/`…_builder` accessors; plain Go values in domain code.
- Keep `libs/*` generic + dep-light; service-specific logic belongs in service package, not lib.
- **Comments are written for `go doc`** — package `doc.go`, contract-shaped function docs, no history. See *Documentation & comments* below.
- `go/gen` generated — `gen.*` symbol missing → regenerate; never edit generated files.

## Documentation & comments (MANDATORY — write for `go doc`)

`go doc` output is the package's contract. It must convey *intention* — what a package
and each function is for — well enough that a reader (or a model) never has to open the
source. Comments that only make sense next to the diff that produced them are noise in
that output.

### 1. Every package has a `doc.go`

One `doc.go` per package, holding the package comment and nothing else (no code). Starts
`Package <name> ` and says what the package *does* and what it owns — not how it grew.

```go
// Package oidcas implements the OpenID Connect authorization server: the
// /authorize, /token, /userinfo, /revoke and /end_session endpoints, the AS
// session cookie, and the login, MFA, recovery and social-login flows the
// auth SPA drives.
//
// Transport lives in the ogen bridge; each endpoint's decision logic is a
// transport-free method on Server so it is testable without HTTP.
package oidcas
```

### 2. Every exported symbol has a doc comment; unexported ones that carry a decision do too

- **Starts with the symbol name**, then a complete sentence: `// mintSession creates …`,
  not `// creates …` or `// This function creates …`. `go doc` prints it verbatim.
- States **what it does, what it returns, and what it refuses** — the contract a caller
  needs. Name the error sentinels and the conditions that produce them.
- **Cite the standard when the behaviour is one.** `RFC 6749 §5.1`, `OIDC Core §3.1.2.1`,
  `RFC 7009 §2.2` — section symbol included. A citation replaces a paragraph of
  explanation and is the one form of "why" that never goes stale.
- Keep it **tight**: a sentence or three. Detail that belongs to one branch belongs at
  that branch, not in the doc comment.

### 3. Never document history in a comment

Comments describe the code as it **is**. The following all belong in git, the PR, or a
report under `.claude/reports/` — never in a doc comment:

- `// Previously this …`, `// Used to …`, `// Changed to …`, `// Now that …`
- dated narrative: `// Until the 2026-09-10 audit …`, `// Phase 7 added …`
- review/finding ids as narrative: `(R1 finding)`, `(audit finding M5)`, `(D3)`,
  `SCENARIO-04` — the durable fact is the *rule*, so state the rule
- `// TODO(name)` describing a past decision rather than pending work

```go
// Bad — history; a reader learns about a diff, not about the function.
// issueRefreshToken used to always mint a token; since the M-series audit it
// is gated on offline_access (see the 2026-09-10 report).

// Good — the rule, stated as the contract it is.
// issueRefreshToken mints a refresh token for ac and persists the binding the
// refresh grant reads. Callers gate it on the offline_access scope
// (OIDC Core §11); it does not check the scope itself.
```

### 4. Inside a function, comment only the non-obvious *why*

Delete anything that restates the next line, narrates the flow, or records how the code
got here. What earns a comment inside a body:

- an **ordering constraint** or an invariant that is not visible locally
  (`// Consume before the binding checks: a rebind window would let a bad PKCE retry.`)
- a **fail-safe** choice whose opposite looks reasonable
  (`// A transport fault is not evidence the identity is gone; keep the session.`)
- a **spec requirement** that explains an otherwise odd branch, with its citation

```go
// Bad — apparent behavior; the code already says this.
// Loop over the enrollments and find the matching id.
for i := range state.Enrollments {

// Bad — history.
// We moved this above the client lookup in the ogen migration.

// Good — the constraint the reader cannot see.
// Rate-limit after client_id validates, so a junk client_id cannot consume a
// victim's bucket.
```

### 5. Verify what the reader actually gets

`go doc` is cheap and is the check: read the package the way a caller will.

```bash
go doc ./services/<group>/<name>          # exported surface + package doc
go doc ./services/<group>/<name> <Symbol> # one symbol
go doc -all ./libs/<name>                 # everything, incl. unexported when -u
```

If the output does not say what the package is for, or a function's line does not say
what it guarantees, the comment is wrong — not the reader.

Scope: production Go (`services/`, `cmd/`, `libs/`, `workers/`). Generated code under
`gen/` is never hand-edited. Test files follow the `go-testing` skill; `go doc` ignores
them, so their comments explain the *rule under test*.

## Testing conventions (summary — defer to `testing` skill for depth)

- **testify** (suite optional). Declarative tests, Given-When-Then separated by blank lines, no control flow in test bodies (`.claude/CLAUDE.md`).
- **White-box** (`package foo`) tests normal for exercising unexported logic; external `foo_test` fine too.
- **In-memory adapters** (`memory.go`, `libs/fxkv/memorykv`) + **miniredis** for backend tests; **minimock** for generated Connect client deps; **httptest** for ogen HTTP-dispatch tests.
- **No injected clock.** Production calls `time.Now()` directly; time-dependent tests use Go 1.26 `testing/synctest` bubbles to control/advance fake clock (allocate otter/memorykv-backed stores *outside* bubble — background goroutines must not live in it). Do not thread `func() time.Time` through production code.

## When adding a new service (checklist)

1. Define/extend `.proto`; regenerate (`proto-regen-loop` skill).
2. Create `services/<group>/<name>/`: `doc.go` (package doc — see *Documentation & comments*), `Server` struct, `Option`s + `WithX`, `New<Name>Server`, compile-time guard, `Store` interface (+ `memory` + real adapters if persists), `Bootstrap(...)` for core services.
3. Keep handler methods thin; business logic in pure funcs; persistence behind `Store`; downstream calls via generated Connect clients.
4. Create `cmd/<group>/<name>/` only if own binary: `Config` + `buildMux` + `main_compose.go` + `main_lambda.go` + Dockerfiles, mirroring existing sibling (`cmd/publicapi/edge` or `cmd/publicapi/oidcas`). Else mount in appropriate aggregator (`cmd/core/fleet` for core; `publicapi` bootstrap for edge).
5. Tests per conventions above.