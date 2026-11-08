package tests
import (
	"webtyp.com/model"
)

type mockRecord struct {
	ID   string
	Name string
}

func (m *mockRecord) ModelName() string { return "mock" }
func (m *mockRecord) Allow() model.Action { return model.AllActions }
func (m *mockRecord) Pointers() []any   { return []any{&m.ID, &m.Name} }
func (m *mockRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "name", Type: model.Text()},
	}
}
func (m *mockRecord) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok { m.ID = v }
	if v, ok := r.String("name"); ok { m.Name = v }
}
func (m *mockRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}
func (m *mockRecord) IsNil() bool { return m == nil }

type mockRecordList []mockRecord

func (l *mockRecordList) Append() model.Fielder {
	*l = append(*l, mockRecord{})
	return &(*l)[len(*l)-1]
}
func (l *mockRecordList) Len() int                    { return len(*l) }
func (l *mockRecordList) At(i int) model.Fielder      { return &(*l)[i] }
func (l *mockRecordList) IsNil() bool                 { return l == nil }
func (l *mockRecordList) DecodeFields(model.FieldReader) {}
func (l *mockRecordList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
