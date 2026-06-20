package filestore_test

import (
	"testing"

	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
	"troubastack/core/internal/store/storetest"
)

func TestContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Collector {
		return filestore.New(t.TempDir()).(store.Collector)
	})
}
