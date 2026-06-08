//go:build windows

package history

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

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

	// Acquire exclusive lock using Windows LockFileEx
	handle := windows.Handle(file.Fd())
	overlapped := &windows.Overlapped{}
	err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &fileLock{file: file}, nil
}

// release releases the file lock.
func (l *fileLock) release() error {
	if l.file == nil {
		return nil
	}

	handle := windows.Handle(l.file.Fd())
	overlapped := &windows.Overlapped{}
	unlockErr := windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
