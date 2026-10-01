//go:build windows

package storage

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive takes an exclusive advisory lock on f, blocking until
// acquired. Windows port: LockFileEx via x/sys/windows (lock_unix.go
// carries the flock counterpart). syscall.Flock does not exist on Windows —
// a direct call broke every Windows build, and with it the Release
// workflow, since the per-entity metadata refactor introduced GlobalLock.
//
// LockFileEx locks a byte range; we lock from offset 0 with no length
// limit (0 = to end of file) and block (LOCKFILE_EXCLUSIVE_LOCK without
// LOCKFILE_FAIL_IMMEDIATELY). Closing the file or process exit releases
// the lock.
func lockFileExclusive(f *os.File) error {
	h := windows.Handle(f.Fd())
	return windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 0, ^uint32(0), &windows.Overlapped{})
}
