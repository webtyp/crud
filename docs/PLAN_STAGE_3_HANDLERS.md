← [Stage 2](PLAN_STAGE_2_TYPES.md) | Next → [Stage 4](PLAN_STAGE_4_MOUNT.md)

# Stage 3 — the StoredEntity checks its rules, the handlers answer a RuleAnswer and publish after the commit

## 3.1 `contract.go` — `StoredEntity` (replace the struct; keep `Public`, `Route`, `Registry`, `CurrentUser` as they are)

```go
// StoredEntity is one entity a module offers and crud stores through a Store. definitionc builds it from the model's Rules and the module's rules; a hook is nil
// exactly when its action is declared model.HaveNoBusinessRule (plain storage) — StoredEntity checks this at mount.
type StoredEntity struct {
	Model   model.Model // a zero record; must implement model.Allower with Allow() != 0
	NewList func() model.ModelSlice
	Store   Store
	// Rules is the model's declared Rules (PatientModel.Rules). A AddMyBusinessRule action needs its hook, a HaveNoBusinessRule action must have none.
	Rules model.Rules
	// Create runs per record, before the insert, inside the transaction. It answers whether to announce (crud.ApplyAndPublish or crud.ApplyWithoutPublishing).
	Create func(a CurrentUser, tx Tx, rec model.Model) (RuleAnswer, error)
	// Update runs per id; old is the stored row and next carries only fields.
	Update func(a CurrentUser, tx Tx, old, next model.Model, fields []string) (RuleAnswer, error)
	// Delete runs per id, before the delete; rec is the stored row.
	Delete func(a CurrentUser, tx Tx, rec model.Model) (RuleAnswer, error)
	// Search replaces the default search: AND-only conditions. Declared with Rules.Read == model.AddMyBusinessRule.
	Search    func(a CurrentUser, text string) ([]storage.Condition, error)
	MFA       model.Action       // actions that require a second factor
}
```

`Committed` disappears from the struct.

**Rename in the same stage (decision D47: `Table` was not literal; it is an entity `crud` stores, SQL or not):**
`git mv table.go stored_entity.go`, `table_create.go stored_entity_create.go`, `table_update.go stored_entity_update.go`, `table_delete.go stored_entity_delete.go`, `table_read.go stored_entity_read.go`, `tests/table_test.go tests/stored_entity_test.go`;
`type Table` → `type StoredEntity`; in `Registry` (`contract.go`) `Table(t Table)` → `AddStoredEntity(entity StoredEntity)` and `Read(m model.ModuleNaming, newFilter …, read …) Route` → `AddReadOnlyEntity(m model.ModuleNaming, newFilter …, read …) Route` (same parameters, name only);
delete the field `NewFilter` of the struct (decision D42: nothing uses it) and its two branches in `stored_entity_read.go`.
The three mount errors say the new name: `ErrNothingAllowed` = `"crud: StoredEntity for a model that allows nothing; set Allow in its model.Definition"`, `ErrNoStore` = `"crud: StoredEntity without a Store; pass orm.TenantStore(db, tenantID, ids)"`, `ErrNoPrimaryKey` = `"crud: StoredEntity model has no primary key field"`; `ErrNoSearchField` = `"crud: the entity has no searchable field"` and `ErrAfterCompositeKey` says "list the entity whole" instead of "list the table whole"; their `lang.json` rows change key text, so stage 5 re-translates them.
`grep -rn "crud\.Table\|reg\.Table(\|reg\.Read(" --include=*.go .` → empty.

## 3.2 `errors.go` — add the out-of-date error

```go
const ErrGeneratedOutOfDate mountError = "crud: the generated code does not match definition.go, so nothing is served; run definitionc and rebuild"

const (
	whatMissing = "is listed in definition.go but was not mounted"
	whatExtra   = "was mounted but is not listed in definition.go"
	whatRules   = "has rules in definition.go that differ from the generated hooks"
	whatName    = "has a Definition name that differs from its ModelName"
)

// outOfDateError is ErrGeneratedOutOfDate with where: the module, the entity and, for rules, the actions as CRUD letters.
type outOfDateError struct {
	module  string
	entity  string
	what    string
	actions model.Action
}

func (e *outOfDateError) Error() string {
	text := string(ErrGeneratedOutOfDate) + " — " + e.module + "." + e.entity + " " + e.what
	if e.actions != 0 {
		text += " (" + e.actions.String() + ")"
	}
	return text
}
```

## 3.3 `stored_entity.go` — checks at mount and the eventPublisher

In `func (r *operationMounter) AddStoredEntity(entity StoredEntity)` insert, **after** the `ErrHookWithoutAction` checks and **before** `schemaFields := entity.Model.Schema()`:

```go
	def := model.Definition{Name: entity.Model.ModelName(), Allow: allow, Rules: entity.Rules}
	if err := definitioncheck.CheckRules(def); err != nil {
		r.failMount(err)
		return
	}
	if bad := mismatchedHookActions(entity); bad != 0 {
		r.failMount(&outOfDateError{module: r.module, entity: def.Name, what: whatRules, actions: bad})
		return
	}
	r.mountedEntities = append(r.mountedEntities, def.Name)
	publisher := eventPublisher{pub: r.pub, module: r.module, entity: def.Name}
```

Add to `stored_entity.go`:

```go
// mismatchedHookActions returns the actions whose declared rule and connected hook disagree: AddMyBusinessRule without its hook, or a hook for a
// HaveNoBusinessRule action. Search is the Read hook.
func mismatchedHookActions(entity StoredEntity) model.Action {
	var bad model.Action
	check := func(a model.Action, hooked bool) {
		if (entity.Rules.Of(a) == model.AddMyBusinessRule) != hooked {
			bad |= a
		}
	}
	check(model.Create, entity.Create != nil)
	check(model.Update, entity.Update != nil)
	check(model.Delete, entity.Delete != nil)
	check(model.Read, entity.Search != nil)
	return bad
}
```

and change the three handler constructors at the end of `AddStoredEntity`:

```go
	mountAction(model.Create, createRecordsHandler(entity, publisher, schemaFields, keyFields, createdAtField, updatedAtField))
	mountAction(model.Update, updateRecordsHandler(entity, publisher, schemaFields, keyFields, updatedAtField))
	mountAction(model.Delete, deleteRecordsHandler(entity, publisher, keyFields))
	mountAction(model.Read, readRecordsHandler(entity, schemaFields))
```

`operationMounter` (the type `registry` in `mount.go`, renamed in stage 0) gains the fields used above; add them now so this stage compiles (stage 4 fills them from `Mount`):

```go
type operationMounter struct {
	reg     router.OperationRegistry
	public  []Public
	pub     events.Publisher // nil: nobody listens
	module  string           // the module's ModelName()
	mountedEntities []string         // every entity mounted by AddStoredEntity or AddReadOnlyEntity, in order
	err     error
}
```

(import `webtyp.com/events` in `mount.go`).

## 3.4 `stored_entity_create.go` — replace the whole file

The body of the transaction loop is the previous one with the hook and the publishing changed; unchanged parts are copied verbatim.

```go
package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

func createRecordsHandler(entity StoredEntity, publisher eventPublisher, schemaFields []model.Field, keyFields []int, createdAtField, updatedAtField int) func(ctx router.Context) {
	return func(ctx router.Context) {
		request := createRequest{list: entity.NewList()}
		if err := ctx.Decode(&request); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateCreateBatch(schemaFields, request.list, request.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if request.list.Len() > 1 {
			if _, ok := entity.Store.(Transactional); !ok {
				router.WriteError(ctx, ErrBatchNotAtomic)
				return
			}
		}

		currentUser := currentUserOf(ctx)
		alreadyStored := make([]bool, request.list.Len())
		ruleAnswers := make([]RuleAnswer, request.list.Len())
		now := int64(time.NowNano())

		err := inTransaction(entity.Store, func(tx Tx) error {
			for i := 0; i < request.list.Len(); i++ {
				rec := request.list.At(i).(model.Model)

				if keyHasSeparator(rec, keyFields) {
					return ErrKeyHasSeparator
				}

				id := keyValueOf(rec, keyFields)

				if id == "" {
					if len(keyFields) > 1 {
						return &model.FieldError{Field: schemaFields[keyFields[0]].Name, Err: model.Invalid("required")}
					}
					setTextField(rec, keyFields[0], entity.Store.NewID())
				} else {
					stored := entity.NewList().Append().(model.Model)
					if err := tx.ReadOne(stored, id); err == nil {
						if err := tx.ReadOne(rec, id); err != nil {
							return err
						}
						alreadyStored[i] = true
						continue
					} else if model.KindOf(err) != model.ErrorNotFound {
						return err
					}
				}

				if createdAtField >= 0 {
					setIntField(rec, createdAtField, now)
				}
				if updatedAtField >= 0 {
					setIntField(rec, updatedAtField, now)
				}

				answer := ApplyAndPublish // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Create != nil {
					res, err := entity.Create(currentUser, tx, rec)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				if err := tx.Create(rec); err != nil {
					return err
				}
				ruleAnswers[i] = answer
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		for i := 0; i < request.list.Len(); i++ {
			if !alreadyStored[i] && ruleAnswers[i] == ApplyAndPublish {
				publisher.publish(model.Create, &Change{Record: request.list.At(i).(model.Model)})
			}
		}

		last := request.list.At(request.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
}
```

## 3.5 `stored_entity_update.go` — replace the whole file

One change beyond the hook: events carry **a copy of the record per id** (the old code reused `request.Record` for every id, so a publisher that keeps the pointer would see the last id for all events).

```go
package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

func updateRecordsHandler(entity StoredEntity, publisher eventPublisher, schemaFields []model.Field, keyFields []int, updatedAtField int) func(ctx router.Context) {
	return func(ctx router.Context) {
		request := UpdateArgs{Record: entity.NewList().Append().(model.Model)}
		if err := ctx.Decode(&request); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(request.IDs, request.hasEmpty, request.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}
		if err := validateUpdateFields(schemaFields, request.Fields, request.Record); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if len(request.IDs) > 1 {
			if _, ok := entity.Store.(Transactional); !ok {
				router.WriteError(ctx, ErrBatchNotAtomic)
				return
			}
		}

		for _, id := range request.IDs {
			if fmt.Contains(id, model.KeySeparator) {
				if len(keyFields) == 1 {
					router.WriteError(ctx, ErrKeyHasSeparator)
					return
				}
			}
		}

		currentUser := currentUserOf(ctx)
		fields := request.Fields
		if updatedAtField >= 0 {
			hasUpdated := false
			for _, f := range fields {
				if f == FieldUpdatedAt {
					hasUpdated = true
					break
				}
			}
			if !hasUpdated {
				fields = append(fields, FieldUpdatedAt)
			}
		}

		ruleAnswers := make([]RuleAnswer, len(request.IDs))
		err := inTransaction(entity.Store, func(tx Tx) error {
			for i, id := range request.IDs {
				old := entity.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				next := request.Record
				if err := setKeyValue(next, keyFields, id); err != nil {
					return err
				}
				if updatedAtField >= 0 {
					setIntField(next, updated, int64(time.NowNano()))
				}
				answer := ApplyAndPublish // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Update != nil {
					res, err := entity.Update(currentUser, tx, old, next, request.Fields)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				if err := tx.UpdateFields(next, id, fields); err != nil {
					return err
				}
				ruleAnswers[i] = answer
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		for i, id := range request.IDs {
			if ruleAnswers[i] != ApplyAndPublish {
				continue
			}
			rec := entity.NewList().Append().(model.Model)
			copyRecord(request.Record, rec)
			if err := setKeyValue(rec, keyFields, id); err != nil {
				router.WriteError(ctx, err)
				return
			}
			publisher.publish(model.Update, &Change{Record: rec, Fields: request.Fields})
		}

		ctx.WriteStatus(statusNoContent)
	}
}
```

## 3.6 `stored_entity_delete.go` — replace the whole file

```go
package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

func deleteRecordsHandler(entity StoredEntity, publisher eventPublisher, keyFields []int) func(ctx router.Context) {
	return func(ctx router.Context) {
		var in DeleteArgs
		if err := ctx.Decode(&request); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(request.IDs, request.hasEmpty, request.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if len(request.IDs) > 1 {
			if _, ok := entity.Store.(Transactional); !ok {
				router.WriteError(ctx, ErrBatchNotAtomic)
				return
			}
		}

		for _, id := range request.IDs {
			if fmt.Contains(id, model.KeySeparator) {
				if len(keyFields) == 1 {
					router.WriteError(ctx, ErrKeyHasSeparator)
					return
				}
			}
		}

		currentUser := currentUserOf(ctx)
		var deleted []model.Model
		var answers []RuleAnswer

		err := inTransaction(entity.Store, func(tx Tx) error {
			deleted = deleted[:0]
			ruleAnswers = ruleAnswers[:0]
			for _, id := range request.IDs {
				old := entity.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				answer := ApplyAndPublish // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Delete != nil {
					res, err := entity.Delete(currentUser, tx, old)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				if err := tx.Delete(old, id); err != nil {
					return err
				}
				deleted = append(deleted, old)
				ruleAnswers = append(ruleAnswers, answer)
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		for i, row := range deleted {
			if ruleAnswers[i] == ApplyAndPublish {
				publisher.publish(model.Delete, &Change{Record: row})
			}
		}

		ctx.WriteStatus(statusNoContent)
	}
}
```

`readRecordsHandler` changes only the type name of the `Search` hook's first argument (`caller := currentUserOf(ctx)` → `currentUser := currentUserOf(ctx)` and `entity.Search(currentUser, args.Search)`). Nothing else in `stored_entity_read.go`.

Run `gotest`: the packages compile; tests of stage 1 still fail on `Mount` (stage 4). Commit: `refactor!: the rule answers a RuleAnswer; crud publishes after commit`.
