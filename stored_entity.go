package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/definitioncheck"
	"webtyp.com/router"
)

const (
	ErrNothingAllowed mountError = "crud: StoredEntity for a model that allows nothing; set Allow in its model.Definition"
	ErrNoStore        mountError = "crud: StoredEntity without a Store; pass orm.TenantStore(db, tenantID, ids)"
	ErrHookWithoutAction mountError = "crud: a rule is set for an action the model does not allow"
	ErrNoPrimaryKey   mountError = "crud: StoredEntity model has no primary key field"
	ErrStampNotInt    mountError = "crud: created_at/updated_at must be an int64 nanosecond field"
	ErrKeyKind        mountError = "crud: a primary key field must be text or int64"
	ErrNoConditionalUpdate mountError = "crud: StoredEntity declares a Revision field but its Store cannot update conditionally; use orm.TenantStore or implement crud.ConditionalUpdater"
	ErrKeyHasSeparator model.Invalid = "crud: a key value cannot contain /"
	ErrBatchNotAtomic  model.Invalid = "crud: the resource saves one record at a time; send one record per request"
	ErrOrCondition     model.Invalid = "crud: a search returned an OR condition; conditions are not grouped, so an OR could read other tenants' rows — return AND conditions only"
	ErrNoSearchField   model.Invalid = "crud: the entity has no searchable field"

	//lang:ignore
	ErrAfter = "crud: After pages "
	ErrAfterWithOrder  model.Invalid = ErrAfter + "by primary key; it cannot be combined with another Order"
	ErrAfterCompositeKey model.Invalid = ErrAfter + "by a single-column key; list the entity whole (its screen uses view.Page{First: view.All})"

	FieldCreatedAt = "created_at"
	FieldUpdatedAt = "updated_at"
)

func (r *operationMounter) AddStoredEntity(entity StoredEntity) {
	if entity.Model == nil || entity.NewList == nil {
		r.failMount(ErrNilFunction)
		return
	}
	allower, ok := entity.Model.(model.Allower)
	if !ok || allower.Allow() == 0 {
		r.failMount(ErrNothingAllowed)
		return
	}
	allow := allower.Allow()
	if entity.Store == nil {
		r.failMount(ErrNoStore)
		return
	}
	if entity.Create != nil && !allow.Has(model.Create) {
		r.failMount(ErrHookWithoutAction)
		return
	}
	if entity.Update != nil && !allow.Has(model.Update) {
		r.failMount(ErrHookWithoutAction)
		return
	}
	if entity.Delete != nil && !allow.Has(model.Delete) {
		r.failMount(ErrHookWithoutAction)
		return
	}
	if entity.Search != nil && !allow.Has(model.Read) {
		r.failMount(ErrHookWithoutAction)
		return
	}

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

	schemaFields := entity.Model.Schema()
	keyFields := keyFieldIndexes(schemaFields)
	if len(keyFields) == 0 {
		r.failMount(ErrNoPrimaryKey)
		return
	}

	ptrs := entity.Model.Pointers()
	for _, i := range keyFields {
		switch ptrs[i].(type) {
		case *string, *int64:
		default:
			r.failMount(ErrKeyKind)
			return
		}
	}

	if checker, ok := entity.Store.(ModelChecker); ok {
		if err := checker.CheckModel(entity.Model); err != nil {
			r.failMount(err)
			return
		}
	}

	createdAtField := timestampFieldIndex(schemaFields, FieldCreatedAt)
	if createdAtField >= 0 {
		if _, ok := ptrs[createdAtField].(*int64); !ok {
			r.failMount(ErrStampNotInt)
			return
		}
	}
	updatedAtField := timestampFieldIndex(schemaFields, FieldUpdatedAt)
	if updatedAtField >= 0 {
		if _, ok := ptrs[updatedAtField].(*int64); !ok {
			r.failMount(ErrStampNotInt)
			return
		}
	}

	revisionField := revisionFieldIndex(schemaFields)
	if revisionField >= 0 {
		if _, ok := entity.Store.(ConditionalUpdater); !ok {
			r.failMount(ErrNoConditionalUpdate)
			return
		}
	}

	modelName := entity.Model.ModelName()

	mountAction := func(action model.Action, h router.HandlerFunc) {
		if !allow.Has(action) {
			return
		}
		if r.reg == nil {
			return
		}
		rt := r.reg.Operation(router.Operation{Module: r.module, Entity: modelName, Action: action}, h)
		if r.isPublicAction(modelName, action) {
			rt.Public()
		} else {
			rt.Requires(model.Resource(modelName), action)
		}
		if action == model.Create || action == model.Update {
			rt.Accepts(entity.Model)
		}
		if entity.MFA.Has(action) {
			rt.RequiresMFA()
		}

		var actionStr string
		switch action {
		case model.Create: actionStr = "Create"
		case model.Read: actionStr = "Read"
		case model.Update: actionStr = "Update"
		case model.Delete: actionStr = "Delete"
		}
		rt.Describe(actionStr + " " + modelName)
	}

	mountAction(model.Create, createRecordsHandler(entity, publisher, schemaFields, keyFields, createdAtField, updatedAtField, revisionField))
	mountAction(model.Update, updateRecordsHandler(entity, publisher, schemaFields, keyFields, updatedAtField, revisionField))
	mountAction(model.Delete, deleteRecordsHandler(entity, publisher, keyFields))
	mountAction(model.Read, readRecordsHandler(entity, schemaFields))
}

func revisionFieldIndex(schemaFields []model.Field) int {
	for i, f := range schemaFields {
		if f.IsRevision() {
			return i
		}
	}
	return -1
}

func keyFieldIndexes(schemaFields []model.Field) []int {
	var idxs []int
	for i, f := range schemaFields {
		if f.IsPK() {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

func timestampFieldIndex(schemaFields []model.Field, name string) int {
	for i, f := range schemaFields {
		if f.Name == name {
			return i
		}
	}
	return -1
}

func setTextField(rec model.Model, i int, v string) {
	*rec.Pointers()[i].(*string) = v
}
func setIntField(rec model.Model, i int, v int64) {
	*rec.Pointers()[i].(*int64) = v
}

func inTransaction(s Store, fn func(tx Tx) error) error {
	if t, ok := s.(Transactional); ok {
		return t.Atomic(fn)
	}
	return fn(s)
}

// runWrite runs fn with the store's own Tx when the write is simple (one record, no business rule: a single statement is atomic by itself) and inside a
// transaction otherwise. A rule always gets a transaction: its check and its write must be one unit.
func runWrite(s Store, simple bool, fn func(tx Tx) error) error {
	if simple {
		return fn(s)
	}
	return inTransaction(s, fn)
}

func keyValueOf(rec model.Model, keyFields []int) string {
	ptrs := rec.Pointers()
	parts := make([]string, len(keyFields))
	for i, idx := range keyFields {
		switch v := ptrs[idx].(type) {
		case *string:
			parts[i] = *v
		case *int64:
			parts[i] = fmt.Convert(*v).String()
		}
	}
	return model.JoinKey(parts...)
}

func setKeyValue(rec model.Model, keyFields []int, id string) error {
	parts, ok := model.SplitKey(id, len(keyFields))
	if !ok {
		return ErrInvalidID
	}
	ptrs := rec.Pointers()
	for i, idx := range keyFields {
		switch ptr := ptrs[idx].(type) {
		case *string:
			*ptr = parts[i]
		case *int64:
			v, err := fmt.Convert(parts[i]).Int64()
			if err != nil {
				return ErrInvalidID
			}
			*ptr = v
		}
	}
	return nil
}

func keyHasSeparator(rec model.Model, keyFields []int) bool {
	ptrs := rec.Pointers()
	for _, idx := range keyFields {
		if s, ok := ptrs[idx].(*string); ok && fmt.Contains(*s, model.KeySeparator) {
			return true
		}
	}
	return false
}


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
