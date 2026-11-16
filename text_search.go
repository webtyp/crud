package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// textSearchConditions turns what the person typed into one condition per word on the row's text_search, passed
// through the same function that wrote it (fmt ToSearchText), joined with AND. "" → none.
//
//	textSearchConditions("Lucía 13.047") // text_search LIKE '%lucia%' AND text_search LIKE '%13047%'
func textSearchConditions(text string) []storage.Condition {
	folded := fmt.Convert(text).ToSearchText().String()
	if folded == "" {
		return nil
	}
	words := fmt.Convert(folded).Split()
	conds := make([]storage.Condition, 0, len(words))
	for _, w := range words {
		conds = append(conds, storage.Like(model.FieldTextSearch, "%"+w+"%"))
	}
	return conds
}

// listsOnlyAfterSearch reports whether the entity's model declares ListsOnlyAfterSearch (model.SearchFirst).
func listsOnlyAfterSearch(m model.Model) bool {
	sf, ok := m.(model.SearchFirst)
	return ok && sf.ListsOnlyAfterSearch()
}
