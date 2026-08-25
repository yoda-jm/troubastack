package blob_test

import (
	"os"
	"path/filepath"
	"testing"

	"troubastack/core/internal/app/blob"
)

func ownerOnly(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Fatalf("%s is group/world accessible: mode %04o", path, got)
	} else if got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}

// T107: blobs are user content (uploaded PDFs/images) — owner-only file in an owner-only dir.
func TestBlobModesAreOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	s, err := blob.NewFile(dir)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	h, err := s.Put([]byte("uploaded pdf bytes"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	ownerOnly(t, filepath.Join(dir, h), 0o600)
	ownerOnly(t, dir, 0o700)
}

// T107: a blob dir from an old install is tightened in place on open.
func TestPreExistingBlobDirTightened(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := blob.NewFile(dir); err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	ownerOnly(t, dir, 0o700)
}
