package app

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestParseBandZip_ZipBombCaps: parseBandZip refuses an archive with too many entries or
// one that decompresses past the running budget — the T63 zip-bomb hardening that gates
// re-enabling import (reviews.md 2026-07-26, Condition 1).
func TestParseBandZip_ZipBombCaps(t *testing.T) {
	t.Run("entry count cap", func(t *testing.T) {
		defer func(v int) { maxImportEntries = v }(maxImportEntries)
		maxImportEntries = 1
		z := makeZip(t, map[string]string{"band.json": `{"formatVersion":1}`, "blobs/x": "y"})
		if _, _, err := parseBandZip(z); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("too-many-entries err=%v, want ErrInvalidInput", err)
		}
	})
	t.Run("decompressed total cap", func(t *testing.T) {
		defer func(v int64) { maxDecompressedBytes = v }(maxDecompressedBytes)
		maxDecompressedBytes = 5 // band.json below is well over 5 bytes
		z := makeZip(t, map[string]string{"band.json": `{"formatVersion":1,"band":{"name":"x"}}`})
		if _, _, err := parseBandZip(z); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("oversized-decompressed err=%v, want ErrInvalidInput", err)
		}
	})
	t.Run("within caps parses fine", func(t *testing.T) {
		z := makeZip(t, map[string]string{"band.json": `{"formatVersion":1,"band":{"name":"OK"}}`})
		man, _, err := parseBandZip(z)
		if err != nil || man.Band.Name != "OK" {
			t.Fatalf("parse within caps: man=%+v err=%v", man, err)
		}
	})
}
