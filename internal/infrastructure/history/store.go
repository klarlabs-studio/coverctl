package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/domain"
)

// DefaultMaxEntries is the default number of history entries to keep.
const DefaultMaxEntries = 100

// FileStore provides JSON file-based storage for coverage history.
type FileStore struct {
	Path       string
	MaxEntries int
}

// Note: fileLock and acquireLock/release are defined in platform-specific files:
// - lock_unix.go for Unix systems (Linux, macOS, BSD)
// - lock_windows.go for Windows

// Load reads the history from the JSON file.
// Returns an empty history if the file doesn't exist.
func (s *FileStore) Load() (domain.History, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.History{}, nil
		}
		return domain.History{}, err
	}

	var h domain.History
	if err := json.Unmarshal(data, &h); err != nil {
		return domain.History{}, err
	}

	return h, nil
}

// Save writes the history to the JSON file.
func (s *FileStore) Save(h domain.History) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path, data, 0o600)
}

// Append adds a new entry to the history and saves it.
// If MaxEntries is set, older entries are removed to maintain the limit.
// Uses file locking to prevent race conditions with concurrent processes.
func (s *FileStore) Append(entry domain.HistoryEntry) error {
	// Acquire exclusive lock to prevent race conditions
	lock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer lock.release()

	h, err := s.Load()
	if err != nil {
		return err
	}

	h.Entries = append(h.Entries, entry)

	// Trim to max entries if configured
	max := s.MaxEntries
	if max == 0 {
		max = DefaultMaxEntries
	}
	if len(h.Entries) > max {
		h.Entries = h.Entries[len(h.Entries)-max:]
	}

	return s.Save(h)
}
