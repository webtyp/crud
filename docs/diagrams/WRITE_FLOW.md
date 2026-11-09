# Write Flow: Validation, Rules, Commit, and Post-Commit Publishing

This diagram describes the execution flow for write operations (create, update, delete) in `webtyp/crud`.
It is read by module authors writing business rules and by developers auditing transactional boundaries and event publishing.

```mermaid
flowchart TD
    A["1. crud validates: schemaFields, ServerOnly, limits"] -->|invalid| X["error: nothing stored, nothing published"]
    A --> B{"2. action declared AddMyBusinessRule?"}
    B -->|"HaveNoBusinessRule (no rule)"| D
    B -->|"AddMyBusinessRule"| C["rule(actor, tx, rec) → (RuleAnswer, error)"]
    C -->|"error"| X
    C -->|"zero RuleAnswer"| X2["ErrNoAnswer: rolled back, nothing published"]
    C -->|"ApplyAndPublish / ApplyWithoutPublishing"| D["3. crud stores inside the transaction"]
    D --> E["4. commit"]
    E -->|"commit failed"| X
    E --> F{"5. announce?"}
    F -->|"HaveNoBusinessRule or ApplyAndPublish"| G["publish Topic(module, entity, action) with *Change"]
    F -->|"ApplyWithoutPublishing"| H["stored, nothing published"]
```

## Walkthrough

1. **1. Validates:** `crud` validates the incoming payload against declared `schemaFields`, ensures `ServerOnly` fields are untouched, and checks pagination and ID limits. Any violation returns immediately without touching storage or publishing.
2. **2. AddMyBusinessRule / HaveNoBusinessRule:** Checks whether the entity's model definition declares `AddMyBusinessRule` or `HaveNoBusinessRule` for the action. Plain storage (`HaveNoBusinessRule`) bypasses rule execution.
3. **3. The rule's RuleAnswer:** When `AddMyBusinessRule` is declared, the connected rule executes inside the transaction: `rule(currentUser, tx, rec)` returning `(RuleAnswer, error)`. Any returned error rolls back the batch.
4. **4. Zero-result refusal:** A rule returning a zero `RuleAnswer` without an error is rejected with `ErrNoAnswer`; the transaction rolls back and nothing is published.
5. **5. Store:** `crud` persists records via the configured `Store` inside the active transaction.
6. **6. Commit:** The transaction is committed. If commit fails, the operation errors out and nothing is announced.
7. **7. Announce or silent:** If the action was `HaveNoBusinessRule` or the rule answered `ApplyAndPublish`, an event is queued for delivery. If the rule answered `ApplyWithoutPublishing`, data is stored silently.
8. **8. Publish Topic with *Change:** Only after successful commit, `crud` publishes an `events.Event` to `Topic(module, entity, action)` with payload `*crud.Change{Record: ..., Fields: ...}`.
