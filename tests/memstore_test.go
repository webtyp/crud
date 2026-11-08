package tests

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"webtyp.com/crud"
	"webtyp.com/model"
)

type memStore struct {
	mu     sync.Mutex
	rows   map[string]map[string]any
	nextID int
}

func newMemStore() *memStore {
	return &memStore{rows: make(map[string]map[string]any)}
}

func (s *memStore) NewID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return strconv.Itoa(s.nextID)
}

func (s *memStore) ReadOne(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	schema := rec.Schema()
	ptrs := rec.Pointers()
	for i, f := range schema {
		if v, ok := row[f.Name]; ok {
			switch p := ptrs[i].(type) {
			case *string:
				*p = v.(string)
			case *int64:
				*p = v.(int64)
			case *bool:
				*p = v.(bool)
			}
		}
	}
	return nil
}

func (s *memStore) Create(rec model.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schema := rec.Schema()
	ptrs := rec.Pointers()
	row := make(map[string]any)
	var parts []string
	for i, f := range schema {
		if f.IsPK() {
			switch p := ptrs[i].(type) {
			case *string:
				parts = append(parts, *p)
			case *int64:
				parts = append(parts, strconv.FormatInt(*p, 10))
			}
		}
		switch p := ptrs[i].(type) {
		case *string:
			row[f.Name] = *p
		case *int64:
			row[f.Name] = *p
		case *bool:
			row[f.Name] = *p
		}
	}
	id := model.JoinKey(parts...)
	s.rows[id] = row
	return nil
}

func (s *memStore) UpdateFields(rec model.Model, id string, fields []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	schema := rec.Schema()
	ptrs := rec.Pointers()
	for _, fName := range fields {
		for i, f := range schema {
			if f.Name == fName {
				switch p := ptrs[i].(type) {
				case *string:
					row[f.Name] = *p
				case *int64:
					row[f.Name] = *p
				case *bool:
					row[f.Name] = *p
				}
			}
		}
	}
	return nil
}

func (s *memStore) Delete(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
	return nil
}

func (s *memStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	if q.After != "" && len(q.Order) > 0 {
		return crud.ErrAfterWithOrder
	}
	for _, c := range q.Conditions {
		if c.Logic() == "OR" {
			return crud.ErrOrCondition
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var ids []string
	for id := range s.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	count := int64(0)
	for _, id := range ids {
		if q.After != "" && id <= q.After {
			continue
		}
		if q.Limit > 0 && count >= q.Limit {
			break
		}

		rowMatch := true
		for _, c := range q.Conditions {
			v, ok := s.rows[id][c.Field()]
			if !ok {
				rowMatch = false
				break
			}

			val := c.Value()
			if valStr, isStr := val.(string); isStr && strings.Contains(valStr, "%") {
				pat := strings.ToLower(strings.Trim(valStr, "%"))
				if !strings.Contains(strings.ToLower(v.(string)), pat) {
					rowMatch = false
				}
			} else {
				if v != val {
					rowMatch = false
				}
			}
		}
		if !rowMatch {
			continue
		}

		rec := newRow()
		schema := rec.Schema()
		ptrs := rec.Pointers()
		for i, f := range schema {
			if v, ok := s.rows[id][f.Name]; ok {
				switch p := ptrs[i].(type) {
				case *string:
					*p = v.(string)
				case *int64:
					*p = v.(int64)
				case *bool:
					*p = v.(bool)
				}
			}
		}
		onRow(rec)
		count++
	}
	return nil
}

// Transactional
type memTx struct {
	*memStore
	backup map[string]map[string]any
}

func (s *memStore) Atomic(fn func(crud.Tx) error) error {
	s.mu.Lock()
	backup := make(map[string]map[string]any)
	for k, v := range s.rows {
		rowCopy := make(map[string]any)
		for rk, rv := range v {
			rowCopy[rk] = rv
		}
		backup[k] = rowCopy
	}
	s.mu.Unlock()

	tx := &memTx{memStore: s, backup: backup}
	err := fn(tx)
	if err != nil {
		s.mu.Lock()
		s.rows = backup
		s.mu.Unlock()
	}
	return err
}
