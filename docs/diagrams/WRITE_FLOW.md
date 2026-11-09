# Creating records: what happens once and what happens on every request

`createRecordsHandler` runs in two moments. At startup (`AddStoredEntity`, **once** per entity) `crud` checks the declaration and builds the function that will answer requests. On **every** `POST /api/<module>/<entity>`
that function runs from the top. The rule you write (`rule_<entity>_create.go`) is called by step 8 only. The branches are verified by `tests/stored_entity_test.go` (stage 1 of the plan).

```mermaid
flowchart TD
    A["ONCE, at startup: AddStoredEntity(entity)<br/>check Allow and Rules agree, check the store and the key exist, read the schema and the key fields"] --> B["ONCE: build createRecordsHandler(entity, events, schemaFields, keyFields, createdAtField, updatedAtField)<br/>it keeps those values and returns the function below"]
    B --> R["EVERY request: POST arrives (already authenticated and RBAC-checked)"]
    R --> C{"1. Decode the body into a list of records"}
    C -- "cannot read" --> E1["answer ErrCannotRead"]
    C --> D{"2. Validate each record against the schema<br/>types, required, ServerOnly, batch size"}
    D -- "invalid" --> E2["answer the validation error"]
    D --> F{"3. More than one record and the store cannot do a transaction?"}
    F -- "yes" --> E3["answer ErrBatchNotAtomic"]
    F -- "no" --> G["4. Open ONE transaction for the whole batch"]
    G --> H["5. For each record: refuse a key containing the separator; mint an id when empty; skip it when the id is already stored (creating twice never duplicates)"]
    H --> I["6. Stamp created_at and updated_at with the current time"]
    I --> J{"7. Does the entity declare AddMyBusinessRule for Create?"}
    J -- "no, HaveNoBusinessRule" --> K["answer = ApplyAndPublish (plain storage always publishes)"]
    J -- "yes" --> L["8. Call YOUR rule: Create(currentUser, tx, record)"]
    L -- "returns an error" --> E4["roll back the whole batch, answer the error"]
    L -- "returns the empty answer" --> E5["roll back, answer ErrNoAnswer"]
    L --> K2["answer = what your rule said: ApplyAndPublish or ApplyWithoutPublishing"]
    K --> M["9. tx.Create(record)"]
    K2 --> M
    M --> N["10. Commit the transaction"]
    N --> O{"11. For each stored record: did the answer say ApplyAndPublish?"}
    O -- "yes" --> P["publish <module>.<entity>.created with *Change, only now that the data is safe"]
    O -- "no, or it was skipped" --> Q["publish nothing"]
    P --> S["12. Answer with the last record created"]
    Q --> S
```

1. Steps A and B happen **once**; the closure keeps `entity`, `events` and the field positions so no request recomputes them.
2. Steps 1 to 3 reject bad input before any database work; nothing is stored when they fail.
3. Steps 4 to 10 are one transaction: if any record fails, none of the batch is stored.
4. Only step 8 is your code; everything else is `crud`, written once for every entity.
5. Events leave at step 11, after the commit, so a rolled-back batch never announces anything.
6. The same shape, with its own differences, serves update (`updateRecordsHandler`: only the sent fields change, `old` and `next` reach the rule) and delete (`deleteRecordsHandler`: the stored row reaches the rule).
