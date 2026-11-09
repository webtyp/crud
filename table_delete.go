package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/fmt"
)

func deleteRecordsHandler(entity Table, keyFields []int) func(ctx router.Context) {
	return func(ctx router.Context) {
		var request DeleteArgs
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

		caller := currentUserOf(ctx)
		var deleted []model.Model

		err := inTransaction(entity.Store, func(tx Tx) error {
			for _, id := range request.IDs {
				old := entity.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				if entity.Delete != nil {
					if err := entity.Delete(caller, tx, old); err != nil {
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

		if entity.Committed != nil {
			for _, row := range deleted {
				entity.Committed(caller, model.Delete, row, nil)
			}
		}

		ctx.WriteStatus(statusNoContent)
	}
}
