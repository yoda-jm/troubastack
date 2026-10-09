//go:build !unix

package filerepo

import "os"

// tryLock is a no-op where flock is unavailable: the single-writer rule is then
// the operator's (stop the server before a CLI write), as it was before LockDir.
func tryLock(*os.File) error { return nil }
