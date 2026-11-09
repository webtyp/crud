package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/fmt"
)

func updateRecordsHandler(entity Table, schemaFields []model.Field, keyFields []int, updatedAtField int) func(ctx router.Context) {
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

		caller := currentUserOf(ctx)
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

		err := inTransaction(entity.Store, func(tx Tx) error {
			for _, id := range request.IDs {
				old := entity.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				next := request.Record
				if err := setKeyValue(next, keyFields, id); err != nil {
					return err
				}
				if updatedAtField >= 0 {
					setIntField(next, updatedAtField, int64(time.NowNano()))
				}
				if entity.Update != nil {
					if err := entity.Update(caller, tx, old, next, request.Fields); err != nil {
						return err
					}
				}
				if err := tx.UpdateFields(next, id, fields); err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		if entity.Committed != nil {
			for _, id := range request.IDs {
				next := request.Record
				setKeyValue(next, keyFields, id)
				entity.Committed(caller, model.Update, next, request.Fields)
			}
		}

		ctx.WriteStatus(statusNoContent)
	}
}
