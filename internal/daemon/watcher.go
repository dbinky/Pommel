package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pommel-dev/pommel/internal/config"
)

// Operation represents the type of file operation
type Operation int

const (
	OpCreate Operation = iota
	OpModify
	OpDelete
	OpRename
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Op        Operation
	Timestamp time.Time
}

// Watcher watches for file system changes in a project directory
type Watcher struct {
	projectRoot string
	config      *config.Config
	logger      *slog.Logger
	fsWatcher   *fsnotify.Watcher
	events      chan FileEvent
	errors      chan error
	pending     map[string]*time.Timer // for debouncing
	pendingOp   map[string]Operation   // track operation type
	pendingMu   sync.Mutex
	ignorer     *Ignorer
	done        chan struct{}
	stopped     bool
	stoppedMu   sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewWatcher creates a new file watcher for the given project root
func NewWatcher(projectRoot string, cfg *config.Config, logger *slog.Logger) (*Watcher, error) {
	// Verify project root exists
	info, err := os.Stat(projectRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrNotExist
	}

	// Use a no-op logger if none provided
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Create ignorer with exclude patterns from config
	ignorer, err := NewIgnorer(projectRoot, cfg.ExcludePatterns)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	w := &Watcher{
		projectRoot: projectRoot,
		config:      cfg,
		logger:      logger,
		fsWatcher:   fsWatcher,
		events:      make(chan FileEvent, 100),
		errors:      make(chan error, 10),
		pending:     make(map[string]*time.Timer),
		pendingOp:   make(map[string]Operation),
		ignorer:     ignorer,
		done:        make(chan struct{}),
	}

	return w, nil
}

// Start begins watching the project directory for changes
func (w *Watcher) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	// Add all existing directories to watch
	if err := w.addWatchRecursively(w.projectRoot); err != nil {
		return err
	}

	// Start event processing goroutine
	go w.processEvents()

	return nil
}

// addWatchRecursively adds the directory and all subdirectories to the watcher
func (w *Watcher) addWatchRecursively(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			w.logger.Debug("skipping inaccessible path", "path", path, "error", err)
			return nil // Skip paths we can't access
		}

		if info.IsDir() {
			// Skip ignored directories
			if w.ignorer.ShouldIgnore(path) {
				w.logger.Debug("skipping ignored directory", "path", path)
				return filepath.SkipDir
			}

			// Add directory to watcher
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Warn("failed to watch directory", "path", path, "error", err)
				return nil
			}
		}

		return nil
	})
}

// processEvents handles events from fsnotify and applies debouncing
func (w *Watcher) processEvents() {
	defer close(w.done)

	for {
		select {
		case <-w.ctx.Done():
			w.cancelAllPending()
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleFsEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
				// Drop error if channel is full
			}
		}
	}
}

// handleFsEvent processes a single fsnotify event
func (w *Watcher) handleFsEvent(event fsnotify.Event) {
	path := event.Name

	// Skip ignored paths
	if w.ignorer.ShouldIgnore(path) {
		w.logger.Debug("ignoring event for excluded path", "path", path, "event", event.Op.String())
		return
	}

	// Determine operation type
	op := w.fsEventToOp(event)

	// Handle new directories - add them to the watch list
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			w.fsWatcher.Add(path)
		}
	}

	// Skip directory events (we only care about file events)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return
	}

	// For delete/rename, we can't stat the file, so proceed
	if !event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return
		}
	}

	// Apply debouncing
	w.debounceEvent(path, op)
}

// fsEventToOp converts an fsnotify event to our Operation type
func (w *Watcher) fsEventToOp(event fsnotify.Event) Operation {
	if event.Has(fsnotify.Create) {
		return OpCreate
	}
	if event.Has(fsnotify.Remove) {
		return OpDelete
	}
	if event.Has(fsnotify.Rename) {
		return OpRename
	}
	if event.Has(fsnotify.Write) {
		return OpModify
	}
	if event.Has(fsnotify.Chmod) {
		return OpModify
	}
	return OpModify
}

// debounceEvent applies debouncing to an event
func (w *Watcher) debounceEvent(path string, op Operation) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	debounceDuration := w.config.Watcher.DebounceDuration()

	// Cancel existing timer if any
	if timer, exists := w.pending[path]; exists {
		timer.Stop()
	}

	// Update the operation - prefer create/delete/rename over modify
	existingOp, hasExisting := w.pendingOp[path]
	if !hasExisting {
		w.pendingOp[path] = op
	} else if existingOp == OpModify && op != OpModify {
		// Upgrade from modify to a more specific operation
		w.pendingOp[path] = op
	}

	// Create new timer
	w.pending[path] = time.AfterFunc(debounceDuration, func() {
		w.pendingMu.Lock()
		finalOp := w.pendingOp[path]
		delete(w.pending, path)
		delete(w.pendingOp, path)
		w.pendingMu.Unlock()

		// Send event
		event := FileEvent{
			Path:      path,
			Op:        finalOp,
			Timestamp: time.Now(),
		}

		select {
		case w.events <- event:
		case <-w.ctx.Done():
		}
	})
}

// cancelAllPending cancels all pending debounce timers
func (w *Watcher) cancelAllPending() {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	for path, timer := range w.pending {
		timer.Stop()
		delete(w.pending, path)
		delete(w.pendingOp, path)
	}
}

// Events returns a receive-only channel for file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Errors returns a receive-only channel for errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Done returns a channel that is closed when the watcher stops
func (w *Watcher) Done() <-chan struct{} {
	return w.done
}

// Stop stops the watcher and cleans up resources
func (w *Watcher) Stop() error {
	w.stoppedMu.Lock()
	if w.stopped {
		w.stoppedMu.Unlock()
		return nil
	}
	w.stopped = true
	w.stoppedMu.Unlock()

	// Cancel context if set
	if w.cancel != nil {
		w.cancel()
	}

	// Close fsnotify watcher
	if err := w.fsWatcher.Close(); err != nil {
		return err
	}

	// Wait for done channel to close (with timeout)
	select {
	case <-w.done:
	case <-time.After(time.Second):
		// Force close done if processEvents hasn't closed it
		// This can happen if Start was never called
	}

	return nil
}
