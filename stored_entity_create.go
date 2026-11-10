package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
	"webtyp.com/time"
)

func createRecordsHandler(entity StoredEntity, publisher eventPublisher, schemaFields []model.Field, keyFields []int, createdAtField, updatedAtField, revisionField int) func(ctx router.Context) {
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

		currentUser := currentUserOf(ctx)
		alreadyStored := make([]bool, request.list.Len())
		ruleAnswers := make([]RuleAnswer, request.list.Len())
		var allReplies []pendingReply
		now := int64(time.NowNano())

		simple := request.list.Len() == 1 && entity.Create == nil
		err := runWrite(entity.Store, simple, func(tx Tx) error {
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
					id = entity.Store.NewID()
					setTextField(rec, keyFields[0], id)
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

				if revisionField >= 0 {
					setIntField(rec, revisionField, 1)
				}
				if createdAtField >= 0 {
					setIntField(rec, createdAtField, now)
				}
				if updatedAtField >= 0 {
					setIntField(rec, updatedAtField, now)
				}

				answer := ApplyAndPublish() // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Create != nil {
					res, err := entity.Create(currentUser, tx, rec)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				replies, err := enqueueCalls(entity, tx, publisher, rec, id, schemaFields, answer)
				if err != nil {
					return err
				}
				allReplies = append(allReplies, replies...)
				if err := tx.Create(rec); err != nil {
					if storage.IsDuplicate(err) {
						return ErrAlreadyExists
					}
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

		for i := 0; i < request.list.Len(); i++ {
			if !alreadyStored[i] && ruleAnswers[i].publish {
				publisher.publish(model.Create, &Change{Record: request.list.At(i).(model.Model)})
			}
		}
		executeReplies(entity, currentUser, allReplies)

		last := request.list.At(request.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
}
