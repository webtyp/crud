package crud

import (
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/securityevent"
)

const (
	topicSeparator = "."
	verbCreated    = "created"
	verbUpdated    = "updated"
	verbDeleted    = "deleted"
)

// Topic is the name of the event crud publishes for an action: "<module>.<entity>.created|updated|deleted".
// A read has none (""). Subscribers build the name with this function, never with a string of their own.
//
//	sub.Subscribe(crud.Topic("patient_directory", "patient", model.Create), func(e events.Event) { … })
func Topic(module, entity string, a model.Action) string {
	var verb string
	switch a {
	case model.Create:
		verb = verbCreated
	case model.Update:
		verb = verbUpdated
	case model.Delete:
		verb = verbDeleted
	default:
		return ""
	}
	return module + topicSeparator + entity + topicSeparator + verb
}

// eventPublisher publishes the changes of one entity. A nil publisher means nobody listens (a demo, a test, mcp harvest): not an error.
type eventPublisher struct {
	pub    events.Publisher
	module string
	entity string
}

// reasonInvalidInput is the security reason of a write that broke the declared shape of the data.
const reasonInvalidInput securityevent.Reason = "the request broke the declared shape of the data"

// rejected reports a write that failed the declared validation as a security event (input_validation_fail: one is a
// typo, many from one caller is probing) and answers the error. With no broker nobody listens.
func (e eventPublisher) rejected(ctx router.Context, err error) {
	if e.pub != nil {
		if r, rerr := securityevent.New(e.pub); rerr == nil {
			_ = r.Report(securityevent.Event{Kind: securityevent.InputRejected, UserID: ctx.UserID(), Resource: e.entity,
				IP: ctx.Value(router.ContextKeyRemoteAddr), Reason: reasonInvalidInput})
		}
	}
	router.WriteError(ctx, err)
}

func (e eventPublisher) publish(a model.Action, ch *Change) {
	if e.pub == nil {
		return
	}
	e.pub.Publish(events.Event{Topic: Topic(e.module, e.entity, a), Payload: ch})
}
