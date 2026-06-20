package memstore_test

import (
	"testing"

	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
	"troubastack/core/internal/store/storetest"
)

func TestContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Collector {
		return memstore.New().(store.Collector)
	})
}
