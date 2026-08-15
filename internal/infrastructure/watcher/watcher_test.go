package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsGoFileChanges(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(WithDebounce(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDir(tmpDir); err != nil {
		t.Fatalf("watch dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := w.Events(ctx)

	// Create a .go file to trigger an event
	goFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case <-events:
		// Success - event received
	case <-ctx.Done():
		t.Fatal("timeout waiting for file change event")
	}
}

func TestWatcherIgnoresNonGoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(WithDebounce(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDir(tmpDir); err != nil {
		t.Fatalf("watch dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	events := w.Events(ctx)

	// Create a non-.go file - should not trigger
	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case <-events:
		t.Fatal("should not receive event for non-.go file")
	case <-ctx.Done():
		// Expected - no event received
	}
}

func TestWatcherWithCustomExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(
		WithDebounce(50*time.Millisecond),
		WithExtensions(".go", ".mod"),
	)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDir(tmpDir); err != nil {
		t.Fatalf("watch dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := w.Events(ctx)

	// Create a .mod file - should trigger with custom extensions
	modFile := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(modFile, []byte("module test"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case <-events:
		// Success - event received
	case <-ctx.Done():
		t.Fatal("timeout waiting for .mod file change event")
	}
}

func TestWatcherSkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hidden directory
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(WithDebounce(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDir(tmpDir); err != nil {
		t.Fatalf("watch dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	events := w.Events(ctx)

	// Create a .go file in hidden dir - should not trigger
	goFile := filepath.Join(hiddenDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package hidden"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case <-events:
		t.Fatal("should not receive event for file in hidden directory")
	case <-ctx.Done():
		// Expected - no event received
	}
}

func TestWatcherDebounces(t *testing.T) {
	tmpDir := t.TempDir()

	// Debounce must be well above typical fsnotify delivery jitter so a
	// create+write burst coalesces under CI load (the previous 100ms window
	// with 20ms sleeps between writes could open a gap and emit twice).
	const debounce = 200 * time.Millisecond

	w, err := New(WithDebounce(debounce))
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDir(tmpDir); err != nil {
		t.Fatalf("watch dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	events := w.Events(ctx)
	goFile := filepath.Join(tmpDir, "test.go")

	// Burst writes with no artificial delay so create/write notifications
	// land inside a single debounce window.
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(goFile, []byte("package main // "+string(rune('a'+i))), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Wait for the coalesced event, then confirm quiet for another debounce.
	select {
	case <-events:
	case <-ctx.Done():
		t.Fatal("timeout waiting for debounced event")
	}

	select {
	case <-events:
		t.Fatal("expected a single debounced event, got a second")
	case <-time.After(debounce + 50*time.Millisecond):
		// Quiet period — coalescing held.
	}
}

func TestHasRelevantExtension(t *testing.T) {
	w := &Watcher{extensions: []string{".go", ".mod"}}

	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"go.mod", true},
		{"README.md", false},
		{"test.txt", false},
	}

	for _, tt := range tests {
		if got := w.hasRelevantExtension(tt.path); got != tt.want {
			t.Errorf("hasRelevantExtension(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
