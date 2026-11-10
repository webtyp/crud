package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

func deleteRecordsHandler(entity StoredEntity, publisher eventPublisher, keyFields []int) func(ctx router.Context) {
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

		currentUser := currentUserOf(ctx)
		var deleted []model.Model
		var ruleAnswers []RuleAnswer
		var allReplies []pendingReply

		simple := len(request.IDs) == 1 && entity.Delete == nil
		err := runWrite(entity.Store, simple, func(tx Tx) error {
			deleted = deleted[:0]
			ruleAnswers = ruleAnswers[:0]
			allReplies = allReplies[:0]
			for _, id := range request.IDs {
				old := entity.NewList().Append().(model.Model)
				if err := tx.ReadOne(old, id); err != nil {
					return err
				}
				answer := ApplyAndPublish() // a HaveNoBusinessRule action is plain storage: it always announces
				if entity.Delete != nil {
					res, err := entity.Delete(currentUser, tx, old)
					if err != nil {
						return err
					}
					if !res.valid() {
						return ErrNoAnswer
					}
					answer = res
				}
				replies, err := enqueueCalls(entity, tx, publisher, old, id, old.Schema(), answer)
				if err != nil {
					return err
				}
				allReplies = append(allReplies, replies...)
				if err := tx.Delete(old, id); err != nil {
					return err
				}
				deleted = append(deleted, old)
				ruleAnswers = append(ruleAnswers, answer)
			}
			return nil
		})

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		for i, row := range deleted {
			if ruleAnswers[i].publish {
				publisher.publish(model.Delete, &Change{Record: row})
			}
		}
		executeReplies(entity, currentUser, allReplies)

		ctx.WriteStatus(statusNoContent)
	}
}
