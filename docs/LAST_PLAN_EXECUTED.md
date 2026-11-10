---
PLAN: "feat!: orchestrate external calls in crud and switch ApplyAndPublish to functions"
TAG: v0.0.13
EXECUTOR: local
REVIEWER: none
---

# Plan — D58: Orchestrate external calls in crud

## Context & Ecosystem search

1. `webtyp/externalcalls` (`v0.2.0`): Durable queue of calls to external systems (`Queue`, `QueuedCall`, `StateQueued`/`StateDone`/`StateFailed`). Imports `orm` (which imports `crud`). Therefore, `crud` cannot import `externalcalls` (import cycle).
2. `webtyp/model` (`v0.2.20`): `ExternalCallDefinition`, `ExternalCallMode` (`WaitForReply`, `RetryInBackground`, `WaitForConfirmation`), `ValidateExternalCalls()`.
3. `webtyp/backoff` (`v0.2.0`): Exponential retry delay policy without clock dependencies.
4. `webtyp/crud`: Persists entities transactionally, validates schemas and permissions, runs domain business rules.

Decision: `crud` declares its own narrow contract `CallQueue` (`Add`, `MarkDone`, `MarkFailedAttempt`, `ConfirmCall`, `RejectCall`) and `CallRequest`. `crud` orchestrates external calls transactionally alongside the record. `appkit` acts as the composition root adapting `*externalcalls.Queue` to `crud.CallQueue`.

## Design gate

### 1. Prior art
1. **Temporal Activities** ([Temporal Go SDK](https://docs.temporal.io/activities)): Workflows request activity executions declaratively. Activities are retried or waited on with explicit timeouts; the orchestration engine guarantees execution state. Here, domain rules decide external calls and `crud` enqueues them in the database transaction.
2. **Stripe PaymentIntents & Webhooks** ([Stripe API Reference](https://docs.stripe.com/api/payment_intents)): Two-phase external actions: creation returns intent/confirmation required (`requires_action`), and final settlement occurs via secure provider webhook/return that verifies with Stripe servers before confirming. Here, `WaitForConfirmation` holds the call in `queued` state until the provider adapter receives confirmation and the domain rule verifies it.
3. **Transactional Outbox Pattern** ([Microservices.io Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html)): Database record mutation and outgoing intent are committed in the same database transaction. A background worker polls pending items and dispatches them with idempotency keys.

### 2. Novice-name test
- `ApplyAndPublish(calls ...ExternalCall)` / `ApplyWithoutPublishing(calls ...ExternalCall)`:
  Does `ApplyAndPublish(PaymentCharge{})` read as "publicar el cobro" to a novice?
  Analysis: "Apply" refers to applying the entity mutation (persisting the record); "Publish" refers to publishing the domain event (`Topic(...)`).
  Alternative evaluated: `ApplyAndPublish().WithCalls(...)` or `ApplyAndPublishWithCalls(...)`.
  Conclusion: Adding secondary builder methods or separate function variants increases concept count and creates multiple ways to do the same thing. In Go, passing optional calls directly to `ApplyAndPublish(...)` keeps a single, uniform exit point with zero allocations when empty. Read as a sentence: "Apply [this mutation] and publish [the domain event], requesting [these external calls]".
- `ExternalCall`: `interface{ ModelName() string }` — literal; self-describing contract for an external call request.
- `IdempotencyKey`: `type IdempotencyKey string` — standard industry term, stable across all attempts of a call.
- `FieldsToSave`: `type FieldsToSave []string` — literal: names of fields to persist after an external call returns.
- `ProviderConfirmation`: carries the webhook/return proof for `WaitForConfirmation`.
- `ConfirmCall` / `RejectCall`: verbs describing settling a `WaitForConfirmation` call.
- `QueuedCall`: domain struct describing a queued call.

### 3. Complexity ledger
```
Concepts the developer must learn   +1 (ExternalCall on rules) / −1 (uint8 RuleAnswer constant)
Files they must touch to do X       0 (declared in entity Definition, returned by rule)
Lines at the call site              0 (same return statement)
Ways to do the same thing           −1 (constant eliminated, single function form)
```

### 4. Where does it belong
- `crud`: Owns the transaction `tx`, executes the business rule, enqueues the `CallRequest` in the same `tx`, and executes `WaitForReply` inline with timeout.
- `externalcalls`: Owns the queue storage schema, backoff retries, and runner state machine.
- `appkit`: Composition root that builds the queue, runner, and background loop.

### 5. What does this change delete?
- Deletes `RuleAnswer` as `uint8` and the constants `ApplyAndPublish` and `ApplyWithoutPublishing`.
- Deletes manual outbox writing outside transaction.

## Stages

1. **Contract & Result:** Replace `RuleAnswer` uint8 with struct, export `ApplyAndPublish` and `ApplyWithoutPublishing` functions, `ExternalCall`, `IdempotencyKey`, `FieldsToSave`, `ProviderConfirmation`, `CallRequest`, `CallQueue`.
2. **StoredEntity & Mount:** Add `ExternalCalls`, `Calls`, `Queue` to `StoredEntity`. Add `MountWithQueue`, `LoopbackWithQueue`, `LoopbackActingAsWithQueue`.
3. **Transaction & Execution:** In `stored_entity_create.go`, `stored_entity_update.go`, `stored_entity_delete.go`, enqueue calls inside `tx`. Execute `WaitForReply` inline with timeout and settle `MarkDone` / `MarkFailedAttempt`.
4. **End-to-end tests:** Fake module testing scenarios (a) transactional rollback, (b) RetryInBackground, (c) WaitForReply timeout/success, (d) WaitForConfirmation settle, (e) stable idempotency key.
5. **Update existing tests & docs:** Update all test call sites from `crud.ApplyAndPublish` to `crud.ApplyAndPublish()`. Update documentation and diagrams.
