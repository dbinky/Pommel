package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWatcher verifies that NewWatcher creates a watcher successfully
func TestNewWatcher(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 100,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, w)

	// Cleanup
	err = w.Stop()
	require.NoError(t, err)
}

// TestNewWatcherInvalidPath verifies that NewWatcher fails with invalid path
func TestNewWatcherInvalidPath(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 100,
		},
	}

	w, err := NewWatcher("/nonexistent/path/that/does/not/exist", cfg, nil)
	assert.Error(t, err)
	assert.Nil(t, w)
}

// TestWatcherStart verifies that Start begins watching without error
func TestWatcherStart(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 100,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)
}

// TestWatcherCreateEvent verifies that file creation triggers OpCreate event
func TestWatcherCreateEvent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-w.Events():
		assert.Equal(t, testFile, event.Path)
		assert.Equal(t, OpCreate, event.Op)
		assert.False(t, event.Timestamp.IsZero())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for create event")
	}
}

// TestWatcherModifyEvent verifies that file modification triggers OpModify event
func TestWatcherModifyEvent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file before starting watcher
	testFile := filepath.Join(tmpDir, "existing.txt")
	err := os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Small delay to ensure watcher is ready
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-w.Events():
		assert.Equal(t, testFile, event.Path)
		assert.Equal(t, OpModify, event.Op)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for modify event")
	}
}

// TestWatcherDeleteEvent verifies that file deletion triggers OpDelete event
func TestWatcherDeleteEvent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file before starting watcher
	testFile := filepath.Join(tmpDir, "to_delete.txt")
	err := os.WriteFile(testFile, []byte("to be deleted"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Small delay to ensure watcher is ready
	time.Sleep(50 * time.Millisecond)

	// Delete the file
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-w.Events():
		assert.Equal(t, testFile, event.Path)
		assert.Equal(t, OpDelete, event.Op)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for delete event")
	}
}

// TestWatcherDebouncing verifies that rapid file changes produce fewer events
func TestWatcherDebouncing(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 200, // 200ms debounce
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "rapid.txt")

	// Create the file first
	err = os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Small delay
	time.Sleep(50 * time.Millisecond)

	// Make rapid modifications (faster than debounce period)
	for i := 0; i < 10; i++ {
		err = os.WriteFile(testFile, []byte("content "+string(rune('0'+i))), 0644)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 10ms between writes
	}

	// Wait for debounce to settle plus some buffer
	time.Sleep(300 * time.Millisecond)

	// Count received events (should be fewer than 10 due to debouncing)
	eventCount := 0
	timeout := time.After(100 * time.Millisecond)
drainLoop:
	for {
		select {
		case <-w.Events():
			eventCount++
		case <-timeout:
			break drainLoop
		}
	}

	// We expect significantly fewer events than 10 due to debouncing
	// The exact number depends on implementation, but should be < 5
	assert.Less(t, eventCount, 5, "expected debouncing to reduce event count")
}

// TestWatcherDebounceConfigurable verifies debounce duration is configurable
func TestWatcherDebounceConfigurable(t *testing.T) {
	tmpDir := t.TempDir()

	// Very short debounce
	cfgShort := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 10,
		},
	}

	wShort, err := NewWatcher(tmpDir, cfgShort, nil)
	require.NoError(t, err)
	defer wShort.Stop()

	// Longer debounce
	tmpDir2 := t.TempDir()
	cfgLong := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 500,
		},
	}

	wLong, err := NewWatcher(tmpDir2, cfgLong, nil)
	require.NoError(t, err)
	defer wLong.Stop()

	// Both watchers should be created with their respective debounce values
	// The actual behavior difference would be observable in integration tests
	assert.NotNil(t, wShort)
	assert.NotNil(t, wLong)
}

// TestWatcherNewDirectoryWatched verifies that new directories are watched
func TestWatcherNewDirectoryWatched(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Create a new subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Small delay for directory to be picked up
	time.Sleep(100 * time.Millisecond)

	// Create a file in the new subdirectory
	testFile := filepath.Join(subDir, "newfile.txt")
	err = os.WriteFile(testFile, []byte("content in subdir"), 0644)
	require.NoError(t, err)

	// Wait for event - should see the file creation
	foundEvent := false
	timeout := time.After(500 * time.Millisecond)
eventLoop:
	for {
		select {
		case event := <-w.Events():
			if event.Path == testFile && event.Op == OpCreate {
				foundEvent = true
				break eventLoop
			}
		case <-timeout:
			break eventLoop
		}
	}

	assert.True(t, foundEvent, "expected to receive create event for file in new directory")
}

// TestWatcherNestedDirectories verifies that nested directories work
func TestWatcherNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure before starting watcher
	nestedDir := filepath.Join(tmpDir, "level1", "level2", "level3")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Small delay
	time.Sleep(50 * time.Millisecond)

	// Create a file in the deeply nested directory
	testFile := filepath.Join(nestedDir, "deep.txt")
	err = os.WriteFile(testFile, []byte("deep content"), 0644)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-w.Events():
		assert.Equal(t, testFile, event.Path)
		assert.Equal(t, OpCreate, event.Op)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event in nested directory")
	}
}

// TestWatcherEventsChannel verifies that Events() returns readable channel
func TestWatcherEventsChannel(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	events := w.Events()
	require.NotNil(t, events)

	// Channel should be receive-only
	var _ <-chan FileEvent = events
}

// TestWatcherEventContainsCorrectPathAndOp verifies events have correct Path and Op
func TestWatcherEventContainsCorrectPathAndOp(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Create a file
	testFile := filepath.Join(tmpDir, "event_test.go")
	err = os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	// Verify event
	select {
	case event := <-w.Events():
		// Path should be absolute
		assert.True(t, filepath.IsAbs(event.Path), "path should be absolute")
		// Path should match the created file
		assert.Equal(t, testFile, event.Path)
		// Op should be OpCreate
		assert.Equal(t, OpCreate, event.Op)
		// Timestamp should be set
		assert.False(t, event.Timestamp.IsZero())
		// Timestamp should be recent (within last minute)
		assert.WithinDuration(t, time.Now(), event.Timestamp, time.Minute)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

// TestWatcherStopClosesCleanly verifies that Stop closes watcher cleanly
func TestWatcherStopClosesCleanly(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Stop should complete without error
	err = w.Stop()
	require.NoError(t, err)

	// Calling Stop again should be safe (idempotent)
	err = w.Stop()
	assert.NoError(t, err)
}

// TestWatcherDoneChannelClosesOnStop verifies Done channel closes on stop
func TestWatcherDoneChannelClosesOnStop(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	done := w.Done()
	require.NotNil(t, done)

	// Done should not be closed yet
	select {
	case <-done:
		t.Fatal("Done channel should not be closed before Stop")
	default:
		// Expected
	}

	// Stop the watcher
	err = w.Stop()
	require.NoError(t, err)

	// Done should now be closed
	select {
	case <-done:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done channel should be closed after Stop")
	}
}

// TestWatcherErrorsChannel verifies Errors() returns readable channel
func TestWatcherErrorsChannel(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	errors := w.Errors()
	require.NotNil(t, errors)

	// Channel should be receive-only
	var _ <-chan error = errors
}

// TestWatcherContextCancellation verifies watcher stops on context cancellation
func TestWatcherContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())

	err = w.Start(ctx)
	require.NoError(t, err)

	done := w.Done()

	// Cancel context
	cancel()

	// Done should close after context cancellation
	select {
	case <-done:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done channel should close after context cancellation")
	}
}

// TestWatcherRenameEvent verifies that file rename triggers OpRename event
func TestWatcherRenameEvent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file before starting watcher
	oldPath := filepath.Join(tmpDir, "old_name.txt")
	err := os.WriteFile(oldPath, []byte("content"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs: 50,
		},
	}

	w, err := NewWatcher(tmpDir, cfg, nil)
	require.NoError(t, err)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// Small delay
	time.Sleep(50 * time.Millisecond)

	// Rename the file
	newPath := filepath.Join(tmpDir, "new_name.txt")
	err = os.Rename(oldPath, newPath)
	require.NoError(t, err)

	// We should receive at least one event related to the rename
	// (implementations may send OpRename, OpDelete+OpCreate, or similar)
	foundRenameRelatedEvent := false
	timeout := time.After(500 * time.Millisecond)
eventLoop:
	for {
		select {
		case event := <-w.Events():
			if event.Path == oldPath || event.Path == newPath {
				foundRenameRelatedEvent = true
				// If it's specifically OpRename, that's ideal
				if event.Op == OpRename {
					break eventLoop
				}
			}
		case <-timeout:
			break eventLoop
		}
	}

	assert.True(t, foundRenameRelatedEvent, "expected to receive event for rename operation")
}
