package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestFileStoreLoad(t *testing.T) {
	t.Run("returns empty history for non-existent file", func(t *testing.T) {
		store := FileStore{Path: filepath.Join(t.TempDir(), "missing.json")}
		h, err := store.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(h.Entries) != 0 {
			t.Fatalf("expected empty history, got %d entries", len(h.Entries))
		}
	})

	t.Run("loads existing history", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		content := `{"entries":[{"timestamp":"2024-01-15T10:00:00Z","overall":75.5,"domains":{}}]}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write test file: %v", err)
		}

		store := FileStore{Path: path}
		h, err := store.Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(h.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(h.Entries))
		}
		if h.Entries[0].Overall != 75.5 {
			t.Fatalf("expected 75.5, got %f", h.Entries[0].Overall)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.json")
		if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
			t.Fatalf("write test file: %v", err)
		}

		store := FileStore{Path: path}
		_, err := store.Load()
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestFileStoreSave(t *testing.T) {
	t.Run("saves history to file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		store := FileStore{Path: path}

		h := domain.History{
			Entries: []domain.HistoryEntry{
				{
					Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
					Overall:   80.0,
					Domains: map[string]domain.DomainEntry{
						"core": {Name: "core", Percent: 85.0, Min: 80.0, Status: domain.StatusPass},
					},
				},
			},
		}

		if err := store.Save(h); err != nil {
			t.Fatalf("save: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}

		// Reload and verify
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if len(loaded.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
		}
		if loaded.Entries[0].Overall != 80.0 {
			t.Fatalf("expected 80.0, got %f", loaded.Entries[0].Overall)
		}
	})

	t.Run("creates directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "dir")
		path := filepath.Join(dir, "history.json")
		store := FileStore{Path: path}

		h := domain.History{
			Entries: []domain.HistoryEntry{{Overall: 70.0}},
		}

		if err := store.Save(h); err != nil {
			t.Fatalf("save: %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file: %v", err)
		}
	})
}

func TestFileStoreAppend(t *testing.T) {
	t.Run("appends entry to empty history", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		store := FileStore{Path: path}

		entry := domain.HistoryEntry{
			Timestamp: time.Now(),
			Overall:   75.0,
		}

		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}

		h, _ := store.Load()
		if len(h.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(h.Entries))
		}
	})

	t.Run("appends to existing history", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		store := FileStore{Path: path}

		// Add first entry
		_ = store.Append(domain.HistoryEntry{Overall: 70.0})

		// Add second entry
		if err := store.Append(domain.HistoryEntry{Overall: 75.0}); err != nil {
			t.Fatalf("append: %v", err)
		}

		h, _ := store.Load()
		if len(h.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(h.Entries))
		}
	})

	t.Run("limits history size", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.json")
		store := FileStore{Path: path, MaxEntries: 3}

		// Add more entries than max
		for i := 0; i < 5; i++ {
			_ = store.Append(domain.HistoryEntry{Overall: float64(70 + i)})
		}

		h, _ := store.Load()
		if len(h.Entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(h.Entries))
		}
		// Should keep the latest entries
		if h.Entries[0].Overall != 72.0 {
			t.Fatalf("expected oldest entry 72.0, got %f", h.Entries[0].Overall)
		}
	})
}
