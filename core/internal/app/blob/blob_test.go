package blob_test

import (
	"bytes"
	"testing"

	"troubastack/core/internal/app/blob"
)

func TestStores(t *testing.T) {
	dir := t.TempDir()
	fileStore, err := blob.NewFile(dir)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	stores := map[string]blob.Store{
		"mem":  blob.NewMem(),
		"file": fileStore,
	}
	for name, s := range stores {
		t.Run(name, func(t *testing.T) {
			data := []byte("hello sheet music")
			h, err := s.Put(data)
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			if h != blob.HashOf(data) {
				t.Fatalf("hash %q != HashOf %q", h, blob.HashOf(data))
			}
			// Idempotent: same bytes → same hash, no error.
			if h2, err := s.Put(data); err != nil || h2 != h {
				t.Fatalf("second Put = (%q,%v), want (%q,nil)", h2, err, h)
			}
			got, err := s.Get(h)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if !bytes.Equal(got, data) {
				t.Fatalf("Get returned different bytes")
			}
			if _, err := s.Get("deadbeef"); err != blob.ErrNotFound {
				t.Fatalf("Get(unknown) err = %v, want ErrNotFound", err)
			}
		})
	}
}
