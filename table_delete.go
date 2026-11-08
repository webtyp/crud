package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"strings"
)

func deleteHandler(t Table, pk []int) func(ctx router.Context) {
	return func(ctx router.Context) {
		var in DeleteArgs
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(in.IDs, in.hasEmpty, in.tooMany); err != nil {
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
		var deleted []model.Model

		err := atomicStore(t.Store, func(tx Tx) error {
			for _, id := range in.IDs {
				old := t.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				if t.Delete != nil {
					if err := t.Delete(caller, tx, old); err != nil {
						return err
					}
				}
				if err := tx.Delete(old, id); err != nil {
					return err
				}
				deleted = append(deleted, old)
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		if t.Committed != nil {
			for _, row := range deleted {
				t.Committed(caller, model.Delete, row, nil)
			}
		}

		ctx.WriteStatus(statusNoContent)
	}
}
