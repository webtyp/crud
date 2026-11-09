package conformance

import (
	"strconv"
	"testing"

	"webtyp.com/crud"
)

// RunBenchmarks measures what a transaction costs next to the database work itself, for any implementation of crud.Store: one insert directly
// and the same insert inside Atomic (what crud does for a write with a rule or a batch). An implementation runs it from its own tests, on a
// real database on a real disk (not a RAM disk, where a commit costs no fsync):
//
//	func BenchmarkStore(b *testing.B) { conformance.RunBenchmarks(b, func(b *testing.B) crud.Store { … }) }
//
// Results are recorded where the numbers are read: docs/PERFORMANCE.md of the repo that runs it.
func RunBenchmarks(b *testing.B, newStore func(b *testing.B) crud.Store) {
	b.Run("Insert_NoTransaction", func(b *testing.B) {
		s := newStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := s.Create(&TestModel{ID: "n" + strconv.Itoa(i)}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Insert_Transaction", func(b *testing.B) {
		s := newStore(b)
		tx, ok := s.(crud.Transactional)
		if !ok {
			b.Skip("the store is not crud.Transactional")
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := "t" + strconv.Itoa(i)
			if err := tx.Atomic(func(t crud.Tx) error { return t.Create(&TestModel{ID: id}) }); err != nil {
				b.Fatal(err)
			}
		}
	})
}
