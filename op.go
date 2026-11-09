package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

const (
	keyRecords = "records"
	keyIDs     = "ids"
	keyFields  = "fields"
	keyRecord  = "record"
	keyFilter  = "filter"
)

// statusNoContent answers an update or delete: the client decodes nothing.
const statusNoContent = 204

func decodeTextList(r model.FieldReader, key string, one *[1]string) (out []string, tooMany bool, hasEmpty bool) {
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
		if fmt.TrimSpace(s) == "" {
			return nil, false, true
		}
		one[0] = s
		return one[:1], false, false
	}
	out = make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := arr.String(i)
		if fmt.TrimSpace(s) == "" {
			hasEmpty = true
		} else {
			out = append(out, s)
		}
	}
	return out, false, hasEmpty
}
