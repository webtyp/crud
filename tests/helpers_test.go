package tests

import (
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/router"
)

// declareRules declares every action in allow: AddMyBusinessRule when it is in domain, HaveNoBusinessRule otherwise.
func declareRules(allow, domain model.Action) model.Rules {
	var r model.Rules
	set := func(a model.Action, dst *model.RuleMode) {
		if !allow.Has(a) {
			return
		}
		if domain.Has(a) {
			*dst = model.AddMyBusinessRule
		} else {
			*dst = model.HaveNoBusinessRule
		}
	}
	set(model.Create, &r.Create)
	set(model.Read, &r.Read)
	set(model.Update, &r.Update)
	set(model.Delete, &r.Delete)
	return r
}

// eventRecorder is an events.Publisher that remembers what was published, in order.
type eventRecorder struct{ got []events.Event }

func (r *eventRecorder) Publish(e events.Event) { r.got = append(r.got, e) }

// operationOf is the operation of the test module "tmod" and its entity "t" for one action.
func operationOf(a model.Action) router.Operation {
	return router.Operation{Module: "tmod", Entity: "t", Action: a}
}
