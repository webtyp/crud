package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

func updateRecordsHandler(entity StoredEntity, publisher eventPublisher, schemaFields []model.Field, keyFields []int, updatedAtField int) func(ctx router.Context) {
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

		currentUser := currentUserOf(ctx)
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

		ruleAnswers := make([]RuleAnswer, len(request.IDs))
		err := inTransaction(entity.Store, func(tx Tx) error {
			for i, id := range request.IDs {
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
				answer := ApplyAndPublish // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Update != nil {
					res, err := entity.Update(currentUser, tx, old, next, request.Fields)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				if err := tx.UpdateFields(next, id, fields); err != nil {
					return err
				}
				ruleAnswers[i] = answer
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		for i, id := range request.IDs {
			if ruleAnswers[i] != ApplyAndPublish {
				continue
			}
			rec := entity.NewList().Append().(model.Model)
			copyRecord(request.Record, rec)
			if err := setKeyValue(rec, keyFields, id); err != nil {
				router.WriteError(ctx, err)
				return
			}
			publisher.publish(model.Update, &Change{Record: rec, Fields: request.Fields})
		}

		ctx.WriteStatus(statusNoContent)
	}
}
