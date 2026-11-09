package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

func createRecordsHandler(entity Table, schemaFields []model.Field, keyFields []int, createdAtField, updatedAtField int) func(ctx router.Context) {
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

		caller := currentUserOf(ctx)
		alreadyStored := make([]bool, request.list.Len())
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

				if createdAtField >= 0 { setIntField(rec, createdAtField, now) }
				if updatedAtField >= 0 { setIntField(rec, updatedAtField, now) }

				if entity.Create != nil {
					if err := entity.Create(caller, tx, rec); err != nil {
						return err
					}
				}
				if err := tx.Create(rec); err != nil {
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
			for i := 0; i < request.list.Len(); i++ {
				if !alreadyStored[i] {
					entity.Committed(caller, model.Create, request.list.At(i).(model.Model), nil)
				}
			}
		}

		last := request.list.At(request.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
}
