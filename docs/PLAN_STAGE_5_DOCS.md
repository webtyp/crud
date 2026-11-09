← [Stage 4](PLAN_STAGE_4_MOUNT.md) | Next → end

# Stage 5 — translations and documentation

## 5.1 Translations

Run `langc sync .`. The new constants (`ErrNoAnswer`, `ErrGeneratedOutOfDate`, the four `what…` strings) may be listed as new keys: fill every empty value for all the
languages in `lang.json` the same way the existing keys are filled (look at the row of an existing `mountError` such as `crud: Table without a Store…`). Delivery requires
**zero untranslated keys and zero reuse warnings**; if a reuse warning names a repeated phrase, split it at the call site so shared words are passed as data.
Commit: `chore: translations`.

## 5.2 Documentation (mandatory templates in AGENTS.md)

- `docs/diagrams/WRITE_FLOW.md` — three parts: a context header (what the flow is and who reads it); the `flowchart TD` written in stage 1; a numbered walkthrough, one
  item per node: 1 validates, 2 AddMyBusinessRule/HaveNoBusinessRule, 3 the rule's `RuleAnswer`, 4 zero-result refusal, 5 store, 6 commit, 7 announce-or-silent, 8 publish `Topic` with `*Change`.
- `README.md` (≤ 150 lines, the four ordered sections): *Why This Exists* — Without: each module wrote `publish` and called the store itself and could forget either;
  With: the rule only answers `crud.ApplyAndPublish`; *Step-by-Step Guide* — a compilable example of a `StoredEntity` with a `AddMyBusinessRule` `Create` rule and a `HaveNoBusinessRule` `Update`,
  mounted with `crud.Mount(reg, nil, broker, mod)`, and a subscriber using `crud.Topic` and `*crud.Change`; *Documentation Map* links `WRITE_FLOW.md`.
- `docs/ARCHITECTURE.md` — four sections; add: where events are published (after the commit, never inside), the `CurrentUser` rename, `Module.Definition` and the out-of-date check.
- `docs/TESTS.md` — remove the rows of the two deleted tests, add one row per new test of stage 1 (same format as existing rows).
- Do not link `docs/PLAN*.md` from permanent docs. Delete `docs/LAST_PLAN_EXECUTED.md` only if the delivery check lists it as obsolete; otherwise leave it.

Commit: `docs: write flow`. Then run the delivery check of [PLAN.md](PLAN.md).
- `docs/diagrams/PLACEMENT.md` already exists (placement of `crud` in the framework, skill **documentation**): once `crud` imports `definitioncheck`, change its edge from «imports (planned)» to «imports», and link the diagram from `docs/ARCHITECTURE.md` section 1.
