package crud

import (
	"strings"

	"webtyp.com/model"
	"webtyp.com/router"
)

const (
	keyRecords = "records"
	keyIDs     = "ids"
	keyFields  = "fields"
	keyRecord  = "record"
)

// statusNoContent answers an update or delete: the client decodes nothing.
const statusNoContent = 204

// Op builds the canonical bare operation name for a model and one of the four model.Action verbs
// ("<ModelName>_<action>", e.g. "room_floor_c", "room_floor_r", "room_floor_u", "room_floor_d").
// It uses "_" rather than "." so the qualified operation name "<Module>.<Op>" contains a single dot
// as required by rpc.Caller ("<module>.<op>").
func Op(m model.ModuleNaming, action model.Action) string {
	switch action {
	case model.Create:
		return m.ModelName() + "_c"
	case model.Read:
		return m.ModelName() + "_r"
	case model.Update:
		return m.ModelName() + "_u"
	case model.Delete:
		return m.ModelName() + "_d"
	default:
		return m.ModelName() + "_" + action.String()
	}
}

// mountOp centralizes router registration, operation name resolution, authorization resource wiring,
// and optional arg schema declaration for all CRUD operations.
func mountOp(reg router.OperationRegistry, m model.ModuleNaming, action model.Action, accepts model.Fielder, h router.HandlerFunc) router.Route {
	res := model.Resource(m.ModelName())
	r := reg.Operation(Op(m, action), h).Requires(res, action)
	if accepts != nil {
		r.Accepts(accepts)
	}
	return r
}

func decodeStrings(r model.FieldReader, key string, one *[1]string) (out []string, tooMany bool, hasEmpty bool) {
	arr, ok := r.Array(key)
	if !ok {
		return nil, false, false
	}
	n := arr.Len()
	if n == 0 {
		return nil, false, false
	}
	if n > MaxPage {
		return nil, true, false
	}
	if n == 1 {
		s := arr.String(0)
		if strings.TrimSpace(s) == "" {
			return nil, false, true
		}
		one[0] = s
		return one[:1], false, false
	}
	out = make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := arr.String(i)
		if strings.TrimSpace(s) == "" {
			hasEmpty = true
		} else {
			out = append(out, s)
		}
	}
	return out, false, hasEmpty
}
