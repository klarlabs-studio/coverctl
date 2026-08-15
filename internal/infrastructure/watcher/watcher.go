package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors Go source files for changes.
type Watcher struct {
	watcher    *fsnotify.Watcher
	debounce   time.Duration
	extensions []string
}

// Option configures the watcher.
type Option func(*Watcher)

// WithDebounce sets the debounce duration for file change events.
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// WithExtensions sets the file extensions to watch.
func WithExtensions(exts ...string) Option {
	return func(w *Watcher) {
		w.extensions = exts
	}
}

// New creates a new file watcher.
func New(opts ...Option) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:    fsw,
		debounce:   500 * time.Millisecond,
		extensions: []string{".go"},
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// WatchDir adds a directory and its subdirectories to the watch list.
func (w *Watcher) WatchDir(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		// Skip hidden directories and common non-source directories
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
			return filepath.SkipDir
		}
		return w.watcher.Add(path)
	})
}

// Events returns a channel that emits when relevant files change.
// The channel is debounced to avoid rapid successive triggers.
func (w *Watcher) Events(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})

	go func() {
		defer close(out)

		var timer *time.Timer
		var timerCh <-chan time.Time

		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return

			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Only trigger on write events for relevant file types
				if !isWriteEvent(event.Op) {
					continue
				}
				if !w.hasRelevantExtension(event.Name) {
					continue
				}

				// Debounce: reset timer on each event. Drain if Stop loses
				// the race with an already-expired timer so a stale tick
				// cannot fire alongside the new timer.
				if timer != nil {
					if !timer.Stop() && timerCh != nil {
						select {
						case <-timerCh:
						default:
						}
					}
				}
				timer = time.NewTimer(w.debounce)
				timerCh = timer.C

			case <-timerCh:
				// Debounce complete, send notification
				timer = nil
				timerCh = nil
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				// Log errors but continue watching
				_ = err
			}
		}
	}()

	return out
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	return w.watcher.Close()
}

func isWriteEvent(op fsnotify.Op) bool {
	return op&fsnotify.Write == fsnotify.Write ||
		op&fsnotify.Create == fsnotify.Create
}

func (w *Watcher) hasRelevantExtension(path string) bool {
	ext := filepath.Ext(path)
	for _, e := range w.extensions {
		if ext == e {
			return true
		}
	}
	return false
}
