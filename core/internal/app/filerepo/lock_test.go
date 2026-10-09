//go:build unix

package filerepo

import (
	"errors"
	"testing"
)

func TestLockDirIsExclusiveUntilClosed(t *testing.T) {
	dir := t.TempDir()
	first, err := LockDir(dir)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	// A second holder — the CLI while the server runs — must be refused, not
	// allowed to write a file the first holder will overwrite.
	if second, err := LockDir(dir); !errors.Is(err, ErrLocked) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("second lock while held: err = %v, want ErrLocked", err)
	}
	first.Close()
	again, err := LockDir(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	again.Close()
}

func TestLockDirDoesNotBlockRepoHandles(t *testing.T) {
	dir := t.TempDir()
	l, err := LockDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// The lock is the process's, not every handle's: New on a locked dir still opens.
	if _, err := New(dir); err != nil {
		t.Fatalf("New on a locked dir: %v", err)
	}
}
