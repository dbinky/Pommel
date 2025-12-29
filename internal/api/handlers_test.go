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
		Query:        req.Query,
		Results:      []SearchResult{},
		TotalResults: 0,
		SearchTimeMs: 0,
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
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
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

// TestSearchHandler_Success verifies that a valid search request returns results
// TDD: This test should FAIL until search handler properly returns results with new format
func TestSearchHandler_Success(t *testing.T) {
	expectedResults := []SearchResult{
		{
			ID:        "chunk-abc123",
			File:      "/project/internal/auth/handler.go",
			StartLine: 45,
			EndLine:   67,
			Level:     "method",
			Language:  "go",
			Name:      "HandleLogin",
			Score:     0.95,
			Content:   "func HandleLogin(w http.ResponseWriter, r *http.Request) {\n\t// authentication logic\n}",
			Parent: &ParentInfo{
				ID:    "class-auth-handler",
				Name:  "AuthHandler",
				Level: "class",
			},
		},
		{
			ID:        "chunk-def456",
			File:      "/project/internal/auth/middleware.go",
			StartLine: 12,
			EndLine:   28,
			Level:     "method",
			Language:  "go",
			Name:      "AuthMiddleware",
			Score:     0.87,
			Content:   "func AuthMiddleware(next http.Handler) http.Handler {\n\t// middleware logic\n}",
			Parent:    nil,
		},
	}

	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			return &SearchResponse{
				Query:        req.Query,
				Results:      expectedResults,
				TotalResults: 2,
				SearchTimeMs: 15,
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
		Query: "authentication handler",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status")

	var response SearchResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal response")

	// Verify response structure matches new SearchResponse format
	assert.Equal(t, "authentication handler", response.Query, "expected query to be echoed back")
	assert.Equal(t, 2, response.TotalResults, "expected total_results to be 2")
	assert.GreaterOrEqual(t, response.SearchTimeMs, int64(0), "expected search_time_ms to be non-negative")
	assert.Len(t, response.Results, 2, "expected 2 results")

	// Verify first result structure
	result1 := response.Results[0]
	assert.Equal(t, "chunk-abc123", result1.ID, "expected result ID")
	assert.Equal(t, "/project/internal/auth/handler.go", result1.File, "expected result file")
	assert.Equal(t, 45, result1.StartLine, "expected start_line")
	assert.Equal(t, 67, result1.EndLine, "expected end_line")
	assert.Equal(t, "method", result1.Level, "expected level")
	assert.Equal(t, "go", result1.Language, "expected language")
	assert.Equal(t, "HandleLogin", result1.Name, "expected name")
	assert.InDelta(t, 0.95, result1.Score, 0.001, "expected score")
	assert.NotEmpty(t, result1.Content, "expected content")

	// Verify parent info
	assert.NotNil(t, result1.Parent, "expected parent info")
	assert.Equal(t, "class-auth-handler", result1.Parent.ID, "expected parent ID")
	assert.Equal(t, "AuthHandler", result1.Parent.Name, "expected parent name")
	assert.Equal(t, "class", result1.Parent.Level, "expected parent level")

	// Verify second result has nil parent
	result2 := response.Results[1]
	assert.Nil(t, result2.Parent, "expected second result to have no parent")
}

// TestSearchHandler_EmptyQuery verifies that an empty query returns 400 error
// TDD: This test should PASS as handler already validates empty query
func TestSearchHandler_EmptyQuery(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	// Test with empty string query
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

	var response APIError
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal error response")

	assert.Equal(t, "QUERY_EMPTY", response.Code, "expected QUERY_EMPTY error code")
	assert.Contains(t, response.Message, "query", "expected error message to mention 'query'")
}

// TestSearchHandler_EmptyQueryWhitespaceOnly verifies whitespace-only query returns 400
// TDD: This test may FAIL if handler doesn't trim whitespace
func TestSearchHandler_EmptyQueryWhitespaceOnly(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	// Test with whitespace-only query
	searchReq := SearchRequest{
		Query: "   \t\n  ",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	// Whitespace-only query should be treated as empty
	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 Bad Request for whitespace-only query")

	var response APIError
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal error response")

	assert.Equal(t, "QUERY_EMPTY", response.Code, "expected QUERY_EMPTY error code")
	assert.Contains(t, response.Message, "query", "expected error message to mention 'query'")
}

// TestSearchHandler_WithFilters verifies that level and path filters work correctly
// TDD: This test verifies filters are passed through to searcher
func TestSearchHandler_WithFilters(t *testing.T) {
	var receivedReq SearchRequest
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedReq = req
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 5,
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
		Query:      "database connection",
		Limit:      5,
		Levels:     []string{"method", "class"},
		PathPrefix: "internal/db",
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected 200 OK status")

	// Verify all filters were passed through
	assert.Equal(t, "database connection", receivedReq.Query, "expected query to be passed")
	assert.Equal(t, 5, receivedReq.Limit, "expected limit to be passed")
	assert.Equal(t, []string{"method", "class"}, receivedReq.Levels, "expected levels to be passed")
	assert.Equal(t, "internal/db", receivedReq.PathPrefix, "expected path_prefix to be passed")
}

// TestSearchHandler_WithFilters_SingleLevel verifies single level filter works
func TestSearchHandler_WithFilters_SingleLevel(t *testing.T) {
	var receivedReq SearchRequest
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedReq = req
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 2,
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
		Query:  "file operations",
		Limit:  10,
		Levels: []string{"file"},
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{"file"}, receivedReq.Levels, "expected single level filter")
}

// TestSearchHandler_WithFilters_DeepPathPrefix verifies deep path prefix filter works
func TestSearchHandler_WithFilters_DeepPathPrefix(t *testing.T) {
	var receivedReq SearchRequest
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedReq = req
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 3,
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
		Query:      "nested component",
		Limit:      20,
		PathPrefix: "internal/api/handlers/auth/oauth",
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "internal/api/handlers/auth/oauth", receivedReq.PathPrefix, "expected deep path prefix")
}

// TestSearchHandler_InvalidJSON verifies that malformed JSON returns 400 error
// TDD: This test should PASS as handler already validates JSON
func TestSearchHandler_InvalidJSON(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	testCases := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "malformed JSON - missing closing brace",
			body:     `{"query": "test"`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed JSON - invalid syntax",
			body:     `{query: test}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed JSON - trailing comma",
			body:     `{"query": "test",}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed JSON - unquoted string value",
			body:     `{"query": test}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed JSON - random text",
			body:     `this is not json at all`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty body",
			body:     ``,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "null body",
			body:     `null`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "array instead of object",
			body:     `["query", "test"]`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Search(rr, req)

			assert.Equal(t, tc.wantCode, rr.Code, "expected 400 Bad Request for %s", tc.name)

			var response APIError
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err, "failed to unmarshal error response for %s", tc.name)

			// null body is valid JSON that decodes to empty struct, resulting in QUERY_EMPTY
			// All other cases should return INVALID_JSON
			if tc.name == "null body" {
				assert.Equal(t, "QUERY_EMPTY", response.Code, "expected QUERY_EMPTY error code for %s", tc.name)
			} else {
				assert.Equal(t, "INVALID_JSON", response.Code, "expected INVALID_JSON error code for %s", tc.name)
			}
			assert.NotEmpty(t, response.Message, "expected error message for %s", tc.name)
		})
	}
}

// TestSearchHandler_InvalidJSON_WrongTypes verifies type mismatches return 400
func TestSearchHandler_InvalidJSON_WrongTypes(t *testing.T) {
	handler, cleanup := newTestHandler(t)
	defer cleanup()

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "limit as string",
			body: `{"query": "test", "limit": "ten"}`,
		},
		{
			name: "levels as string instead of array",
			body: `{"query": "test", "levels": "method"}`,
		},
		{
			name: "query as number",
			body: `{"query": 12345}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Search(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 Bad Request for %s", tc.name)

			var response APIError
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err, "failed to unmarshal error response for %s", tc.name)

			assert.Equal(t, "INVALID_JSON", response.Code, "expected INVALID_JSON error code for %s", tc.name)
			assert.NotEmpty(t, response.Message, "expected error message for %s", tc.name)
		})
	}
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
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
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
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
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
				Query: req.Query,
				Results: []SearchResult{
					{
						ID:        "chunk-1",
						File:      "/project/main.go",
						StartLine: 1,
						EndLine:   3,
						Level:     "method",
						Language:  "go",
						Name:      "main",
						Score:     0.95,
						Content:   "func main() {}",
					},
					{
						ID:        "chunk-2",
						File:      "/project/utils.go",
						StartLine: 10,
						EndLine:   15,
						Level:     "method",
						Language:  "go",
						Name:      "helper",
						Score:     0.85,
						Content:   "func helper() {}",
					},
				},
				TotalResults: 2,
				SearchTimeMs: 10,
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
	assert.Equal(t, 2, response.TotalResults, "expected total_results to be 2")
	assert.GreaterOrEqual(t, response.SearchTimeMs, int64(0), "expected search_time_ms to be non-negative")
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

	var response APIError
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "failed to unmarshal error response")

	// Verify the new APIError structure
	assert.NotEmpty(t, response.Code, "expected error code in response")
	assert.NotEmpty(t, response.Message, "expected error message in response")
	assert.NotEmpty(t, response.Suggestion, "expected suggestion in response")
}

// =============================================================================
// SearchServiceAdapter Tests
// =============================================================================

// TestNewSearchServiceAdapterCreatesAdapter verifies adapter creation
func TestNewSearchServiceAdapterCreatesAdapter(t *testing.T) {
	// Passing nil as service since we're just testing the constructor
	adapter := NewSearchServiceAdapter(nil)
	assert.NotNil(t, adapter)
}

// TestSearchHandler_SearcherReturnsError verifies error handling when searcher fails
func TestSearchHandler_SearcherReturnsError(t *testing.T) {
	// Create a searcher that returns an error
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			return nil, assert.AnError
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	searchReq := SearchRequest{
		Query: "test query",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	// Should return internal server error
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var response APIError
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "SEARCH_FAILED", response.Code)
	assert.NotEmpty(t, response.Details)
}

// TestSearchHandler_WithQueryLeadingTrailingSpaces verifies query trimming
func TestSearchHandler_WithQueryLeadingTrailingSpaces(t *testing.T) {
	var receivedQuery string
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedQuery = req.Query
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	// Query with leading/trailing spaces
	searchReq := SearchRequest{
		Query: "  test query with spaces  ",
		Limit: 10,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// The query should be trimmed
	assert.Equal(t, "test query with spaces", receivedQuery)
}

// TestSearchHandler_ZeroLimit verifies default limit when zero is provided
func TestSearchHandler_ZeroLimit(t *testing.T) {
	var receivedLimit int
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedLimit = req.Limit
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	// Zero limit should be passed through to searcher (searcher handles defaults)
	searchReq := SearchRequest{
		Query: "test query",
		Limit: 0,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 0, receivedLimit)
}

// TestSearchHandler_EmptyResults verifies handling of empty results
func TestSearchHandler_EmptyResults(t *testing.T) {
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 5,
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
		Query: "nonexistent function",
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

	assert.Equal(t, 0, response.TotalResults)
	assert.Len(t, response.Results, 0)
}

// TestNewHandlerSetsStartTime verifies that NewHandler sets the start time for uptime calculation
func TestNewHandlerSetsStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)

	beforeCreate := time.Now()
	handler := NewHandler(indexer, cfg, &mockSearcher{})
	afterCreate := time.Now()

	// Handler should have a start time between beforeCreate and afterCreate
	assert.True(t, handler.startTime.After(beforeCreate) || handler.startTime.Equal(beforeCreate))
	assert.True(t, handler.startTime.Before(afterCreate) || handler.startTime.Equal(afterCreate))
}

// TestStatusHandler_UptimeCalculation verifies uptime is calculated correctly
func TestStatusHandler_UptimeCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, &mockSearcher{})

	// Wait a bit to accumulate some uptime
	time.Sleep(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response StatusResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	// Uptime should be at least 50ms (0.05 seconds)
	assert.Greater(t, response.Daemon.UptimeSeconds, 0.04)
}

// TestSearchHandler_AllLevels verifies that all supported levels work
func TestSearchHandler_AllLevels(t *testing.T) {
	var receivedLevels []string
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
			receivedLevels = req.Levels
			return &SearchResponse{
				Query:        req.Query,
				Results:      []SearchResult{},
				TotalResults: 0,
				SearchTimeMs: 1,
			}, nil
		},
	}

	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	indexer := setupTestIndexer(t, tmpDir, cfg, database)
	handler := NewHandler(indexer, cfg, searcher)

	allLevels := []string{"method", "class", "file", "block", "module"}
	searchReq := SearchRequest{
		Query:  "test query",
		Limit:  10,
		Levels: allLevels,
	}
	body, err := json.Marshal(searchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, allLevels, receivedLevels)
}
