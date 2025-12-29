package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers for Router Tests
// =============================================================================

// setupTestRouter creates a router with test dependencies
func setupTestRouter(t *testing.T) (*Router, func()) {
	tmpDir := t.TempDir()
	cfg := testRouterConfig()
	database := setupRouterTestDB(t, tmpDir)
	indexer := setupRouterTestIndexer(t, tmpDir, cfg, database)

	router := NewRouter(indexer, cfg, &mockRouterSearcher{})

	cleanup := func() {
		database.Close()
	}

	return router, cleanup
}

// testRouterConfig creates a default config for router testing
func testRouterConfig() *config.Config {
	return &config.Config{
		Version: 1,
		ChunkLevels: []string{
			"method",
			"class",
			"file",
		},
		IncludePatterns: []string{
			"**/*.go",
			"**/*.py",
			"**/*.js",
		},
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/.pommel/**",
		},
		Watcher: config.WatcherConfig{
			DebounceMs:  100,
			MaxFileSize: 1048576,
		},
		Daemon: config.DaemonConfig{
			Host:     "127.0.0.1",
			Port:     7331,
			LogLevel: "info",
		},
		Embedding: config.EmbeddingConfig{
			Model:     "mock-embedder",
			BatchSize: 32,
			CacheSize: 1000,
		},
		Search: config.SearchConfig{
			DefaultLimit:  10,
			DefaultLevels: []string{"method", "class", "file"},
		},
	}
}

// testRouterLogger creates a silent logger for router testing
func testRouterLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// setupRouterTestDB creates a temporary database for router testing
func setupRouterTestDB(t *testing.T, projectRoot string) *db.DB {
	database, err := db.Open(projectRoot)
	require.NoError(t, err)

	err = database.Migrate(context.Background())
	require.NoError(t, err)

	return database
}

// setupRouterTestIndexer creates an indexer for router testing
func setupRouterTestIndexer(t *testing.T, projectRoot string, cfg *config.Config, database *db.DB) *daemon.Indexer {
	emb := embedder.NewMockEmbedder()
	logger := testRouterLogger()

	indexer, err := daemon.NewIndexer(projectRoot, cfg, database, emb, logger)
	require.NoError(t, err)
	return indexer
}

// mockRouterSearcher implements the Searcher interface for router testing
type mockRouterSearcher struct{}

func (m *mockRouterSearcher) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	return &SearchResponse{
		Results: []SearchResult{},
		Query:   req.Query,
		Limit:   req.Limit,
	}, nil
}

// =============================================================================
// Route Registration Tests
// =============================================================================

// TestRouterRegistersHealthRoute verifies that /health route is registered
func TestRouterRegistersHealthRoute(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should not return 404 - route exists
	assert.NotEqual(t, http.StatusNotFound, rr.Code, "expected /health route to be registered")
	assert.Equal(t, http.StatusOK, rr.Code, "expected /health to return 200")
}

// TestRouterRegistersStatusRoute verifies that /status route is registered
func TestRouterRegistersStatusRoute(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code, "expected /status route to be registered")
	assert.Equal(t, http.StatusOK, rr.Code, "expected /status to return 200")
}

// TestRouterRegistersSearchRoute verifies that /search route is registered
func TestRouterRegistersSearchRoute(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	searchReq := SearchRequest{
		Query: "test query",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code, "expected /search route to be registered")
	assert.Equal(t, http.StatusOK, rr.Code, "expected /search to return 200 for valid request")
}

// TestRouterRegistersReindexRoute verifies that /reindex route is registered
func TestRouterRegistersReindexRoute(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code, "expected /reindex route to be registered")
	assert.Equal(t, http.StatusAccepted, rr.Code, "expected /reindex to return 202")
}

// TestRouterRegistersConfigRoute verifies that /config route is registered
func TestRouterRegistersConfigRoute(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code, "expected /config route to be registered")
	assert.Equal(t, http.StatusOK, rr.Code, "expected /config to return 200")
}

// =============================================================================
// HTTP Method Tests
// =============================================================================

// TestRouterHealthOnlyAcceptsGET verifies that /health only accepts GET
func TestRouterHealthOnlyAcceptsGET(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code,
				"expected %s to /health to return 405 Method Not Allowed", method)
		})
	}
}

// TestRouterStatusOnlyAcceptsGET verifies that /status only accepts GET
func TestRouterStatusOnlyAcceptsGET(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/status", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code,
				"expected %s to /status to return 405 Method Not Allowed", method)
		})
	}
}

// TestRouterSearchOnlyAcceptsPOST verifies that /search only accepts POST
func TestRouterSearchOnlyAcceptsPOST(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/search", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code,
				"expected %s to /search to return 405 Method Not Allowed", method)
		})
	}
}

// TestRouterReindexOnlyAcceptsPOST verifies that /reindex only accepts POST
func TestRouterReindexOnlyAcceptsPOST(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/reindex", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code,
				"expected %s to /reindex to return 405 Method Not Allowed", method)
		})
	}
}

// TestRouterConfigOnlyAcceptsGET verifies that /config only accepts GET
func TestRouterConfigOnlyAcceptsGET(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/config", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code,
				"expected %s to /config to return 405 Method Not Allowed", method)
		})
	}
}

// =============================================================================
// Middleware Tests
// =============================================================================

// TestRouterAppliesTimeoutMiddleware verifies that timeout middleware is applied
func TestRouterAppliesTimeoutMiddleware(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// The router should have timeout middleware configured
	// We can verify this by checking that requests don't hang forever
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	// This should complete within a reasonable time (not hang)
	done := make(chan bool)
	go func() {
		router.ServeHTTP(rr, req)
		done <- true
	}()

	select {
	case <-done:
		assert.Equal(t, http.StatusOK, rr.Code)
	case <-time.After(5 * time.Second):
		t.Fatal("request timed out - middleware may not be working correctly")
	}
}

// TestRouterAppliesContentTypeMiddleware verifies that responses have JSON content type
func TestRouterAppliesContentTypeMiddleware(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/health", nil},
		{http.MethodGet, "/status", nil},
		{http.MethodGet, "/config", nil},
	}

	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			var body *bytes.Buffer
			if ep.body != nil {
				body = bytes.NewBuffer(ep.body)
			} else {
				body = nil
			}

			var req *http.Request
			if body != nil {
				req = httptest.NewRequest(ep.method, ep.path, body)
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			contentType := rr.Header().Get("Content-Type")
			assert.Equal(t, "application/json", contentType,
				"expected %s to return application/json content type", ep.path)
		})
	}
}

// =============================================================================
// 404 Not Found Tests
// =============================================================================

// TestRouterReturns404ForUnknownRoutes verifies that unknown routes return 404
func TestRouterReturns404ForUnknownRoutes(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	unknownPaths := []string{
		"/unknown",
		"/api/v1/health",
		"/healthz",
		"/ready",
		"/metrics",
	}

	for _, path := range unknownPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code,
				"expected %s to return 404 Not Found", path)
		})
	}
}

// =============================================================================
// Integration-style Router Tests
// =============================================================================

// TestRouterFullHealthFlow verifies the complete health check flow through router
func TestRouterFullHealthFlow(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.False(t, response.Timestamp.IsZero())
}

// TestRouterFullSearchFlow verifies the complete search flow through router
func TestRouterFullSearchFlow(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	searchReq := SearchRequest{
		Query: "find functions",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response SearchResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "find functions", response.Query)
	assert.Equal(t, 10, response.Limit)
}

// TestRouterFullReindexFlow verifies the complete reindex flow through router
func TestRouterFullReindexFlow(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response ReindexResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "started", response.Status)
}

// TestRouterFullConfigFlow verifies the complete config flow through router
func TestRouterFullConfigFlow(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response ConfigResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Config)
	assert.Equal(t, 1, response.Config.Version)
}

// =============================================================================
// Router Handler Interface Tests
// =============================================================================

// TestRouterImplementsHTTPHandler verifies that Router implements http.Handler
func TestRouterImplementsHTTPHandler(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Verify Router can be used as http.Handler
	var _ http.Handler = router
}

// TestRouterCanBeUsedWithHTTPServer verifies router can be used with http.Server
func TestRouterCanBeUsedWithHTTPServer(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create a test server with the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Make a real HTTP request
	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var response HealthResponse
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
}
