//go:build !windows

package storage

import (
	"os"
	"syscall"
)

// lockFileExclusive takes an exclusive advisory lock on f, blocking until
// acquired. BSD flock on Unix (lock_windows.go carries the LockFileEx
// counterpart). syscall.Flock does not exist on Windows — a direct call
// here broke every Windows build, and with it the Release workflow, since
// the per-entity metadata refactor introduced GlobalLock.
//
// The lock is advisory; the passed *os.File owns it — closing the file
// (or process exit) releases the lock.
func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}
