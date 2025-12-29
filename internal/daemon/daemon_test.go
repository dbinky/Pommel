package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// daemonTestLogger returns a logger that discards all output
func daemonTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// daemonTestConfig returns a config with short timeouts for testing
func daemonTestConfig() *config.Config {
	cfg := config.Default()
	cfg.Watcher.DebounceMs = 10 // Short debounce for tests
	cfg.Daemon.Port = 0        // Use random available port
	return cfg
}

// Note: SearchRequest, SearchResponse, and SearchResult are now defined in daemon.go

// =============================================================================
// Daemon Creation Tests
// =============================================================================

func TestNew_CreatesWithValidConfig(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)
}

func TestNew_InitializesAllComponents(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Verify all components are initialized
	assert.NotNil(t, daemon.watcher, "watcher should be initialized")
	assert.NotNil(t, daemon.indexer, "indexer should be initialized")
	assert.NotNil(t, daemon.state, "state manager should be initialized")
	assert.NotNil(t, daemon.db, "database should be initialized")
	assert.NotNil(t, daemon.embedder, "embedder should be initialized")
}

func TestNew_FailsWithInvalidProjectRoot(t *testing.T) {
	// Arrange
	projectRoot := "/nonexistent/path/that/does/not/exist"
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)
}

func TestNew_FailsWithNilConfig(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, nil, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)
}

// =============================================================================
// Lifecycle Tests
// =============================================================================

func TestRun_StartsAllComponents(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait a bit for startup
	time.Sleep(100 * time.Millisecond)

	// Assert - daemon should be running (context timeout will stop it)
	cancel()
	err = <-errCh
	// Context cancellation is expected, not a failure
	assert.True(t, err == nil || err == context.Canceled || err == context.DeadlineExceeded,
		"Run should complete without unexpected error, got: %v", err)
}

func TestRun_WritesPIDFile(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Assert
	pidPath := filepath.Join(projectRoot, ".pommel", "pommel.pid")
	_, err = os.Stat(pidPath)
	assert.NoError(t, err, "PID file should exist at %s", pidPath)

	// Cleanup
	cancel()
	<-errCh
}

func TestRun_ChecksForAlreadyRunningDaemon(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	// Create first daemon and start it
	daemon1, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- daemon1.Run(ctx1)
	}()

	// Wait for first daemon to start
	time.Sleep(100 * time.Millisecond)

	// Create second daemon
	daemon2, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	// Act - try to run second daemon
	err = daemon2.Run(ctx2)

	// Assert - should fail because daemon is already running
	assert.Error(t, err, "Run should fail when another daemon is already running")

	// Cleanup
	cancel1()
	<-errCh1
}

func TestRun_GracefulShutdownOnContextCancel(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Assert - should shutdown gracefully within reasonable time
	select {
	case err := <-errCh:
		// Context cancellation is expected
		assert.True(t, err == nil || err == context.Canceled,
			"Should shutdown gracefully, got: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Daemon did not shutdown within timeout")
	}

	// PID file should be cleaned up
	pidPath := filepath.Join(projectRoot, ".pommel", "pommel.pid")
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "PID file should be removed after shutdown")
}

func TestRun_GracefulShutdownOnSIGTERM(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM to current process
	// Note: This tests signal handling setup, but in tests we send to self
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = process.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Assert - should shutdown gracefully within reasonable time
	select {
	case err := <-errCh:
		// Signal-triggered shutdown is expected
		assert.True(t, err == nil || err == context.Canceled,
			"Should shutdown gracefully on SIGTERM, got: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Daemon did not shutdown within timeout after SIGTERM")
	}
}

// =============================================================================
// File Event Processing Tests
// =============================================================================

func TestFileCreate_TriggersIndexing(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Act - create a new file
	testFile := filepath.Join(projectRoot, "test.go")
	err = os.WriteFile(testFile, []byte("package main\n\nfunc hello() {}"), 0644)
	require.NoError(t, err)

	// Wait for debounce and indexing
	time.Sleep(200 * time.Millisecond)

	// Assert - file should be indexed
	// This will need to be verified via the database or indexer stats
	stats := daemon.indexer.Stats()
	assert.Greater(t, stats.TotalFiles, int64(0), "File should be indexed")

	// Cleanup
	cancel()
	<-errCh
}

func TestFileModify_TriggersReindexing(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create initial file
	testFile := filepath.Join(projectRoot, "test.go")
	err := os.WriteFile(testFile, []byte("package main\n\nfunc hello() {}"), 0644)
	require.NoError(t, err)

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for initial indexing
	time.Sleep(200 * time.Millisecond)

	initialStats := daemon.indexer.Stats()

	// Act - modify the file
	err = os.WriteFile(testFile, []byte("package main\n\nfunc hello() {}\nfunc world() {}"), 0644)
	require.NoError(t, err)

	// Wait for debounce and re-indexing
	time.Sleep(200 * time.Millisecond)

	// Assert - file should be re-indexed (chunk count might change)
	finalStats := daemon.indexer.Stats()
	assert.GreaterOrEqual(t, finalStats.TotalChunks, initialStats.TotalChunks,
		"Chunks should be updated after modification")

	// Cleanup
	cancel()
	<-errCh
}

func TestFileDelete_TriggersRemovalFromIndex(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create initial file
	testFile := filepath.Join(projectRoot, "test.go")
	err := os.WriteFile(testFile, []byte("package main\n\nfunc hello() {}"), 0644)
	require.NoError(t, err)

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for initial indexing
	time.Sleep(200 * time.Millisecond)

	initialStats := daemon.indexer.Stats()
	require.Greater(t, initialStats.TotalFiles, int64(0), "File should be initially indexed")

	// Act - delete the file
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Wait for debounce and removal
	time.Sleep(200 * time.Millisecond)

	// Assert - file should be removed from index
	finalStats := daemon.indexer.Stats()
	assert.Less(t, finalStats.TotalFiles, initialStats.TotalFiles,
		"File count should decrease after deletion")

	// Cleanup
	cancel()
	<-errCh
}

// =============================================================================
// Initial Indexing Tests
// =============================================================================

func TestInitialIndex_RunsWhenDatabaseEmpty(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create some files before starting daemon
	testFile1 := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile1, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	testFile2 := filepath.Join(projectRoot, "util.go")
	err = os.WriteFile(testFile2, []byte("package main\n\nfunc helper() {}"), 0644)
	require.NoError(t, err)

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for initial indexing to complete
	time.Sleep(500 * time.Millisecond)

	// Assert - existing files should be indexed
	stats := daemon.indexer.Stats()
	assert.GreaterOrEqual(t, stats.TotalFiles, int64(2),
		"Existing files should be indexed on startup")

	// Cleanup
	cancel()
	<-errCh
}

func TestInitialIndex_SkippedWhenDatabaseHasData(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create a file
	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// First daemon run to populate database
	daemon1, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- daemon1.Run(ctx1)
	}()

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)
	cancel1()
	<-errCh1

	// Record the indexed file count
	stats1 := daemon1.indexer.Stats()

	// Create second daemon - should skip initial indexing
	daemon2, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	startTime := time.Now()
	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- daemon2.Run(ctx2)
	}()

	// Wait briefly for startup (should be fast since no reindexing)
	time.Sleep(100 * time.Millisecond)

	// Assert - data should still be present, startup should be fast
	stats2 := daemon2.indexer.Stats()
	assert.Equal(t, stats1.TotalFiles, stats2.TotalFiles,
		"Database data should be preserved between daemon restarts")

	// Startup should be faster than a full reindex would take
	elapsed := time.Since(startTime)
	assert.Less(t, elapsed, 400*time.Millisecond,
		"Daemon startup should be fast when skipping initial indexing")

	// Cleanup
	cancel2()
	<-errCh2
}

// =============================================================================
// API Server Tests
// =============================================================================

func TestAPIServer_StartsOnConfiguredPort(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = 17421 // Use a specific test port
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Act - try to connect to the server
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17421/health")

	// Assert
	require.NoError(t, err, "Should be able to connect to API server")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Cleanup
	cancel()
	<-errCh
}

func TestAPIServer_HealthEndpointAccessible(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = 17422 // Use a specific test port
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Act
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17422/health")

	// Assert
	require.NoError(t, err, "Health endpoint should be accessible")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Cleanup
	cancel()
	<-errCh
}

// =============================================================================
// Search Tests
// =============================================================================

func TestDaemon_Search(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create a file with searchable content
	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte(`package main

// calculateSum adds two numbers together
func calculateSum(a, b int) int {
	return a + b
}

// calculateDifference subtracts two numbers
func calculateDifference(a, b int) int {
	return a - b
}
`), 0644)
	require.NoError(t, err)

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx)
	}()

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)

	// Act
	req := SearchRequest{
		Query: "add two numbers",
		Limit: 5,
	}
	resp, err := daemon.Search(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Results, "Search should return results")

	// The calculateSum function should be in results
	found := false
	for _, result := range resp.Results {
		if result.FilePath == testFile {
			found = true
			break
		}
	}
	assert.True(t, found, "Search results should include the test file")

	// Cleanup
	cancel()
	<-errCh
}
