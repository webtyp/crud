---
PLAN: "feat!: crud loopback Context.RawQuery() (router v0.5.0)"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 17452436091097938068
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Open it after Stage 2 with whatever compiles and keep committing to it.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.

# Plan: the `crud` loopback follows the router Context contract

## Why (read once)

`webtyp.com/router` v0.5.0 changed the `router.Context` contract: `Path()` is the path only and the new `RawQuery() string` is the query (`"a=1&b=two"`, `""` when none), read with `router.QueryReader(ctx.RawQuery())`. This repo implements `Context`, so it stops compiling until it does the same. The contract is held by `router/conformance` (`Run` for transports served by a `Router`); a transport that disagrees fails it.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/router@v0.5.0
go mod tidy
```
No `replace` in `go.mod`. If a module cannot be fetched, stop and say so in the PR; never write a hash by hand.

## APIs used

| symbol | exact signature | tag | file |
|---|---|---|---|
| `router.Context.RawQuery` | `RawQuery() string` | router v0.5.0 | `router.go` |
| `router.PathOf` | `func PathOf(target string) string` | router v0.5.0 | `query_codec.go` |
| `router.QueryOf` | `func QueryOf(path string) string` | router v0.5.0 | `query_codec.go` |
| `router.DecodeArgs` | `func DecodeArgs(method, query string, body []byte, into model.Decodable, fromBody func(body []byte, into model.Decodable) error) error` | router v0.5.0 | `query_codec.go` |
If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — `loopback.go`

The loopback context carries no URL: an operation is called by name with arguments, never by address. Its answers are the contract's empty ones. Next to `Path()` (line 163) add:

```go
// RawQuery is empty: a loopback call has no URL, its arguments travel in the body.
func (c *loopbackContext) RawQuery() string { return "" }
```

Then run `go vet ./...` at the root and in `tests/` (nested module if `tests/go.mod` exists): every type of the repo's tests that implements `router.Context` (a fake context) fails to compile until it has `RawQuery() string { return "" }`; add exactly that method to each, one file at a time. `grep -rn "SetAssurance(" --include=*_test.go .` lists them.

## Stage 2 — tests and docs

| Test | setup → call → expected |
|---|---|
| `TestLoopbackContext_HasNoURL` (`tests/loopback_test.go`, new file if none) | a handler registered through `crud.LoopbackActingAs` that writes `ctx.Path() + "|" + ctx.RawQuery()` answers `|` (both empty) |

Add its row to `docs/TESTS.md` (`| [Name](../tests/file_test.go) | rule |`). One sentence in `docs/ARCHITECTURE.md`: the loopback has no URL, `Path()` and `RawQuery()` are empty.

## Delivery check

`gotest` passes (it builds, vets, races and runs every destination); the pull request is open. A tree that does not compile or whose tests did not run is a failed delivery.
