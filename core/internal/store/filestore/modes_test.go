package filestore_test

import (
	"os"
	"path/filepath"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
)

// T107: annotation history (songs/*.jsonl) is user content — owner-only file in an owner-only dir.
func TestRecordModesAreOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	st := filestore.New(dir).(store.Collector)
	if _, err := st.AppendRevision("song1", domain.Revision{Summary: "r1"}); err != nil {
		t.Fatalf("AppendRevision: %v", err)
	}
	rec := filepath.Join(dir, "songs", "song1.jsonl")
	info, err := os.Stat(rec)
	if err != nil {
		t.Fatalf("stat %s: %v", rec, err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Fatalf("%s is group/world accessible: mode %04o", rec, got)
	} else if got != 0o600 {
		t.Fatalf("%s mode = %04o, want 0600", rec, got)
	}
	d, err := os.Stat(filepath.Join(dir, "songs"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Mode().Perm()&0o077 != 0 {
		t.Fatalf("songs dir is group/world accessible: %04o", d.Mode().Perm())
	}
}
