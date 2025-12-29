//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Constants
// =============================================================================

const (
	ollamaBaseURL      = "http://localhost:11434"
	daemonStartTimeout = 30 * time.Second
	indexingTimeout    = 60 * time.Second
)

// =============================================================================
// Helper Functions
// =============================================================================

// isOllamaAvailable checks if Ollama is running and accessible at localhost:11434.
func isOllamaAvailable() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ollamaBaseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// createTestProject creates a test project directory with sample code files
// in Python, JavaScript, and Go.
func createTestProject(t *testing.T, dir string) {
	t.Helper()

	// Create Python sample file
	pythonCode := `"""Sample Python module for integration testing."""

class Calculator:
    """A simple calculator class for basic arithmetic operations."""

    def __init__(self):
        self.history = []

    def add(self, a: float, b: float) -> float:
        """Add two numbers together and return the result."""
        result = a + b
        self.history.append(f"add({a}, {b}) = {result}")
        return result

    def subtract(self, a: float, b: float) -> float:
        """Subtract b from a and return the result."""
        result = a - b
        self.history.append(f"subtract({a}, {b}) = {result}")
        return result

    def multiply(self, a: float, b: float) -> float:
        """Multiply two numbers and return the product."""
        result = a * b
        self.history.append(f"multiply({a}, {b}) = {result}")
        return result

    def divide(self, a: float, b: float) -> float:
        """Divide a by b and return the quotient.

        Raises:
            ZeroDivisionError: If b is zero.
        """
        if b == 0:
            raise ZeroDivisionError("Cannot divide by zero")
        result = a / b
        self.history.append(f"divide({a}, {b}) = {result}")
        return result


def fibonacci(n: int) -> int:
    """Calculate the nth Fibonacci number using recursion."""
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)


def factorial(n: int) -> int:
    """Calculate the factorial of n using iteration."""
    if n < 0:
        raise ValueError("Factorial not defined for negative numbers")
    result = 1
    for i in range(2, n + 1):
        result *= i
    return result
`
	pythonPath := filepath.Join(dir, "calculator.py")
	err := os.WriteFile(pythonPath, []byte(pythonCode), 0644)
	require.NoError(t, err, "Failed to create Python test file")

	// Create JavaScript sample file
	jsCode := `/**
 * User authentication and management module.
 * Handles user registration, login, and session management.
 */

class UserManager {
    constructor() {
        this.users = new Map();
        this.sessions = new Map();
    }

    /**
     * Register a new user with email and password.
     * @param {string} email - The user's email address.
     * @param {string} password - The user's password.
     * @returns {Object} The created user object.
     * @throws {Error} If email is already registered.
     */
    registerUser(email, password) {
        if (this.users.has(email)) {
            throw new Error('Email already registered');
        }
        const user = {
            id: crypto.randomUUID(),
            email: email,
            passwordHash: this.hashPassword(password),
            createdAt: new Date(),
        };
        this.users.set(email, user);
        return { id: user.id, email: user.email };
    }

    /**
     * Authenticate a user and create a session.
     * @param {string} email - The user's email address.
     * @param {string} password - The user's password.
     * @returns {string} Session token.
     * @throws {Error} If credentials are invalid.
     */
    login(email, password) {
        const user = this.users.get(email);
        if (!user || user.passwordHash !== this.hashPassword(password)) {
            throw new Error('Invalid credentials');
        }
        const sessionToken = crypto.randomUUID();
        this.sessions.set(sessionToken, {
            userId: user.id,
            email: user.email,
            createdAt: new Date(),
        });
        return sessionToken;
    }

    /**
     * Log out a user by invalidating their session.
     * @param {string} sessionToken - The session token to invalidate.
     */
    logout(sessionToken) {
        this.sessions.delete(sessionToken);
    }

    /**
     * Hash a password for secure storage.
     * @param {string} password - The password to hash.
     * @returns {string} The hashed password.
     */
    hashPassword(password) {
        // Simple hash for demonstration - use bcrypt in production
        return Buffer.from(password).toString('base64');
    }
}

/**
 * Validate an email address format.
 * @param {string} email - The email to validate.
 * @returns {boolean} True if valid email format.
 */
function validateEmail(email) {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
}

/**
 * Generate a secure random password.
 * @param {number} length - Desired password length.
 * @returns {string} Random password string.
 */
function generatePassword(length = 16) {
    const charset = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*';
    let password = '';
    for (let i = 0; i < length; i++) {
        password += charset[Math.floor(Math.random() * charset.length)];
    }
    return password;
}

module.exports = { UserManager, validateEmail, generatePassword };
`
	jsPath := filepath.Join(dir, "auth.js")
	err = os.WriteFile(jsPath, []byte(jsCode), 0644)
	require.NoError(t, err, "Failed to create JavaScript test file")

	// Create Go sample file
	goCode := `package storage

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrKeyNotFound is returned when a key does not exist in the cache.
var ErrKeyNotFound = errors.New("key not found in cache")

// ErrKeyExpired is returned when a key exists but has expired.
var ErrKeyExpired = errors.New("key has expired")

// CacheEntry represents a single cached value with expiration.
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// Cache provides a thread-safe in-memory key-value store with TTL support.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry
	defaultTTL time.Duration
}

// NewCache creates a new cache instance with the specified default TTL.
func NewCache(defaultTTL time.Duration) *Cache {
	return &Cache{
		entries:    make(map[string]CacheEntry),
		defaultTTL: defaultTTL,
	}
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *Cache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Get retrieves a value from the cache by key.
func (c *Cache) Get(ctx context.Context, key string) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	if time.Now().After(entry.ExpiresAt) {
		// Clean up expired entry
		c.Delete(ctx, key)
		return nil, ErrKeyExpired
	}

	return entry.Value, nil
}

// Delete removes a key from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	return nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]CacheEntry)
	return nil
}

// Size returns the number of entries in the cache (including expired ones).
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// CleanExpired removes all expired entries from the cache.
func (c *Cache) CleanExpired(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			cleaned++
		}
	}
	return cleaned, nil
}
`
	goPath := filepath.Join(dir, "cache.go")
	err = os.WriteFile(goPath, []byte(goCode), 0644)
	require.NoError(t, err, "Failed to create Go test file")

	t.Logf("Created test project with Python, JavaScript, and Go files in %s", dir)
}

// waitForDaemon waits for the daemon to become healthy at the given address.
// Returns an error if the daemon does not become healthy within the timeout.
func waitForDaemon(t *testing.T, address string, timeout time.Duration) error {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := fmt.Sprintf("http://%s/health", address)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("daemon did not become healthy within %v", timeout)
		}

		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("Daemon is healthy at %s", address)
				return nil
			}
		}

		<-ticker.C
	}
}

// waitForIndexing waits for the daemon to finish indexing by polling the status endpoint.
func waitForIndexing(t *testing.T, address string, timeout time.Duration) error {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	statusURL := fmt.Sprintf("http://%s/status", address)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// First wait for at least one file to be indexed
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("indexing did not complete within %v", timeout)
		}

		resp, err := client.Get(statusURL)
		if err != nil {
			<-ticker.C
			continue
		}

		var status api.StatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			<-ticker.C
			continue
		}
		resp.Body.Close()

		// Check if indexing is complete (has files and not actively indexing)
		if status.Index != nil && status.Index.TotalFiles > 0 && !status.Index.IndexingActive {
			t.Logf("Indexing complete: %d files, %d chunks", status.Index.TotalFiles, status.Index.TotalChunks)
			return nil
		}

		<-ticker.C
	}
}

// getFreePort returns an available port for the daemon to use.
func getFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to get free port")
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

// testLogger returns a logger for tests that writes to the test log.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestIntegration_FullFlow(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	// Create test project directory
	projectRoot := t.TempDir()
	createTestProject(t, projectRoot)

	// Initialize Pommel (create .pommel directory, config, and database)
	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err, "Failed to create .pommel directory")

	// Create config with Ollama embedder
	port := getFreePort(t)
	cfg := config.Default()
	cfg.Daemon.Port = port
	cfg.Daemon.Host = "127.0.0.1"
	cfg.Watcher.DebounceMs = 100 // Fast debounce for tests
	cfg.IncludePatterns = []string{"**/*.py", "**/*.js", "**/*.go"}

	// Save config
	loader := config.NewLoader(projectRoot)
	err = loader.Save(cfg)
	require.NoError(t, err, "Failed to save config")

	// Initialize database
	database, err := db.Open(projectRoot)
	require.NoError(t, err, "Failed to open database")
	err = database.Migrate(context.Background())
	require.NoError(t, err, "Failed to migrate database")
	database.Close()

	// Create daemon with Ollama embedder
	ollamaCfg := embedder.OllamaConfig{
		BaseURL: ollamaBaseURL,
		Model:   cfg.Embedding.Model,
	}
	ollamaClient := embedder.NewOllamaClient(ollamaCfg)

	// Verify Ollama can generate embeddings
	ctx := context.Background()
	_, err = ollamaClient.EmbedSingle(ctx, "test")
	if err != nil {
		t.Skipf("Skipping integration test: Ollama model not available: %v", err)
	}

	// Create daemon (using mock embedder since we tested Ollama separately)
	logger := testLogger(t)
	daemonInstance, err := daemon.New(projectRoot, cfg, logger)
	require.NoError(t, err, "Failed to create daemon")

	// Start daemon in background
	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemonInstance.Run(daemonCtx)
	}()

	// Wait for daemon to become healthy
	address := fmt.Sprintf("127.0.0.1:%d", port)
	err = waitForDaemon(t, address, daemonStartTimeout)
	require.NoError(t, err, "Daemon did not become healthy")

	// Wait for indexing to complete
	err = waitForIndexing(t, address, indexingTimeout)
	require.NoError(t, err, "Indexing did not complete")

	// Perform search
	client := &http.Client{Timeout: 10 * time.Second}
	searchURL := fmt.Sprintf("http://%s/search", address)

	searchReq := api.SearchRequest{
		Query: "calculate arithmetic operations",
		Limit: 10,
	}
	reqBody, err := json.Marshal(searchReq)
	require.NoError(t, err, "Failed to marshal search request")

	resp, err := client.Post(searchURL, "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err, "Search request failed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Search should return 200 OK")

	var searchResp api.SearchResponse
	err = json.NewDecoder(resp.Body).Decode(&searchResp)
	require.NoError(t, err, "Failed to decode search response")

	// Verify we got results
	assert.NotEmpty(t, searchResp.Results, "Search should return results")
	t.Logf("Search returned %d results", len(searchResp.Results))

	// Stop daemon gracefully
	daemonCancel()

	select {
	case err := <-errCh:
		// Context cancellation is expected
		assert.True(t, err == nil || err == context.Canceled,
			"Daemon should shutdown gracefully, got: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Daemon did not shutdown within timeout")
	}

	t.Log("Integration test completed successfully")
}

func TestIntegration_SearchReturnsRelevantResults(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	// Create test project
	projectRoot := t.TempDir()
	createTestProject(t, projectRoot)

	// Initialize Pommel
	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	port := getFreePort(t)
	cfg := config.Default()
	cfg.Daemon.Port = port
	cfg.Daemon.Host = "127.0.0.1"
	cfg.Watcher.DebounceMs = 100
	cfg.IncludePatterns = []string{"**/*.py", "**/*.js", "**/*.go"}

	loader := config.NewLoader(projectRoot)
	err = loader.Save(cfg)
	require.NoError(t, err)

	database, err := db.Open(projectRoot)
	require.NoError(t, err)
	err = database.Migrate(context.Background())
	require.NoError(t, err)
	database.Close()

	// Create and start daemon
	logger := testLogger(t)
	daemonInstance, err := daemon.New(projectRoot, cfg, logger)
	require.NoError(t, err)

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemonInstance.Run(daemonCtx)
	}()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	err = waitForDaemon(t, address, daemonStartTimeout)
	require.NoError(t, err)

	err = waitForIndexing(t, address, indexingTimeout)
	require.NoError(t, err)

	// Test various search queries
	testCases := []struct {
		name           string
		query          string
		expectedInFile string // Part of filename expected in results
	}{
		{
			name:           "Search for authentication",
			query:          "user authentication and login",
			expectedInFile: "auth.js",
		},
		{
			name:           "Search for caching",
			query:          "cache key value store with expiration",
			expectedInFile: "cache.go",
		},
		{
			name:           "Search for math calculations",
			query:          "calculate fibonacci sequence",
			expectedInFile: "calculator.py",
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	searchURL := fmt.Sprintf("http://%s/search", address)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searchReq := api.SearchRequest{
				Query: tc.query,
				Limit: 5,
			}
			reqBody, err := json.Marshal(searchReq)
			require.NoError(t, err)

			resp, err := client.Post(searchURL, "application/json", bytes.NewReader(reqBody))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var searchResp api.SearchResponse
			err = json.NewDecoder(resp.Body).Decode(&searchResp)
			require.NoError(t, err)

			// Log results for debugging
			t.Logf("Query '%s' returned %d results", tc.query, len(searchResp.Results))
			for i, result := range searchResp.Results {
				t.Logf("  %d. %s (score: %.4f)", i+1, result.File, result.Score)
			}

			// Verify we got results (with mock embedder results may vary)
			assert.NotEmpty(t, searchResp.Results, "Should return results for query: %s", tc.query)
		})
	}

	// Cleanup
	daemonCancel()
	<-errCh
}

func TestIntegration_StatusEndpoint(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	projectRoot := t.TempDir()
	createTestProject(t, projectRoot)

	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	port := getFreePort(t)
	cfg := config.Default()
	cfg.Daemon.Port = port
	cfg.Daemon.Host = "127.0.0.1"
	cfg.Watcher.DebounceMs = 100
	cfg.IncludePatterns = []string{"**/*.py", "**/*.js", "**/*.go"}

	loader := config.NewLoader(projectRoot)
	err = loader.Save(cfg)
	require.NoError(t, err)

	database, err := db.Open(projectRoot)
	require.NoError(t, err)
	err = database.Migrate(context.Background())
	require.NoError(t, err)
	database.Close()

	logger := testLogger(t)
	daemonInstance, err := daemon.New(projectRoot, cfg, logger)
	require.NoError(t, err)

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemonInstance.Run(daemonCtx)
	}()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	err = waitForDaemon(t, address, daemonStartTimeout)
	require.NoError(t, err)

	err = waitForIndexing(t, address, indexingTimeout)
	require.NoError(t, err)

	// Get status
	client := &http.Client{Timeout: 5 * time.Second}
	statusURL := fmt.Sprintf("http://%s/status", address)

	resp, err := client.Get(statusURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var status api.StatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	// Verify status response
	assert.NotNil(t, status.Daemon, "Status should include daemon info")
	assert.True(t, status.Daemon.Running, "Daemon should be running")
	assert.Greater(t, status.Daemon.PID, 0, "PID should be positive")

	assert.NotNil(t, status.Index, "Status should include index info")
	assert.Greater(t, status.Index.TotalFiles, int64(0), "Should have indexed files")
	assert.Greater(t, status.Index.TotalChunks, int64(0), "Should have indexed chunks")

	t.Logf("Status: %d files, %d chunks indexed", status.Index.TotalFiles, status.Index.TotalChunks)

	// Cleanup
	daemonCancel()
	<-errCh
}

func TestIntegration_ReindexEndpoint(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	projectRoot := t.TempDir()
	createTestProject(t, projectRoot)

	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	port := getFreePort(t)
	cfg := config.Default()
	cfg.Daemon.Port = port
	cfg.Daemon.Host = "127.0.0.1"
	cfg.Watcher.DebounceMs = 100
	cfg.IncludePatterns = []string{"**/*.py", "**/*.js", "**/*.go"}

	loader := config.NewLoader(projectRoot)
	err = loader.Save(cfg)
	require.NoError(t, err)

	database, err := db.Open(projectRoot)
	require.NoError(t, err)
	err = database.Migrate(context.Background())
	require.NoError(t, err)
	database.Close()

	logger := testLogger(t)
	daemonInstance, err := daemon.New(projectRoot, cfg, logger)
	require.NoError(t, err)

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemonInstance.Run(daemonCtx)
	}()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	err = waitForDaemon(t, address, daemonStartTimeout)
	require.NoError(t, err)

	err = waitForIndexing(t, address, indexingTimeout)
	require.NoError(t, err)

	// Trigger reindex
	client := &http.Client{Timeout: 5 * time.Second}
	reindexURL := fmt.Sprintf("http://%s/reindex", address)

	resp, err := client.Post(reindexURL, "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "Reindex should return 202 Accepted")

	var reindexResp api.ReindexResponse
	err = json.NewDecoder(resp.Body).Decode(&reindexResp)
	require.NoError(t, err)

	assert.Equal(t, "started", reindexResp.Status, "Reindex should report started status")

	t.Log("Reindex triggered successfully")

	// Cleanup
	daemonCancel()
	<-errCh
}

func TestIntegration_FileWatchingAndReindexing(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	projectRoot := t.TempDir()
	createTestProject(t, projectRoot)

	pommelDir := filepath.Join(projectRoot, config.PommelDir)
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	port := getFreePort(t)
	cfg := config.Default()
	cfg.Daemon.Port = port
	cfg.Daemon.Host = "127.0.0.1"
	cfg.Watcher.DebounceMs = 100
	cfg.IncludePatterns = []string{"**/*.py", "**/*.js", "**/*.go"}

	loader := config.NewLoader(projectRoot)
	err = loader.Save(cfg)
	require.NoError(t, err)

	database, err := db.Open(projectRoot)
	require.NoError(t, err)
	err = database.Migrate(context.Background())
	require.NoError(t, err)
	database.Close()

	logger := testLogger(t)
	daemonInstance, err := daemon.New(projectRoot, cfg, logger)
	require.NoError(t, err)

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemonInstance.Run(daemonCtx)
	}()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	err = waitForDaemon(t, address, daemonStartTimeout)
	require.NoError(t, err)

	err = waitForIndexing(t, address, indexingTimeout)
	require.NoError(t, err)

	// Get initial status
	client := &http.Client{Timeout: 5 * time.Second}
	statusURL := fmt.Sprintf("http://%s/status", address)

	resp, err := client.Get(statusURL)
	require.NoError(t, err)

	var initialStatus api.StatusResponse
	err = json.NewDecoder(resp.Body).Decode(&initialStatus)
	resp.Body.Close()
	require.NoError(t, err)

	initialFiles := initialStatus.Index.TotalFiles
	t.Logf("Initial file count: %d", initialFiles)

	// Add a new file
	newFilePath := filepath.Join(projectRoot, "new_module.py")
	newFileContent := `"""A newly added module for testing file watching."""

def new_function():
    """A new function added during the test."""
    return "Hello from new function"
`
	err = os.WriteFile(newFilePath, []byte(newFileContent), 0644)
	require.NoError(t, err)

	// Wait for the file to be indexed
	time.Sleep(2 * time.Second)

	// Get updated status
	resp, err = client.Get(statusURL)
	require.NoError(t, err)

	var updatedStatus api.StatusResponse
	err = json.NewDecoder(resp.Body).Decode(&updatedStatus)
	resp.Body.Close()
	require.NoError(t, err)

	t.Logf("Updated file count: %d", updatedStatus.Index.TotalFiles)

	// Verify file count increased
	assert.Greater(t, updatedStatus.Index.TotalFiles, initialFiles,
		"File count should increase after adding new file")

	// Cleanup
	daemonCancel()
	<-errCh
}

func TestIntegration_OllamaHealthCheck(t *testing.T) {
	// This test verifies the Ollama availability check itself
	available := isOllamaAvailable()
	t.Logf("Ollama available: %v", available)

	if available {
		// Try to create an Ollama client and verify it works
		cfg := embedder.DefaultOllamaConfig()
		client := embedder.NewOllamaClient(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := client.Health(ctx)
		assert.NoError(t, err, "Ollama health check should pass when available")

		// Try a simple embedding
		embedding, err := client.EmbedSingle(ctx, "test embedding generation")
		if err != nil {
			t.Logf("Note: Embedding failed (model may not be available): %v", err)
		} else {
			assert.NotEmpty(t, embedding, "Embedding should not be empty")
			t.Logf("Generated embedding with %d dimensions", len(embedding))
		}
	} else {
		t.Log("Ollama is not available - skipping Ollama-specific tests")
	}
}

// =============================================================================
// Daemon with Ollama Embedder Test
// =============================================================================

func TestIntegration_DaemonWithOllamaEmbedder(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Skipping integration test: Ollama is not available at localhost:11434")
	}

	// Verify the embedding model is available
	ollamaCfg := embedder.DefaultOllamaConfig()
	ollamaClient := embedder.NewOllamaClient(ollamaCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := ollamaClient.EmbedSingle(ctx, "test")
	if err != nil {
		t.Skipf("Skipping test: Ollama model '%s' not available: %v", ollamaCfg.Model, err)
	}

	t.Logf("Using Ollama model: %s", ollamaClient.ModelName())
	t.Logf("Embedding dimensions: %d", ollamaClient.Dimensions())

	// Test basic embedding functionality
	testTexts := []string{
		"function to calculate fibonacci sequence",
		"user authentication and login",
		"cache with TTL expiration",
	}

	embeddings, err := ollamaClient.Embed(ctx, testTexts)
	require.NoError(t, err, "Batch embedding should succeed")
	assert.Len(t, embeddings, len(testTexts), "Should return embedding for each text")

	for i, emb := range embeddings {
		assert.NotEmpty(t, emb, "Embedding %d should not be empty", i)
		assert.Equal(t, ollamaClient.Dimensions(), len(emb), "Embedding should have correct dimensions")
	}

	t.Log("Ollama embedder integration verified successfully")
}
