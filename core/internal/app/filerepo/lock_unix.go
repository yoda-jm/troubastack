//go:build unix

package filerepo

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func tryLock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrLocked
		}
		return fmt.Errorf("filerepo: lock: %w", err)
	}
	return nil
}
