package filerepo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrLocked means another process (normally the running server) holds the data
// dir's writer lock.
var ErrLocked = errors.New("filerepo: data dir is in use by another troubacore process")

// LockDir takes the data dir's single-writer lock (<dir>/app.lock) and returns
// the open lock file; the lock lives until it is closed or the process exits.
//
// filerepo keeps the whole dataset in memory and rewrites app.json whole on every
// mutation, so two writers on one dir silently lose each other's writes — the
// operator's `reset-password` against a live server printed a token the server
// then overwrote on its next flush. The server holds this lock for its lifetime;
// the offline CLIs that write the app store take it too, and fail with ErrLocked
// instead of writing into a file the server will clobber. Advisory (flock), so
// it works across containers that mount the same volume. New does NOT take it:
// it is the process's job, once, not every repo handle's.
func LockDir(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("filerepo: mkdir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "app.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("filerepo: open lock: %w", err)
	}
	if err := tryLock(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
