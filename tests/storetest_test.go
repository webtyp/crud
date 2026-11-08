package tests

import (
	"testing"
	"webtyp.com/crud"
	"webtyp.com/crud/storetest"
)

func TestMemStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) crud.Store {
		return newMemStore()
	})
}
