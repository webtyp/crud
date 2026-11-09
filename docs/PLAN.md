---
PLAN: "refactor!: crud validates, the handler answers crud.RuleAnswer, crud publishes after commit; Caller becomes CurrentUser; Module.Definition is checked at Mount"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

Stage files: [0 names](PLAN_STAGE_0_NAMES.md) · [1 tests](PLAN_STAGE_1_TESTS.md) · [2 types](PLAN_STAGE_2_TYPES.md) · [3 handlers](PLAN_STAGE_3_HANDLERS.md) · [4 mount](PLAN_STAGE_4_MOUNT.md) · [6 operation identity](PLAN_STAGE_6_OP.md) · [5 docs](PLAN_STAGE_5_DOCS.md) (run 6 before 5).

## Goal

A domain handler no longer persists or publishes. The flow of a write becomes:

1. `crud` validates (schemaFields, `ServerOnly`, limits, RBAC) — unchanged.
2. If the model declares the action `model.AddMyBusinessRule`, the handler's rule validates its business and answers `(crud.RuleAnswer, error)`.
   If it declares `model.HaveNoBusinessRule`, there is no rule: plain storage.
3. `crud` stores inside the transaction, commits, and publishes the change **only after the commit**, and only if the rule answered
   `crud.ApplyAndPublish` (a `HaveNoBusinessRule` action always announces). `crud.ApplyWithoutPublishing` stores without publishing.

Four more changes ride along (one breaking version, never two): `crud.Op` (the string operation name) is deleted and operations are addressed by `router.Operation` (stage 6, REST on the wire is the transports' job); the others: `crud.Caller` is renamed `crud.CurrentUser` (it clashes with `router.Caller`, the client that calls operations);
`StoredEntity.Committed` is deleted; `crud.Module` gains `Definition() model.ModuleDefinition` and `crud.Mount` refuses to serve when what the
module mounted differs from what its `Definition` lists (the user forgot to run `definitionc`).

## Ecosystem search

Searched `RuleAnswer`, `ApplyAndPublish`, `Publisher`, `Committed`, `Definition()` over `webtyp/*` and `veltylabs/*`: `events.Publisher` already
is the publishing contract (reuse, `crud` imports `webtyp.com/events`); `StoredEntity.Committed` is the only publishing path today and is deleted;
`model.Rules`/`model.ModuleDefinition` come from the `model` plan (reuse). New in this repo: `RuleAnswer`, `Change`, `Topic`. No new library.

## Design gate

1. **Prior art** (unverified in this session): Rails `after_commit` callbacks and Django `transaction.on_commit` — an event is announced
   only after the transaction commits; the transactional-outbox write-ups ([summary](https://milanjovanovic.tech/blog/outbox-pattern-dual-write-problem))
   explain why publishing inside the transaction announces data that may roll back. Difference here: the handler only decides, `crud` publishes.
2. **Novice-name test**: `crud.ApplyAndPublish` = "save and announce"; `crud.ApplyWithoutPublishing` = "save without announcing"; `crud.CurrentUser` = who acts.
3. **Ledger**: concepts +3 (`RuleAnswer`, `Change`, `Topic`) −2 (`Committed`, `Caller`); lines at a module call site −12 (no publish code);
   ways to publish an event: 1 → 1 (the hand-written `publish` is deleted in the module plans).
4. **Where**: `crud` owns the write flow, so it owns when to publish; `events` stays the contract; the module owns only its rule.
5. **Deletes**: `Caller`, `StoredEntity.Committed`, the `Committed` calls in the three write handlers.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

The plans of `model` (v0.2.17), `router` (v0.4.20) and `definitioncheck` (v0.0.2) are published. Dependencies (no `replace`):

```bash
go get webtyp.com/model@v0.2.17
go get webtyp.com/router@v0.4.20
go get webtyp.com/definitioncheck@v0.0.2   # the judge of Rules/Allow
go get webtyp.com/events@v0.1.8
go mod tidy
```

## APIs used

| symbol | exact signature | tag | file |
|---|---|---|---|
| `events.Publisher` | `type Publisher interface { Publish(e Event) }` | events v0.1.8 | `events/events.go:43` |
| `events.Event` | `struct { Topic string; Payload model.Encodable }` | events v0.1.8 | `events/events.go` |
| `model.Encodable` | `interface { IsNil() bool; EncodeFields(w FieldWriter) }` | model v0.2.17 | `model/codec.go` |
| `model.FieldWriter` | `Object(name string, val Encodable)`, `Array(name string, n int) ArrayWriter` | model v0.2.17 | `model/codec.go:6` |
| `model.ArrayWriter` | `String(val string)` | model v0.2.17 | `model/codec.go:22` |
| `model.Rules`, `model.RuleMode` | `struct{Create, Read, Update, Delete RuleMode}`; `AddMyBusinessRule`, `HaveNoBusinessRule`; `func (r Rules) Of(a Action) RuleMode` | model v0.2.17 (model plan) | `model/rules.go` |
| `definitioncheck.CheckRules` | `func CheckRules(def model.Definition) error` | definitioncheck (its plan) | `definitioncheck/rules.go` |
| `definitioncheck.CheckModule` | `func CheckModule(m model.ModuleDefinition) error` | definitioncheck (its plan) | `definitioncheck/module.go` |
| `model.ModuleDefinition` | `struct{ Name string; Entities []Definition }` (no methods: `definitioncheck.CheckModule` judges it) | model v0.2.17 | `model/module_definition.go` |

If a signature differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stages

| Stage | Files | Commit |
|---|---|---|
| 0 | the helpers of stage 0 in `table*.go`, `loopback.go`, `mount.go`, `create.go`, `validate.go` (names only) | `refactor: literal names for crud's internal helpers` |
| 1 | `tests/*` (rewrite hooks, add the new tests, red) | `test: results, events, actor, definition check (red)` |
| 2 | `result.go`, `change.go`, `topic.go`, `errors.go`, `contract.go` (CurrentUser) | `feat: RuleAnswer, Change, Topic; Caller becomes CurrentUser` |
| 3 | `stored_entity.go`, `stored_entity_create.go`, `stored_entity_update.go`, `stored_entity_delete.go` | `refactor!: the rule answers a RuleAnswer; crud publishes after commit` |
| 4 | `mount.go`, `contract.go` (Module), `loopback.go` | `refactor!: Mount checks Definition against what was mounted` |
| 6 | `op.go`, `stored_entity.go`, `read.go`, `loopback.go`, `create.go`, `update.go`, `delete.go`, `tests/*` | `refactor!: router.Operation replaces crud.Op` |
| 5 | `lang.json`, `README.md`, `docs/ARCHITECTURE.md`, `docs/TESTS.md`, `docs/diagrams/WRITE_FLOW.md` | `docs: write flow` |

## Delivery check

1. `gotest` (builds, vets, runs; a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `langc sync .` → zero untranslated keys, zero reuse warnings.
3. `grep -rn "Caller\b" --include=*.go . | grep -v "router.Caller"` → empty; `grep -rn "Committed" --include=*.go .` → empty;
   `grep -rn "callerOf" --include=*.go .` → empty.
4. `git diff --stat main` lists only files named in the stages; nothing else deleted.
5. Consumers of this repo (`rpc`, `mcp`, `dbsync`, `view`, `appkit`, `layout`, `orm`, `definitionc`) stop compiling against this change; their own plans
   adapt them (`crud.Mount` gains a parameter, `Caller` becomes `CurrentUser`, `Committed` disappears). Do not touch them here.
