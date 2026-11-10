package crud

import (
	"webtyp.com/model"
)

// ExternalCall represents a call to an external system declared in the entity's Definition.ExternalCalls.
type ExternalCall interface {
	ModelName() string
}

// IdempotencyKey is the stable identifier of a call attempt, matching the queued call id.
type IdempotencyKey string

// FieldsToSave is the list of field names that crud updates on the record after an external call returns.
type FieldsToSave []string

// Fields wraps a list of field names into FieldsToSave.
//
//	return crud.Fields("pay_url", "status"), nil
func Fields(names ...string) FieldsToSave {
	return FieldsToSave(names)
}

// ProviderConfirmation carries verification tokens or payloads sent by a provider webhook or customer return.
//
//	conf := crud.ProviderConfirmation{Token: "tok_123"}
type ProviderConfirmation struct {
	Token   string
	Payload string
	Raw     []byte
}

// QueuedCall represents a call stored in the queue.
type QueuedCall struct {
	Id       string
	TenantId string
	Module   string
	Entity   string
	RecordId string
	Call     string
	Mode     model.ExternalCallMode
}

// CallRequest contains the data needed to store a queued external call.
type CallRequest struct {
	TenantId string
	Module   string
	Entity   string
	RecordId string
	Call     string
	Mode     model.ExternalCallMode
}

// CallQueue is the narrow contract crud needs to queue and settle external calls without importing externalcalls.
type CallQueue interface {
	Add(tx Tx, r CallRequest) (string, error)
	MarkDone(id string) error
	MarkFailedAttempt(id string, call model.ExternalCallDefinition, reason string) error
	ConfirmCall(id string) error
	RejectCall(id, reason string) error
}

// ExternalCallFunc is the function that makes an external call for a record.
type ExternalCallFunc func(a CurrentUser, rec model.Model, key IdempotencyKey) (FieldsToSave, error)

// RuleAnswer is what a rule answers when its business allows the write: store it, say whether to publish the event,
// and optionally request external calls. The zero value is not an answer: a rule that returns it is refused (ErrNoAnswer).
//
//	return crud.ApplyAndPublish(), nil
//	return crud.ApplyAndPublish(PaymentCharge{}), nil
type RuleAnswer struct {
	publish bool
	calls   []ExternalCall
	isValid bool
}

// ApplyAndPublish tells crud to store the change, queue any external calls, and publish the event after the commit.
//
//	return crud.ApplyAndPublish(), nil
//	return crud.ApplyAndPublish(PaymentCharge{}), nil
func ApplyAndPublish(calls ...ExternalCall) RuleAnswer {
	return RuleAnswer{publish: true, calls: calls, isValid: true}
}

// ApplyWithoutPublishing tells crud to store the change, queue any external calls, and publish nothing.
//
//	return crud.ApplyWithoutPublishing(), nil
//	return crud.ApplyWithoutPublishing(PaymentCharge{}), nil
func ApplyWithoutPublishing(calls ...ExternalCall) RuleAnswer {
	return RuleAnswer{publish: false, calls: calls, isValid: true}
}

// valid reports whether the rule really answered (the zero RuleAnswer did not).
func (r RuleAnswer) valid() bool { return r.isValid }

// Calls returns the external calls requested by the rule.
func (r RuleAnswer) Calls() []ExternalCall { return r.calls }

// Publishes reports whether the rule requested publishing the event.
func (r RuleAnswer) Publishes() bool { return r.publish }

// ConfirmCall settles a queued WaitForConfirmation call as done.
//
//	err := crud.ConfirmCall(queue, id)
func ConfirmCall(q CallQueue, id string) error {
	if q == nil {
		return ErrNoCallQueue
	}
	return q.ConfirmCall(id)
}

// RejectCall settles a queued WaitForConfirmation call as failed with the given reason.
//
//	err := crud.RejectCall(queue, id, "payment refused")
func RejectCall(q CallQueue, id, reason string) error {
	if q == nil {
		return ErrNoCallQueue
	}
	return q.RejectCall(id, reason)
}
