package filerepo

import (
	"os"
	"path/filepath"
	"testing"

	"troubastack/core/internal/app"
)

// assertOwnerOnly reads the mode BACK from disk (umask can only remove bits, so the resulting mode is
// what matters, not the requested one — T107) and fails if any group/other bit is set.
func assertOwnerOnly(t *testing.T, path string, want os.FileMode) {
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

// T107: app.json holds every bcrypt hash + session token; after a write it must be owner-only, in an
// owner-only dir.
func TestFileModesAreOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	r, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.CreateUser(app.User{ID: "u1", Username: "m", PasswordHash: "$2a$10$hash"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	assertOwnerOnly(t, filepath.Join(dir, "app.json"), 0o600)
	assertOwnerOnly(t, dir, 0o700)
}

// T107: an install created before this change (0o755 dir + 0o644 app.json) is tightened IN PLACE on
// open — old installs must not stay wide open.
func TestPreExistingModesAreTightened(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "app.json"), 0o644); err != nil { // force the wide-open state past umask
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err != nil {
		t.Fatalf("New: %v", err)
	}
	assertOwnerOnly(t, filepath.Join(dir, "app.json"), 0o600)
	assertOwnerOnly(t, dir, 0o700)
}
