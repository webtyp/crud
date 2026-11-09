package tests

import (
	"testing"
	"webtyp.com/crud"
	"webtyp.com/crud/conformance"
)

func TestMemStore(t *testing.T) {
	conformance.Run(t, func(t *testing.T) crud.Store {
		return newMemStore()
	})
}
