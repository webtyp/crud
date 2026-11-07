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

// validateCreateBatch checks batch size limits and executes declarative schema validation
// on every record in the batch prior to database persistence.
func validateCreateBatch(list model.ModelSlice, tooMany bool) error {
	if tooMany || list.Len() > MaxPage {
		return ErrBatchTooLarge
	}
	if list.Len() == 0 {
		return ErrNoRecords
	}
	for i := 0; i < list.Len(); i++ {
		elem := list.At(i)
		if v, ok := elem.(model.Validator); ok {
			if err := v.Validate(model.ActionCreate); err != nil {
				return err
			}
		} else if err := model.ValidateFields(model.ActionCreate, elem); err != nil {
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
