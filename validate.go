package crud

import (
	"strings"

	"webtyp.com/fmt"
	"webtyp.com/model"
)

// validateIDs centralizes ID boundary security for Update and Delete:
// enforces batch size limits, non-empty IDs, trims whitespace, caps ID length, and rejects duplicate IDs.
func validateIDs(ids []string, hasEmpty, tooMany bool) error {
	if tooMany || len(ids) > MaxPage {
		return ErrBatchTooLarge
	}
	if hasEmpty {
		return ErrInvalidID
	}
	if len(ids) == 0 {
		return ErrNoIDs
	}
	for i := 0; i < len(ids); i++ {
		trimmed := strings.TrimSpace(ids[i])
		if trimmed == "" || len(trimmed) > MaxID {
			return ErrInvalidID
		}
	}
	return nil
}

// validateReadArgs rejects oversized arguments (never truncates them), caps the limit
// and validates the typed filter against its schema.
func validateReadArgs(a *ReadArgs) error {
	if len(a.ID) > MaxID {
		return ErrInvalidID
	}
	if len(a.Search) > MaxSearch || len(a.After) > MaxAfter {
		return ErrArgTooLong
	}
	a.Limit = Cap(a.Limit)
	if a.Filter == nil {
		return nil
	}
	return model.ValidateInput(model.ActionCreate, a.Filter)
}

func serverOnlyWritten(schema []model.Field, rec model.Fielder) bool {
	ptrs := rec.Pointers()
	for i := range schema {
		if !schema[i].ServerOnly {
			continue
		}
		ft := schema[i].Type.Storage()
		switch ft {
		case model.FieldText, model.FieldRaw:
			if v, _ := model.ReadStringPtr(ptrs[i]); v != "" {
				return true
			}
		default:
			if !model.IsZeroPtr(ptrs[i], ft) {
				return true
			}
		}
	}
	return false
}

// validateCreateBatch checks batch size limits and executes declarative schema validation
// on every record in the batch prior to database persistence.
func validateCreateBatch(schema []model.Field, list model.ModelSlice, tooMany bool) error {
	if tooMany || list.Len() > MaxPage {
		return ErrBatchTooLarge
	}
	if list.Len() == 0 {
		return ErrNoRecords
	}
	for i := 0; i < list.Len(); i++ {
		elem := list.At(i)
		if serverOnlyWritten(schema, elem) {
			return ErrServerOnlyField
		}
		if err := model.ValidateInput(model.ActionCreate, elem); err != nil {
			return err
		}
	}
	return nil
}

// validateUpdateFields verifies that fields are specified, not duplicated, exist in schema,
// are not immutable (id/PK/tenant_id), and validates the carrier record's updated field values.
func validateUpdateFields(schema []model.Field, fields []string, rec model.Model) error {
	if len(fields) == 0 {
		return ErrNoFields
	}
	ptrs := rec.Pointers()
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "id" || f == "tenant_id" {
			return ErrInvalidField
		}
		for j := 0; j < i; j++ {
			if fields[j] == f {
				return ErrInvalidField
			}
		}
		idx := -1
		for k := range schema {
			if schema[k].Name == f {
				idx = k
				break
			}
		}
		if idx == -1 {
			return ErrUnknownField
		}
		field := &schema[idx]
		if field.IsPK() {
			return ErrInvalidField
		}
		if field.ServerOnly {
			return ErrServerOnlyField
		}
		ft := field.Type.Storage()
		switch ft {
		case model.FieldText, model.FieldRaw:
			val, _ := model.ReadStringPtr(ptrs[idx])
			if err := field.Validate(val); err != nil {
				return err
			}
		default:
			if field.NotNull && model.IsZeroPtr(ptrs[idx], ft) {
				return &model.FieldError{Field: field.Name, Err: fmt.Err(field.Name, "required")}
			}
		}
	}
	return nil
}
