---
name: pubapi-gen-cwd
description: Enforce correct working directory for Go + buf + npm commands in this monorepo. Auto-invokes whenever a Bash command fails with `pattern ./...: directory prefix ... does not contain modules listed in go.work`, `cannot find main module, but found .git/config`, `no required module provides package`, `pattern ./...: directory prefix . does not contain main module`, or any "directory not found" error on a Go path. Also use proactively before running `go test`, `go build`, `go generate`, `golangci-lint`, `buf generate`, `make`, or `npm run`.
allowed-tools: Bash
---

# Working-Directory Discipline

Monorepo got three build roots. Wrong root = cryptic errors look like missing modules/packages/files. Fix always "cd to right root first".

## The Three Roots

| Root     | Absolute path | Owns                                                      | Use for                                                                                          |
| -------- | ------------- | --------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Go       | `go`          | `go.mod`, `go.work`                                       | `go build`, `go test`, `go generate`, `golangci-lint`, `make` (Go), `buf generate` (core protos) |
| OpenAPI  | `papi`        | `buf.gen.app.yaml`, `buf.gen.oidcas.yaml`, generated yaml | `make` (yaml regen), `buf generate --template ...`                                               |
| Frontend | `frontend`    | `package.json`, `tsconfig.json`                           | `npm run *`, `npx tsc`, `npx vite`                                                               |

Repo root for `git`, `protos/` edits, reading reports. NEVER run `go` / `npm` / `make` from it.

## Always-Wrong Symptoms

Match error string verbatim. No retry without cwd change.

| Error includes                                                                      | Real cause                                                                               | Fix                                                      |
| ----------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `directory prefix ./... does not contain modules listed in go.work`                 | Go tool from repo root                                                                   | `cd go` first                                            |
| `cannot find main module, but found .git/config`                                    | Same; at repo root                                                                       | `cd .../go` first                                        |
| `directory prefix . does not contain main module or its selected dependencies`      | `go build ./...` from repo root                                                          | `cd .../go` first                                        |
| `pattern ./cmd/publicapi/oidcas/...: directory prefix ... does not contain modules` | Same                                                                                     | `cd .../go` first                                        |
| `stat /Users/.../cmd/publicapi/...: directory not found`                            | Go tool from `papi/` or `frontend/`                                                      | `cd .../go` first                                        |
| `pattern ./gen: ...` after successful `buf generate`                                | `go generate ./gen` from outside `go/`                                                   | `cd .../go` first                                        |
| `npm error Missing script: "..."`                                                   | `npm run X` from repo root, not `frontend/`                                              | `cd .../frontend` first                                  |
| `buf: failed to read inputs: ... no such file or directory` on `../protos`          | `buf generate` from outside dir holding `buf.yaml`/`buf.gen.yaml`                        | `cd .../go` (core) or `cd .../papi` (app/oidcas)         |
| `grep: go/services/...: No such file or directory` — path that definitely exists    | **cwd persisted from a previous `cd`** (see Rule 1); repo-relative path resolved against `go/`/`frontend/` | Re-run with `cd /absolute/repo/root && ...`             |
| `cd: no such file or directory: frontend`                                           | Same — already inside `frontend/` (or `go/`) from an earlier call                        | Absolute `cd`, never relative                            |
| `zsh: no matches found: --include=*.go`                                             | zsh globs the unquoted flag before `grep` sees it                                        | Quote it: `--include="*.go"`, or scope by directory instead |

## Commands By Root

### Go root (`cd go`)

```bash
go build ./...
go build -tags lambda ./cmd/publicapi/edge
go test ./cmd/publicapi/oidcas/...
go test ./services/core/...
go generate ./gen           # regen ogen from yaml
golangci-lint run ./libs/...
make protos                 # buf generate core/* + minimock
make codegen                # protos + openapi
buf generate                # core/* only; see buf.gen.yaml inputs
```

### Papi root (`cd papi`)

```bash
make                                       # runs both buf-generate templates
buf generate --template buf.gen.app.yaml
buf generate --template buf.gen.oidcas.yaml
```

### Frontend root (`cd frontend`)

```bash
npm install
npm run openapi:gen           # both app + oidcas TS
npm run openapi:app
npm run openapi:oidcas
npx tsc --noEmit
npx vite                      # dev server
```

## Rules

1. **Default to absolute `cd` paths.** cwd **DOES persist across separate `Bash` calls** —
   verified: `cd .../go && pwd` then a fresh call with bare `pwd` still prints `.../go`.
   (This rule previously claimed the opposite. It was wrong, and the wrong version caused
   repeated failures: after a `cd go`, repo-relative paths like
   `go/services/.../server.go` or `papi/openapi_oidcas.yaml` resolve against `go/` and
   fail with `No such file or directory`, and `cd frontend` fails with
   `cd: no such file or directory: frontend`.) Never rely on inheriting a cwd. Always lead
   with `cd /absolute/path && actual_command`.

2. **Never `cd .. && ...`** to jump roots — easy to miscount, land at repo root running Go. Absolute always.

3. **Verify after `cd`.** Unsure? `pwd && command` so failure tell you which root shell was in.

4. **Combine within one root.** `cd X && cmd1 && cmd2` fine; shell stays in X for chain. Second `Bash` call need own `cd`.

5. **Repo root commands.** Only `git`, `find`, `grep`, editing files outside three roots safe from repo root.

## Quick Decision

| You want to...                            | cd to          |
| ----------------------------------------- | -------------- |
| Build/test/lint any Go code               | `.../go`       |
| Run/install npm dependency or generate TS | `.../frontend` |
| Regenerate openapi yaml from proto        | `.../papi`     |
| Run `git`, edit a `.proto` file           | repo root      |