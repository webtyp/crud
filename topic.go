package crud

import (
	"webtyp.com/events"
	"webtyp.com/model"
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

func (e eventPublisher) publish(a model.Action, ch *Change) {
	if e.pub == nil {
		return
	}
	e.pub.Publish(events.Event{Topic: Topic(e.module, e.entity, a), Payload: ch})
}
