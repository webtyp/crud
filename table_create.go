package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

func createHandler(t Table, schema []model.Field, pk []int, created, updated int) func(ctx router.Context) {
	return func(ctx router.Context) {
		in := createIn{list: t.NewList()}
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateCreateBatch(schema, in.list, in.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if in.list.Len() > 1 {
			if _, ok := t.Store.(Transactional); !ok {
				router.WriteError(ctx, ErrBatchNotAtomic)
				return
			}
		}

		caller := callerOf(ctx)
		skipped := make([]bool, in.list.Len())
		now := int64(time.NowNano())

		err := atomicStore(t.Store, func(tx Tx) error {
			for i := 0; i < in.list.Len(); i++ {
				rec := in.list.At(i).(model.Model)

				if hasSeparator(rec, pk) {
					return ErrKeyHasSeparator
				}

				id := keyOf(rec, pk)

				if id == "" {
					if len(pk) > 1 {
						return &model.FieldError{Field: schema[pk[0]].Name, Err: model.Invalid("required")}
					}
					setString(rec, pk[0], t.Store.NewID())
				} else {
					stored := t.NewList().Append().(model.Model)
					if err := tx.ReadOne(stored, id); err == nil {
						if err := tx.ReadOne(rec, id); err != nil {
							return err
						}
						skipped[i] = true
						continue
					} else if model.KindOf(err) != model.ErrorNotFound {
						return err
					}
				}

				if created >= 0 { setInt(rec, created, now) }
				if updated >= 0 { setInt(rec, updated, now) }

				if t.Create != nil {
					if err := t.Create(caller, tx, rec); err != nil {
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

		if t.Committed != nil {
			for i := 0; i < in.list.Len(); i++ {
				if !skipped[i] {
					t.Committed(caller, model.Create, in.list.At(i).(model.Model), nil)
				}
			}
		}

		last := in.list.At(in.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
}
