package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"strings"
)

func updateHandler(t Table, schema []model.Field, pk []int, updated int) func(ctx router.Context) {
	return func(ctx router.Context) {
		in := UpdateArgs{Record: t.NewList().Append().(model.Model)}
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(in.IDs, in.hasEmpty, in.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}
		if err := validateUpdateFields(schema, in.Fields, in.Record); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if len(in.IDs) > 1 {
			if _, ok := t.Store.(Transactional); !ok {
				router.WriteError(ctx, ErrBatchNotAtomic)
				return
			}
		}

		for _, id := range in.IDs {
			if strings.Contains(id, model.KeySeparator) {
				if len(pk) == 1 {
					router.WriteError(ctx, ErrKeyHasSeparator)
					return
				}
			}
		}

		caller := callerOf(ctx)
		fields := in.Fields
		if updated >= 0 {
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

		err := atomicStore(t.Store, func(tx Tx) error {
			for _, id := range in.IDs {
				old := t.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				next := in.Record
				if err := setKey(next, pk, id); err != nil {
					return err
				}
				if updated >= 0 {
					setInt(next, updated, int64(time.NowNano()))
				}
				if t.Update != nil {
					if err := t.Update(caller, tx, old, next, in.Fields); err != nil {
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

		if t.Committed != nil {
			for _, id := range in.IDs {
				next := in.Record
				setKey(next, pk, id)
				t.Committed(caller, model.Update, next, in.Fields)
			}
		}

		ctx.WriteStatus(statusNoContent)
	}
}
