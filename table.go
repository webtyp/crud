package crud

import (
	"strings"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

const (
	ErrNothingAllowed mountError = "crud: Table for a model that allows nothing; set Allow in its model.Definition"
	ErrNoStore        mountError = "crud: Table without a Store; pass orm.TenantStore(db, tenantID, ids)"
	ErrHookWithoutAction mountError = "crud: a rule is set for an action the model does not allow"
	ErrNoPrimaryKey   mountError = "crud: Table model has no primary key field"
	ErrStampNotInt    mountError = "crud: created_at/updated_at must be an int64 nanosecond field"
	ErrKeyKind        mountError = "crud: a primary key field must be text or int64"
	ErrKeyHasSeparator model.Invalid = "crud: a key value cannot contain /"
	ErrBatchNotAtomic  model.Invalid = "crud: the resource saves one record at a time; send one record per request"
	ErrOrCondition     model.Invalid = "crud: a search returned an OR condition; conditions are not grouped, so an OR could read other tenants' rows — return AND conditions only"
	ErrNoSearchField   model.Invalid = "crud: the table has no searchable field"

	//lang:ignore
	ErrAfter = "crud: After pages "
	ErrAfterWithOrder  model.Invalid = ErrAfter + "by primary key; it cannot be combined with another Order"
	ErrAfterCompositeKey model.Invalid = ErrAfter + "by a single-column key; list the table whole (its screen uses view.Page{First: view.All})"

	FieldCreatedAt = "created_at"
	FieldUpdatedAt = "updated_at"
)

func (r *registry) Table(t Table) {
	if t.Model == nil || t.NewList == nil {
		r.fail(ErrNilFunction)
		return
	}
	allower, ok := t.Model.(model.Allower)
	if !ok || allower.Allow() == 0 {
		r.fail(ErrNothingAllowed)
		return
	}
	allow := allower.Allow()
	if t.Store == nil {
		r.fail(ErrNoStore)
		return
	}
	if t.Create != nil && !allow.Has(model.Create) {
		r.fail(ErrHookWithoutAction)
		return
	}
	if t.Update != nil && !allow.Has(model.Update) {
		r.fail(ErrHookWithoutAction)
		return
	}
	if t.Delete != nil && !allow.Has(model.Delete) {
		r.fail(ErrHookWithoutAction)
		return
	}
	if (t.Search != nil || t.NewFilter != nil) && !allow.Has(model.Read) {
		r.fail(ErrHookWithoutAction)
		return
	}

	schema := t.Model.Schema()
	pk := pkIndex(schema)
	if len(pk) == 0 {
		r.fail(ErrNoPrimaryKey)
		return
	}

	ptrs := t.Model.Pointers()
	for _, i := range pk {
		switch ptrs[i].(type) {
		case *string, *int64:
		default:
			r.fail(ErrKeyKind)
			return
		}
	}

	if checker, ok := t.Store.(ModelChecker); ok {
		if err := checker.CheckModel(t.Model); err != nil {
			r.fail(err)
			return
		}
	}

	createdIdx := stampIndex(schema, FieldCreatedAt)
	if createdIdx >= 0 {
		if _, ok := ptrs[createdIdx].(*int64); !ok {
			r.fail(ErrStampNotInt)
			return
		}
	}
	updatedIdx := stampIndex(schema, FieldUpdatedAt)
	if updatedIdx >= 0 {
		if _, ok := ptrs[updatedIdx].(*int64); !ok {
			r.fail(ErrStampNotInt)
			return
		}
	}

	modelName := t.Model.ModelName()

	mountAction := func(action model.Action, h router.HandlerFunc) {
		if !allow.Has(action) {
			return
		}
		op := r.reg.Operation(Op(t.Model, action), h)
		if r.isPublic(modelName, action) {
			op.Public()
		} else {
			op.Requires(model.Resource(modelName), action)
		}
		if action == model.Create || action == model.Update {
			op.Accepts(t.Model)
		}
		if t.MFA.Has(action) {
			op.RequiresMFA()
		}

		var actionStr string
		switch action {
		case model.Create: actionStr = "Create"
		case model.Read: actionStr = "Read"
		case model.Update: actionStr = "Update"
		case model.Delete: actionStr = "Delete"
		}
		op.Describe(actionStr + " " + modelName)
	}

	mountAction(model.Create, createHandler(t, schema, pk, createdIdx, updatedIdx))
	mountAction(model.Update, updateHandler(t, schema, pk, updatedIdx))
	mountAction(model.Delete, deleteHandler(t, pk))
	mountAction(model.Read, readHandler(t, schema))
}

func pkIndex(schema []model.Field) []int {
	var idxs []int
	for i, f := range schema {
		if f.IsPK() {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

func stampIndex(schema []model.Field, name string) int {
	for i, f := range schema {
		if f.Name == name {
			return i
		}
	}
	return -1
}

func setString(rec model.Model, i int, v string) {
	*rec.Pointers()[i].(*string) = v
}
func setInt(rec model.Model, i int, v int64) {
	*rec.Pointers()[i].(*int64) = v
}

func atomicStore(s Store, fn func(tx Tx) error) error {
	if t, ok := s.(Transactional); ok {
		return t.Atomic(fn)
	}
	return fn(s)
}

func keyOf(rec model.Model, pk []int) string {
	ptrs := rec.Pointers()
	parts := make([]string, len(pk))
	for i, idx := range pk {
		switch v := ptrs[idx].(type) {
		case *string:
			parts[i] = *v
		case *int64:
			parts[i] = fmt.Convert(*v).String()
		}
	}
	return model.JoinKey(parts...)
}

func setKey(rec model.Model, pk []int, id string) error {
	parts, ok := model.SplitKey(id, len(pk))
	if !ok {
		return ErrInvalidID
	}
	ptrs := rec.Pointers()
	for i, idx := range pk {
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

func hasSeparator(rec model.Model, pk []int) bool {
	ptrs := rec.Pointers()
	for _, idx := range pk {
		if s, ok := ptrs[idx].(*string); ok && strings.Contains(*s, model.KeySeparator) {
			return true
		}
	}
	return false
}
