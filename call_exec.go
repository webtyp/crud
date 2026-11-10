package crud

import (
	"webtyp.com/model"
	"webtyp.com/time"
)

type pendingReply struct {
	callID string
	def    model.ExternalCallDefinition
	rec    model.Model
	id     string
}

func enqueueCalls(entity StoredEntity, tx Tx, publisher eventPublisher, rec model.Model, id string, schemaFields []model.Field, answer RuleAnswer) ([]pendingReply, error) {
	if len(answer.calls) == 0 {
		return nil, nil
	}
	if entity.Queue == nil {
		return nil, ErrNoCallQueue
	}
	var replies []pendingReply
	tid := tenantIDOf(entity, rec, schemaFields)
	entityName := entity.Model.ModelName()
	for _, call := range answer.calls {
		def, ok := matchCall(entityName, call, entity.ExternalCalls)
		if !ok {
			return nil, ErrUndeclaredCall
		}
		req := CallRequest{
			TenantId: tid,
			Module:   publisher.module,
			Entity:   entityName,
			RecordId: id,
			Call:     def.Name,
			Mode:     def.Mode,
		}
		callID, err := entity.Queue.Add(tx, req)
		if err != nil {
			return nil, err
		}
		if def.Mode == model.WaitForReply {
			replies = append(replies, pendingReply{
				callID: callID,
				def:    def,
				rec:    rec,
				id:     id,
			})
		}
	}
	return replies, nil
}

func executeReplies(entity StoredEntity, user CurrentUser, replies []pendingReply) {
	for _, pr := range replies {
		executeWaitForReply(entity, user, pr.rec, pr.id, pr.callID, pr.def)
	}
}

func executeWaitForReply(entity StoredEntity, user CurrentUser, rec model.Model, recordID, callID string, def model.ExternalCallDefinition) {
	fn := entity.Calls[def.Name]
	if fn == nil {
		_ = entity.Queue.MarkFailedAttempt(callID, def, "no handler registered for call "+def.Name)
		return
	}
	waitMs := int(def.MaxWaitSeconds * 1000)
	if waitMs <= 0 {
		waitMs = 10000
	}

	type callResult struct {
		fields FieldsToSave
		err    error
	}
	done := make(chan callResult, 1)
	go func() {
		fields, err := fn(user, rec, IdempotencyKey(callID))
		done <- callResult{fields: fields, err: err}
	}()

	timedOut := make(chan struct{})
	timer := time.AfterFunc(waitMs, func() {
		close(timedOut)
	})
	defer timer.Stop()

	select {
	case res := <-done:
		if res.err != nil {
			_ = entity.Queue.MarkFailedAttempt(callID, def, res.err.Error())
			return
		}
		if len(res.fields) > 0 {
			if err := entity.Store.UpdateFields(rec, recordID, res.fields); err != nil {
				_ = entity.Queue.MarkFailedAttempt(callID, def, err.Error())
				return
			}
		}
		_ = entity.Queue.MarkDone(callID)
	case <-timedOut:
		_ = entity.Queue.MarkFailedAttempt(callID, def, "timeout")
	}
}

func matchCall(entityName string, call ExternalCall, defs []model.ExternalCallDefinition) (model.ExternalCallDefinition, bool) {
	mn := call.ModelName()
	for _, d := range defs {
		if mn == d.Name || mn == entityName+"."+d.Name {
			return d, true
		}
	}
	return model.ExternalCallDefinition{}, false
}

func tenantIDOf(entity StoredEntity, rec model.Model, schemaFields []model.Field) string {
	if entity.TenantID != nil {
		if tid := entity.TenantID(rec); tid != "" {
			return tid
		}
	}
	ptrs := rec.Pointers()
	for i, f := range schemaFields {
		if f.Name == "tenant_id" {
			if ptr, ok := ptrs[i].(*string); ok {
				return *ptr
			}
		}
	}
	return ""
}
