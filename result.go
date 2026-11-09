package crud

// RuleAnswer is what a rule answers when its business allows the write: store it, and say whether to publish the event.
// The zero value is not an answer: a rule that returns it is refused (ErrNoAnswer).
//
//	return crud.ApplyAndPublish, nil
type RuleAnswer uint8

const (
	// ApplyAndPublish: crud stores the change and, after the commit, publishes it on Topic(module, entity, action).
	ApplyAndPublish RuleAnswer = iota + 1
	// ApplyWithoutPublishing: crud stores the change and publishes nothing.
	ApplyWithoutPublishing
)

// valid reports whether the rule really answered (the zero RuleAnswer did not).
func (r RuleAnswer) valid() bool { return r == ApplyAndPublish || r == ApplyWithoutPublishing }
