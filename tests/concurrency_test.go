package tests

import (
	"strconv"
	"sync"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

// TestConcurrent_MixedRequestsShareNoState runs create, read, update and delete from many goroutines against ONE mounted module. Run with -race:
// it proves crud keeps no per-request state in what the requests share (the mounter, the handlers' captured values, the loopback caller).
func TestConcurrent_MixedRequestsShareNoState(t *testing.T) {
	const workers, rounds = 32, 100
	mod, rec := benchModule(true)
	safe := &lockedPublisher{inner: rec}
	caller := crud.LoopbackActingAs(safe, "u1", mod)

	var wg sync.WaitGroup
	failures := make(chan error, workers*rounds*4)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				var saved tRec
				var err error
				caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "w" + strconv.Itoa(w)}}}, &saved, func(e error) { err = e })
				if err != nil {
					failures <- err
					continue
				}
				caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &tRec{Name: "x"}}, nil, func(e error) { err = e })
				var got tRecList
				caller.Call(operationOf(model.Read), &crud.ReadArgs{ID: saved.ID}, &got, func(e error) { err = e })
				caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{saved.ID}}, nil, func(e error) { err = e })
				if err != nil {
					failures <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("a concurrent request failed: %v", err)
	}
}
