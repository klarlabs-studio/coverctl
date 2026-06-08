//go:build unix

package history

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
)

// safeIntFd converts a uintptr file descriptor to int, guarding against overflow.
func safeIntFd(fd uintptr) (int, error) {
	if fd > uintptr(math.MaxInt) {
		return 0, fmt.Errorf("file descriptor %d overflows int", fd)
	}
	return int(fd), nil
}

// fileLock represents a file-based lock for concurrent access protection.
type fileLock struct {
	file *os.File
}

// acquireLock creates an exclusive lock on the history file.
// This prevents race conditions when multiple processes access the file.
func (s *FileStore) acquireLock() (*fileLock, error) {
	lockPath := s.Path + ".lock"
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	// Acquire exclusive lock (blocking)
	fd, err := safeIntFd(file.Fd())
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = file.Close() // Best-effort close on lock failure
		return nil, err
	}

	return &fileLock{file: file}, nil
}

// release releases the file lock.
func (l *fileLock) release() error {
	if l.file == nil {
		return nil
	}
	// Release lock - best-effort, always close file afterwards
	fd, fdErr := safeIntFd(l.file.Fd())
	if fdErr != nil {
		return l.file.Close()
	}
	unlockErr := syscall.Flock(fd, syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
