package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

// intPtr is a helper to create *int values
func intPtr(i int) *int {
	return &i
}

// daemonTestConfig returns a config with short timeouts for testing
func daemonTestConfig() *config.Config {
	cfg := config.Default()
	cfg.Watcher.DebounceMs = 10 // Short debounce for tests
	cfg.Daemon.Port = intPtr(0) // Use random available port (0 = system-assigned)
	return cfg
}

// skipIfNoOllama skips the test if Ollama is not available
func skipIfNoOllama(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Skipping test: Ollama not available")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Skipping test: Ollama not responding correctly")
	}
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

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
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

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
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

	// Cleanup - close daemon2 first (it didn't run, so just close resources)
	require.NoError(t, daemon2.Close())

	// Then stop daemon1
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
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM signal handling not supported on Windows; using os.Interrupt instead")
	}

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

	// Send SIGTERM to current process (Unix only)
	// Note: This tests signal handling setup, but in tests we send to self
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = sendSIGTERM(process)
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
	skipIfNoOllama(t)
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
	cfg.Daemon.Port = intPtr(17421) // Use a specific test port
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
	cfg.Daemon.Port = intPtr(17422) // Use a specific test port
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
	skipIfNoOllama(t)
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

// =============================================================================
// DaemonError Tests
// =============================================================================

func TestDaemonError_ImplementsError(t *testing.T) {
	// Verify DaemonError implements the error interface
	var err error = &DaemonError{
		Code:    "TEST_ERROR",
		Message: "Test error message",
	}
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Test error message")
}

func TestDaemonError_ErrorMethod_WithoutSuggestion(t *testing.T) {
	err := &DaemonError{
		Code:    "TEST_CODE",
		Message: "Test message without suggestion",
	}

	expected := "Test message without suggestion"
	assert.Equal(t, expected, err.Error())
}

func TestDaemonError_ErrorMethod_WithSuggestion(t *testing.T) {
	err := &DaemonError{
		Code:       "TEST_CODE",
		Message:    "Test message",
		Suggestion: "Try this to fix the issue",
	}

	expected := "Test message. Try this to fix the issue"
	assert.Equal(t, expected, err.Error())
}

func TestDaemonError_Unwrap_ReturnsCause(t *testing.T) {
	cause := assert.AnError
	err := &DaemonError{
		Code:    "WRAPPED_ERROR",
		Message: "Wrapped error",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, cause, unwrapped)
}

func TestDaemonError_Unwrap_ReturnsNilWhenNoCause(t *testing.T) {
	err := &DaemonError{
		Code:    "NO_CAUSE_ERROR",
		Message: "Error without cause",
	}

	unwrapped := err.Unwrap()
	assert.Nil(t, unwrapped)
}

func TestDaemonError_AllFields(t *testing.T) {
	cause := assert.AnError
	err := &DaemonError{
		Code:       "FULL_ERROR",
		Message:    "Full error message",
		Suggestion: "Full suggestion",
		Cause:      cause,
	}

	assert.Equal(t, "FULL_ERROR", err.Code)
	assert.Equal(t, "Full error message", err.Message)
	assert.Equal(t, "Full suggestion", err.Suggestion)
	assert.Equal(t, cause, err.Cause)
}

// =============================================================================
// New Daemon Error Cases
// =============================================================================

func TestNew_FailsWithFileAsProjectRoot(t *testing.T) {
	// Create a file instead of a directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "somefile.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	// Act - try to create daemon with a file path as project root
	daemon, err := New(filePath, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)

	// Verify it's the right error type
	var daemonErr *DaemonError
	require.ErrorAs(t, err, &daemonErr)
	assert.Equal(t, "PROJECT_ROOT_NOT_DIRECTORY", daemonErr.Code)
}

func TestNew_CreatesDefaultLogger_WhenNilProvided(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()

	// Act - pass nil logger
	daemon, err := New(projectRoot, cfg, nil)

	// Assert - should succeed with a default logger
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
}

func TestNew_UsesDefaultCacheSize_WhenZeroProvided(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.CacheSize = 0 // Zero cache size
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert - should succeed with default cache size
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
}

func TestNew_UsesDefaultCacheSize_WhenNegativeProvided(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.CacheSize = -100 // Negative cache size
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert - should succeed with default cache size
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
}

// =============================================================================
// Search Error Cases
// =============================================================================

func TestDaemon_Search_WithLevelFilter(t *testing.T) {
	skipIfNoOllama(t)
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create a test file
	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte(`package main

func main() {}
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

	// Act - search with level filter
	req := SearchRequest{
		Query:  "main function",
		Limit:  5,
		Levels: []string{"method"},
	}
	resp, err := daemon.Search(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Results should all be of the method level
	for _, result := range resp.Results {
		assert.Equal(t, "method", result.Level)
	}

	// Cleanup
	cancel()
	<-errCh
}

func TestDaemon_Search_WithPathPrefix(t *testing.T) {
	skipIfNoOllama(t)
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	// Create subdirectory
	subDir := filepath.Join(projectRoot, "pkg", "handler")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create files in different locations
	err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte(`package main

func main() {}
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, "handler.go"), []byte(`package handler

func Handler() {}
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

	// Act - search with path prefix
	req := SearchRequest{
		Query:      "function",
		Limit:      10,
		PathPrefix: "pkg/",
	}
	resp, err := daemon.Search(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	// All results should be from pkg/
	for _, result := range resp.Results {
		assert.Contains(t, result.FilePath, "pkg/")
	}

	// Cleanup
	cancel()
	<-errCh
}

func TestDaemon_Search_ScoreCalculation(t *testing.T) {
	skipIfNoOllama(t)
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	logger := daemonTestLogger()

	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte(`package main

func main() {}
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
		Query: "main function",
		Limit: 5,
	}
	resp, err := daemon.Search(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Scores should be between 0 and 1
	for _, result := range resp.Results {
		assert.GreaterOrEqual(t, result.Score, 0.0)
		assert.LessOrEqual(t, result.Score, 1.0)
	}

	// Cleanup
	cancel()
	<-errCh
}

func TestDaemon_Search_DefaultLimit(t *testing.T) {
	skipIfNoOllama(t)
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.Search.DefaultLimit = 5 // Set a specific default limit
	logger := daemonTestLogger()

	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte(`package main

func main() {}
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

	// Act - search without specifying limit
	req := SearchRequest{
		Query: "main",
		Limit: 0, // Zero means use default
	}
	resp, err := daemon.Search(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 5, resp.Limit, "Should use configured default limit")

	// Cleanup
	cancel()
	<-errCh
}

// =============================================================================
// SearchService Tests
// =============================================================================

func TestDaemon_SearchService(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	logger := daemonTestLogger()

	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	// Act
	svc := daemon.SearchService()

	// Assert
	assert.NotNil(t, svc, "SearchService should return a non-nil service")

	// Cleanup - required on Windows to release file handles
	require.NoError(t, daemon.Close())
}

// =============================================================================
// HTTP Handler Tests (Internal)
// =============================================================================

func TestDaemon_HandleHealth_ReturnsOK(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17423)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Make HTTP request to health endpoint
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17423/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify response contains required fields
	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, projectRoot, health["project_root"])
	assert.Equal(t, float64(17423), health["port"])
	assert.Contains(t, health, "timestamp")

	cancel()
	<-errCh
}

func TestDaemon_HandleStatus_ReturnsStatus(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17424)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Make HTTP request to status endpoint
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17424/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify JSON response structure
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	_, hasDaemon := result["daemon"]
	_, hasIndex := result["index"]
	assert.True(t, hasDaemon, "Response should have daemon key")
	assert.True(t, hasIndex, "Response should have index key")

	cancel()
	<-errCh
}

func TestDaemon_HandleSearch_MethodNotAllowed(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17425)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// GET should not be allowed for search
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17425/search")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	cancel()
	<-errCh
}

func TestDaemon_HandleSearch_InvalidJSON(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17426)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Send invalid JSON
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:17426/search", "application/json", bytes.NewBufferString("{invalid}"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	cancel()
	<-errCh
}

func TestDaemon_HandleSearch_ValidRequest(t *testing.T) {
	skipIfNoOllama(t)
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.Daemon.Port = intPtr(17427)
	logger := daemonTestLogger()

	// Create a test file
	testFile := filepath.Join(projectRoot, "main.go")
	err := os.WriteFile(testFile, []byte(`package main

func main() {}
`), 0644)
	require.NoError(t, err)

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(500 * time.Millisecond) // Wait for indexing

	// Send valid search request
	reqBody := `{"query": "main function", "limit": 5}`
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:17427/search", "application/json", bytes.NewBufferString(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	<-errCh
}

func TestDaemon_HandleReindex_MethodNotAllowed(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17428)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// GET should not be allowed for reindex
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17428/reindex")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	cancel()
	<-errCh
}

func TestDaemon_HandleReindex_ValidRequest(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17429)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Send valid reindex request
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:17429/reindex", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	cancel()
	<-errCh
}

func TestDaemon_HandleConfig_ReturnsConfig(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Daemon.Port = intPtr(17430)
	logger := daemonTestLogger()

	d, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Get config
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:17430/config")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify JSON response structure
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	_, hasConfig := result["config"]
	assert.True(t, hasConfig, "Response should have config key")

	cancel()
	<-errCh
}

// =============================================================================
// Unknown Model Dimension Handling Tests
// =============================================================================

func TestNew_UnknownModel_NoDimensions_FailsToStart(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "unknown-custom-model"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions configured
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)

	var daemonErr *DaemonError
	require.ErrorAs(t, err, &daemonErr)
	assert.Equal(t, "UNKNOWN_MODEL_DIMENSIONS", daemonErr.Code)
	assert.Contains(t, daemonErr.Message, "unknown-custom-model")
}

func TestNew_UnknownModel_WithDimensions_Starts(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "custom-embedding-model"
	cfg.Embedding.Ollama.Dimensions = 512
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_KnownModel_StartsWithoutDimensionsConfig(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "unclemusclez/jina-embeddings-v2-base-code"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions needed for known model
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_OllamaRemote_UnknownModel_RequiresDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama-remote"
	cfg.Embedding.Ollama.URL = "http://remote-server:11434"
	cfg.Embedding.Ollama.Model = "remote-custom-model"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)

	var daemonErr *DaemonError
	require.ErrorAs(t, err, &daemonErr)
	assert.Equal(t, "UNKNOWN_MODEL_DIMENSIONS", daemonErr.Code)
}

// =============================================================================
// Database Dimension Verification Tests
// =============================================================================

func TestNew_UnknownModel_DatabaseHasCorrectDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "custom-model-1024"
	cfg.Embedding.Ollama.Dimensions = 1024
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Assert - verify database was created with correct dimensions
	assert.Equal(t, 1024, daemon.db.Dimensions())

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_KnownModel_DatabaseHasRegistryDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "sellerscrisp/jina-embeddings-v4-text-code-q4"
	cfg.Embedding.Ollama.Dimensions = 768 // Wrong dimensions in config - should be ignored
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Assert - registry dimensions (1024) should be used, not config (768)
	assert.Equal(t, 1024, daemon.db.Dimensions())

	// Cleanup
	require.NoError(t, daemon.Close())
}
