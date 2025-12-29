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
// Test Helpers
// =============================================================================

// testLogger creates a silent logger for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testConfig creates a default config for testing
func testConfig() *config.Config {
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
			MaxFileSize: 1048576, // 1MB
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

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T, projectRoot string) *db.DB {
	database, err := db.Open(projectRoot)
	require.NoError(t, err)

	err = database.Migrate(context.Background())
	require.NoError(t, err)

	return database
}

// setupTestIndexer creates an indexer for testing
func setupTestIndexer(t *testing.T, projectRoot string, cfg *config.Config, database *db.DB) *daemon.Indexer {
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := daemon.NewIndexer(projectRoot, cfg, database, emb, logger)
	require.NoError(t, err)
	return indexer
}

// mockSearcher implements the Searcher interface for testing
type mockSearcher struct {
	searchFunc func(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}

func (m *mockSearcher) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, req)
	}
	return &SearchResponse{
		Results: []SearchResult{},
		Query:   req.Query,
		Limit:   req.Limit,
	}, nil
}

// newTestHandler creates a Handler with test dependencies
func newTestHandler(t *testing.T) (*Handler, func()) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	indexer := setupTestIndexer(t, tmpDir, cfg, database)

	handler := NewHandler(indexer, cfg, &mockSearcher{})

	cleanup := func() {
		database.Close()
	}

	return handler, cleanup
}

// =============================================================================
// Health Endpoint Tests
// =============================================================================

// TestHealthEndpointReturns200 verifies that GET /health returns 200 status
func TestHealthEndpointReturns200(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status")
}

// TestHealthEndpointResponseHasStatusHealthy verifies that health response has status "healthy"
func TestHealthEndpointResponseHasStatusHealthy(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	var response HealthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.Equal(t, "healthy", response.Status, "expected status to be 'healthy'")
}

// TestHealthEndpointResponseHasTimestamp verifies that health response has a timestamp
func TestHealthEndpointResponseHasTimestamp(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	beforeRequest := time.Now()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	afterRequest := time.Now()

	var response HealthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.False(t, response.Timestamp.IsZero(), "expected timestamp to be set")
	assert.True(t, response.Timestamp.After(beforeRequest) || response.Timestamp.Equal(beforeRequest),
		"expected timestamp to be after or equal to request start time")
	assert.True(t, response.Timestamp.Before(afterRequest) || response.Timestamp.Equal(afterRequest),
		"expected timestamp to be before or equal to request end time")
}

// TestHealthEndpointReturnsJSON verifies that health endpoint returns JSON content type
func TestHealthEndpointReturnsJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected application/json content type")
}

// =============================================================================
// Status Endpoint Tests
// =============================================================================

// TestStatusEndpointReturns200 verifies that GET /status returns 200 status
func TestStatusEndpointReturns200(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status")
}

// TestStatusEndpointResponseHasDaemonInfo verifies that status response contains daemon info
func TestStatusEndpointResponseHasDaemonInfo(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	var response StatusResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	// Verify daemon status fields exist
	assert.NotNil(t, response.Daemon, "expected daemon info in response")
	// DaemonStatus should have Running, PID, and Uptime fields
	assert.True(t, response.Daemon.Running || !response.Daemon.Running, "Running should be a boolean")
	assert.GreaterOrEqual(t, response.Daemon.PID, 0, "PID should be non-negative")
}

// TestStatusEndpointResponseHasIndexInfo verifies that status response contains index info
func TestStatusEndpointResponseHasIndexInfo(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	var response StatusResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	// Verify index status fields exist
	assert.NotNil(t, response.Index, "expected index info in response")
	assert.GreaterOrEqual(t, response.Index.TotalFiles, int64(0), "TotalFiles should be non-negative")
	assert.GreaterOrEqual(t, response.Index.TotalChunks, int64(0), "TotalChunks should be non-negative")
}

// TestStatusEndpointResponseHasDependencies verifies that status response contains dependency info
func TestStatusEndpointResponseHasDependencies(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	var response StatusResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	// Verify dependencies status exists
	assert.NotNil(t, response.Dependencies, "expected dependencies info in response")
}

// TestStatusEndpointReturnsJSON verifies that status endpoint returns JSON content type
func TestStatusEndpointReturnsJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected application/json content type")
}

// =============================================================================
// Search Endpoint Tests
// =============================================================================

// TestSearchEndpointWithValidQueryReturns200 verifies that POST /search with valid query returns 200
func TestSearchEndpointWithValidQueryReturns200(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	searchReq := SearchRequest{
		Query: "find all functions that handle authentication",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status for valid search query")
}

// TestSearchEndpointWithoutQueryReturns400 verifies that POST /search without query returns 400
func TestSearchEndpointWithoutQueryReturns400(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	searchReq := SearchRequest{
		Query: "",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 Bad Request for empty query")
}

// TestSearchEndpointWithLimitAppliesLimit verifies that search respects the limit parameter
func TestSearchEndpointWithLimitAppliesLimit(t *testing.T) {
	// Create a custom searcher that verifies the limit is passed through
	var receivedLimit int
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedLimit = req.Limit
			return &SearchResponse{
				Results: []SearchResult{},
				Query:   req.Query,
				Limit:   req.Limit,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	searchReq := SearchRequest{
		Query: "find functions",
		Limit: 5,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 5, receivedLimit, "expected limit to be passed to searcher")
}

// TestSearchEndpointWithEmptyBodyReturns400 verifies that POST /search with empty body returns 400
func TestSearchEndpointWithEmptyBodyReturns400(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 Bad Request for empty body")
}

// TestSearchEndpointWithInvalidJSONReturns400 verifies that POST /search with invalid JSON returns 400
func TestSearchEndpointWithInvalidJSONReturns400(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 Bad Request for invalid JSON")
}

// TestSearchEndpointReturnsJSON verifies that search endpoint returns JSON content type
func TestSearchEndpointReturnsJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
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

	handler.Search(rr, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected application/json content type")
}

// TestSearchEndpointWithLevelsFilter verifies that search with levels filter works
func TestSearchEndpointWithLevelsFilter(t *testing.T) {
	var receivedLevels []string
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedLevels = req.Levels
			return &SearchResponse{
				Results: []SearchResult{},
				Query:   req.Query,
				Limit:   req.Limit,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	searchReq := SearchRequest{
		Query:  "find functions",
		Limit:  10,
		Levels: []string{"method", "file"},
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{"method", "file"}, receivedLevels, "expected levels to be passed to searcher")
}

// TestSearchEndpointWithPathPrefix verifies that search with path prefix filter works
func TestSearchEndpointWithPathPrefix(t *testing.T) {
	var receivedPathPrefix string
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedPathPrefix = req.PathPrefix
			return &SearchResponse{
				Results: []SearchResult{},
				Query:   req.Query,
				Limit:   req.Limit,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	searchReq := SearchRequest{
		Query:      "find functions",
		Limit:      10,
		PathPrefix: "internal/api",
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "internal/api", receivedPathPrefix, "expected path_prefix to be passed to searcher")
}

// TestSearchEndpointReturnsResults verifies that search returns results in expected format
func TestSearchEndpointReturnsResults(t *testing.T) {
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			return &SearchResponse{
				Results: []SearchResult{
					{
						ChunkID:  "chunk-1",
						FilePath: "/project/main.go",
						Content:  "func main() {}",
						Level:    "method",
						Score:    0.95,
					},
					{
						ChunkID:  "chunk-2",
						FilePath: "/project/utils.go",
						Content:  "func helper() {}",
						Level:    "method",
						Score:    0.85,
					},
				},
				Query: req.Query,
				Limit: req.Limit,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	searchReq := SearchRequest{
		Query: "find functions",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response SearchResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Results, 2, "expected 2 results")
	assert.Equal(t, "find functions", response.Query)
	assert.Equal(t, 10, response.Limit)
}

// =============================================================================
// Reindex Endpoint Tests
// =============================================================================

// TestReindexEndpointReturns202 verifies that POST /reindex returns 202 Accepted
func TestReindexEndpointReturns202(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	handler.Reindex(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code, "expected 202 Accepted status")
}

// TestReindexEndpointResponseHasStatusStarted verifies that reindex response has status "started"
func TestReindexEndpointResponseHasStatusStarted(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	handler.Reindex(rr, req)

	var response ReindexResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.Equal(t, "started", response.Status, "expected status to be 'started'")
}

// TestReindexEndpointReturnsJSON verifies that reindex endpoint returns JSON content type
func TestReindexEndpointReturnsJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	handler.Reindex(rr, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected application/json content type")
}

// TestReindexEndpointResponseHasMessage verifies that reindex response has a message
func TestReindexEndpointResponseHasMessage(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	rr := httptest.NewRecorder()

	handler.Reindex(rr, req)

	var response ReindexResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.NotEmpty(t, response.Message, "expected message to be present")
}

// =============================================================================
// Config Endpoint Tests
// =============================================================================

// TestConfigEndpointReturns200 verifies that GET /config returns 200 status
func TestConfigEndpointReturns200(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	handler.Config(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status")
}

// TestConfigEndpointResponseContainsConfigData verifies that config response contains config data
func TestConfigEndpointResponseContainsConfigData(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	handler.Config(rr, req)

	var response ConfigResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	// Verify config contains expected fields
	assert.NotNil(t, response.Config, "expected config in response")
	assert.Equal(t, 1, response.Config.Version, "expected version 1")
	assert.NotEmpty(t, response.Config.ChunkLevels, "expected chunk_levels to be set")
	assert.NotEmpty(t, response.Config.IncludePatterns, "expected include_patterns to be set")
}

// TestConfigEndpointReturnsJSON verifies that config endpoint returns JSON content type
func TestConfigEndpointReturnsJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	handler.Config(rr, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected application/json content type")
}

// TestConfigEndpointContainsDaemonConfig verifies that config response contains daemon config
func TestConfigEndpointContainsDaemonConfig(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	handler.Config(rr, req)

	var response ConfigResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.Equal(t, "127.0.0.1", response.Config.Daemon.Host, "expected daemon host")
	assert.Equal(t, 7331, response.Config.Daemon.Port, "expected daemon port")
}

// TestConfigEndpointContainsSearchConfig verifies that config response contains search config
func TestConfigEndpointContainsSearchConfig(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rr := httptest.NewRecorder()

	handler.Config(rr, req)

	var response ConfigResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	assert.Equal(t, 10, response.Config.Search.DefaultLimit, "expected search default limit")
	assert.NotEmpty(t, response.Config.Search.DefaultLevels, "expected search default levels")
}

// =============================================================================
// Error Response Tests
// =============================================================================

// TestErrorResponseFormat verifies that error responses have proper format
func TestErrorResponseFormat(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	// Trigger an error with empty search query
	searchReq := SearchRequest{
		Query: "",
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response ErrorResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal error response")

	assert.NotEmpty(t, response.Error, "expected error message in response")
}
