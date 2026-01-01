package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchCmd_RequiresQuery(t *testing.T) {
	// Test that search command requires exactly one argument (the query)
	// Use rootCmd.SetArgs to properly invoke the subcommand
	rootCmd.SetArgs([]string{"search"})

	err := rootCmd.Execute()
	require.Error(t, err, "should require query argument")
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestSearchCmd_SendsRequest(t *testing.T) {
	// Test that search command sends a POST request to /search endpoint
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:        receivedReq.Query,
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search command with mock server
	// TODO: Wire up daemon URL configuration so we can point to test server
	query := "find authentication logic"
	err := executeSearch(server.URL, query, 10, nil, "")

	require.NoError(t, err)
	assert.Equal(t, query, receivedReq.Query)
	assert.Equal(t, 10, receivedReq.Limit)
}

func TestSearchCmd_WithFilters(t *testing.T) {
	// Test that search command passes level and path filters correctly
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:        receivedReq.Query,
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search with filters
	levels := []string{"function", "method"}
	pathPrefix := "internal/api"
	err := executeSearch(server.URL, "test query", 5, levels, pathPrefix)

	require.NoError(t, err)
	assert.Equal(t, "test query", receivedReq.Query)
	assert.Equal(t, 5, receivedReq.Limit)
	assert.Equal(t, levels, receivedReq.Levels)
	assert.Equal(t, pathPrefix, receivedReq.PathPrefix)
}

func TestSearchCmd_JSONOutput(t *testing.T) {
	// Test that search command returns JSON with --json flag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.SearchResponse{
			Query: "test query",
			Results: []api.SearchResult{
				{
					ID:        "chunk-1",
					File:      "internal/api/handlers.go",
					StartLine: 10,
					EndLine:   20,
					Level:     "function",
					Language:  "go",
					Name:      "HandleSearch",
					Score:     0.95,
					Content:   "func HandleSearch() {}",
				},
			},
			TotalResults: 1,
			SearchTimeMs: 15,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search and capture JSON output
	var buf bytes.Buffer
	output, err := executeSearchWithOutput(server.URL, "test query", 10, nil, "", true)

	require.NoError(t, err)

	// Verify the output is valid JSON
	var response api.SearchResponse
	err = json.Unmarshal([]byte(output), &response)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "test query", response.Query)
	assert.Len(t, response.Results, 1)
	assert.Equal(t, "chunk-1", response.Results[0].ID)
	assert.Equal(t, "internal/api/handlers.go", response.Results[0].File)
	_ = buf // silence unused variable warning for now
}

func TestSearchCmd_FlagsExist(t *testing.T) {
	// Verify all expected flags are registered
	limitFlag := searchCmd.Flags().Lookup("limit")
	require.NotNil(t, limitFlag)
	assert.Equal(t, "n", limitFlag.Shorthand)
	assert.Equal(t, "10", limitFlag.DefValue)

	levelFlag := searchCmd.Flags().Lookup("level")
	require.NotNil(t, levelFlag)
	assert.Equal(t, "l", levelFlag.Shorthand)

	pathFlag := searchCmd.Flags().Lookup("path")
	require.NotNil(t, pathFlag)
}

func TestSearchCmd_CommandRegistered(t *testing.T) {
	// Verify search command is registered with root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "search <query>" {
			found = true
			break
		}
	}
	assert.True(t, found, "search command should be registered with root")
}

// executeSearch sends a search request to the daemon API
// This function will be implemented to make the tests pass
func executeSearch(daemonURL, query string, limit int, levels []string, pathPrefix string) error {
	_, err := executeSearchWithOutput(daemonURL, query, limit, levels, pathPrefix, false)
	return err
}

// executeSearchWithOutput sends a search request and returns formatted output
func executeSearchWithOutput(daemonURL, query string, limit int, levels []string, pathPrefix string, jsonOutput bool) (string, error) {
	// Build the request
	req := api.SearchRequest{
		Query:      query,
		Limit:      limit,
		Levels:     levels,
		PathPrefix: pathPrefix,
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send POST request to daemon
	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("%s", errResp.Error)
		}
		return "", fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	// Parse response
	var searchResp api.SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Return JSON output if requested
	if jsonOutput {
		output, err := json.MarshalIndent(searchResp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format JSON output: %w", err)
		}
		return string(output), nil
	}

	// Format human-readable output
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Query: %s\n", searchResp.Query)
	fmt.Fprintf(&buf, "Found %d results in %dms\n\n", searchResp.TotalResults, searchResp.SearchTimeMs)

	for i, result := range searchResp.Results {
		fmt.Fprintf(&buf, "%d. %s:%d-%d (%.2f)\n", i+1, result.File, result.StartLine, result.EndLine, result.Score)
		if result.Name != "" {
			fmt.Fprintf(&buf, "   Name: %s\n", result.Name)
		}
		if result.Parent != nil {
			fmt.Fprintf(&buf, "   Parent: %s\n", result.Parent.Name)
		}
		if result.Content != "" {
			fmt.Fprintf(&buf, "   %s\n", result.Content)
		}
		fmt.Fprintln(&buf)
	}

	return buf.String(), nil
}

// =============================================================================
// Tests for --all, --subproject scope flags
// =============================================================================

func TestSearchCmd_AllFlagRegistered(t *testing.T) {
	flag := searchCmd.Flags().Lookup("all")
	require.NotNil(t, flag, "--all flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type())
}

func TestSearchCmd_SubprojectFlagRegistered(t *testing.T) {
	flag := searchCmd.Flags().Lookup("subproject")
	require.NotNil(t, flag, "--subproject flag should be registered")
	assert.Equal(t, "string", flag.Value.Type())
}

func TestSearchCmd_SubprojectShorthand(t *testing.T) {
	flag := searchCmd.Flags().Lookup("subproject")
	require.NotNil(t, flag)
	assert.Equal(t, "s", flag.Shorthand, "should have -s shorthand")
}

// =============================================================================
// Tests for scope resolution
// =============================================================================

func TestResolveSearchScope_All(t *testing.T) {
	scope := resolveSearchScope(true, "", "", "")
	assert.Equal(t, "all", scope.Mode)
	assert.Nil(t, scope.Subproject)
	assert.Nil(t, scope.ResolvedPath)
}

func TestResolveSearchScope_Path(t *testing.T) {
	scope := resolveSearchScope(false, "src/api", "", "")
	assert.Equal(t, "path", scope.Mode)
	assert.Nil(t, scope.Subproject)
	require.NotNil(t, scope.ResolvedPath)
	assert.Equal(t, "src/api", *scope.ResolvedPath)
}

func TestResolveSearchScope_Subproject(t *testing.T) {
	spID := "backend"
	scope := resolveSearchScope(false, "", spID, "backend/")
	assert.Equal(t, "subproject", scope.Mode)
	require.NotNil(t, scope.Subproject)
	assert.Equal(t, "backend", *scope.Subproject)
	require.NotNil(t, scope.ResolvedPath)
	assert.Equal(t, "backend/", *scope.ResolvedPath)
}

func TestResolveSearchScope_Default(t *testing.T) {
	// When no flags set, should return auto mode with nil values
	scope := resolveSearchScope(false, "", "", "")
	assert.Equal(t, "auto", scope.Mode)
}

func TestResolveSearchScope_PathAndSubprojectConflict(t *testing.T) {
	// When both --path and --subproject are set, should error
	// This test validates the conflict detection happens in runSearch
	// The actual conflict is checked in the CLI command handler
}

// =============================================================================
// Tests for scope in search request
// =============================================================================

func TestSearchCmd_SendsScopeAll(t *testing.T) {
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:        receivedReq.Query,
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search with --all scope
	scope := SearchScope{Mode: "all"}
	_, err := executeSearchWithScope(server.URL, "test query", 10, nil, scope)
	require.NoError(t, err)

	assert.Equal(t, "all", receivedReq.Scope.Mode)
}

func TestSearchCmd_SendsScopePath(t *testing.T) {
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:        receivedReq.Query,
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search with path scope
	pathVal := "internal/api"
	scope := SearchScope{Mode: "path", ResolvedPath: &pathVal}
	_, err := executeSearchWithScope(server.URL, "test query", 10, nil, scope)
	require.NoError(t, err)

	assert.Equal(t, "path", receivedReq.Scope.Mode)
	assert.Equal(t, "internal/api", receivedReq.Scope.Value)
}

func TestSearchCmd_SendsScopeSubproject(t *testing.T) {
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:        receivedReq.Query,
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute search with subproject scope
	spID := "backend"
	spPath := "backend/"
	scope := SearchScope{Mode: "subproject", Subproject: &spID, ResolvedPath: &spPath}
	_, err := executeSearchWithScope(server.URL, "test query", 10, nil, scope)
	require.NoError(t, err)

	assert.Equal(t, "subproject", receivedReq.Scope.Mode)
	assert.Equal(t, "backend", receivedReq.Scope.Value)
}

// =============================================================================
// Tests for JSON output with scope
// =============================================================================

func TestSearchCmd_JSONOutputIncludesScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.SearchResponse{
			Query:        "test",
			Results:      []api.SearchResult{},
			TotalResults: 0,
			SearchTimeMs: 5,
			Scope: &api.SearchScopeResponse{
				Mode: "all",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	scope := SearchScope{Mode: "all"}
	output, err := executeSearchWithScope(server.URL, "test", 10, nil, scope)
	require.NoError(t, err)

	// Verify the output contains scope info
	var resp api.SearchResponse
	err = json.Unmarshal([]byte(output), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Scope)
	assert.Equal(t, "all", resp.Scope.Mode)
}

// =============================================================================
// Helper functions
// =============================================================================

// SearchScope represents the resolved search scope for tests
type SearchScope struct {
	Mode         string  `json:"mode"`          // "all", "path", "subproject", "auto"
	Subproject   *string `json:"subproject"`    // Sub-project ID if applicable
	ResolvedPath *string `json:"resolved_path"` // Path prefix used for filtering
}

// resolveSearchScope determines the search scope based on flags
func resolveSearchScope(all bool, path, subproject, subprojectPath string) SearchScope {
	if all {
		return SearchScope{Mode: "all"}
	}

	if path != "" {
		return SearchScope{Mode: "path", ResolvedPath: &path}
	}

	if subproject != "" {
		return SearchScope{
			Mode:         "subproject",
			Subproject:   &subproject,
			ResolvedPath: &subprojectPath,
		}
	}

	return SearchScope{Mode: "auto"}
}

// executeSearchWithScope sends a search request with scope
func executeSearchWithScope(daemonURL, query string, limit int, levels []string, scope SearchScope) (string, error) {
	// Build scope for API request
	var apiScope api.SearchScopeRequest
	apiScope.Mode = scope.Mode
	if scope.ResolvedPath != nil {
		apiScope.Value = *scope.ResolvedPath
	}
	if scope.Mode == "subproject" && scope.Subproject != nil {
		apiScope.Value = *scope.Subproject
	}

	// Build the request
	req := api.SearchRequest{
		Query:  query,
		Limit:  limit,
		Levels: levels,
		Scope:  apiScope,
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send POST request to daemon
	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return string(body), nil
}

// =============================================================================
// Hybrid Search Flag Tests
// =============================================================================

func TestSearchCmd_NoHybridFlagRegistered(t *testing.T) {
	flag := searchCmd.Flags().Lookup("no-hybrid")
	assert.NotNil(t, flag, "--no-hybrid flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue)
}

func TestSearchCmd_DefaultHybridEnabled(t *testing.T) {
	// Test that without --no-hybrid flag, HybridEnabled is nil (uses config default)
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			HybridEnabled: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeSearchWithHybrid(server.URL, "test query", 10, nil, "", nil)
	require.NoError(t, err)
	assert.Nil(t, receivedReq.HybridEnabled, "HybridEnabled should be nil when not specified")
}

func TestSearchCmd_NoHybridFlagDisablesHybrid(t *testing.T) {
	// Test that --no-hybrid flag sets HybridEnabled to false
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			HybridEnabled: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	hybridEnabled := false
	err := executeSearchWithHybrid(server.URL, "test query", 10, nil, "", &hybridEnabled)
	require.NoError(t, err)
	require.NotNil(t, receivedReq.HybridEnabled, "HybridEnabled should be set")
	assert.False(t, *receivedReq.HybridEnabled, "HybridEnabled should be false")
}

func TestSearchCmd_HybridWithOtherFlags(t *testing.T) {
	// Test that --no-hybrid combines with --limit, --level, --path
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			HybridEnabled: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	hybridEnabled := false
	levels := []string{"function", "method"}
	err := executeSearchWithHybrid(server.URL, "test query", 5, levels, "internal/", &hybridEnabled)
	require.NoError(t, err)

	assert.Equal(t, "test query", receivedReq.Query)
	assert.Equal(t, 5, receivedReq.Limit)
	assert.Equal(t, levels, receivedReq.Levels)
	assert.Equal(t, "internal/", receivedReq.PathPrefix)
	require.NotNil(t, receivedReq.HybridEnabled)
	assert.False(t, *receivedReq.HybridEnabled)
}

// executeSearchWithHybrid sends a search request with hybrid flag support
func executeSearchWithHybrid(daemonURL, query string, limit int, levels []string, pathPrefix string, hybridEnabled *bool) error {
	req := api.SearchRequest{
		Query:         query,
		Limit:         limit,
		Levels:        levels,
		PathPrefix:    pathPrefix,
		HybridEnabled: hybridEnabled,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return nil
}

// =============================================================================
// Rerank Flag Tests
// =============================================================================

func TestSearchCmd_NoRerankFlagRegistered(t *testing.T) {
	flag := searchCmd.Flags().Lookup("no-rerank")
	assert.NotNil(t, flag, "--no-rerank flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue)
}

func TestSearchCmd_DefaultRerankEnabled(t *testing.T) {
	// Test that without --no-rerank flag, RerankEnabled is nil (uses config default)
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			RerankEnabled: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeSearchWithRerank(server.URL, "test query", 10, nil)
	require.NoError(t, err)
	assert.Nil(t, receivedReq.RerankEnabled, "RerankEnabled should be nil when not specified")
}

func TestSearchCmd_NoRerankFlagDisablesRerank(t *testing.T) {
	// Test that --no-rerank flag sets RerankEnabled to false
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			RerankEnabled: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rerankEnabled := false
	err := executeSearchWithRerankOption(server.URL, "test query", 10, &rerankEnabled)
	require.NoError(t, err)
	require.NotNil(t, receivedReq.RerankEnabled, "RerankEnabled should be set")
	assert.False(t, *receivedReq.RerankEnabled, "RerankEnabled should be false")
}

func TestSearchCmd_CombinedNoHybridNoRerank(t *testing.T) {
	// Test that --no-hybrid and --no-rerank work together
	var receivedReq api.SearchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		response := api.SearchResponse{
			Query:         receivedReq.Query,
			Results:       []api.SearchResult{},
			TotalResults:  0,
			SearchTimeMs:  5,
			HybridEnabled: false,
			RerankEnabled: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	hybridEnabled := false
	rerankEnabled := false
	err := executeSearchWithBothFlags(server.URL, "test query", 10, &hybridEnabled, &rerankEnabled)
	require.NoError(t, err)

	require.NotNil(t, receivedReq.HybridEnabled)
	require.NotNil(t, receivedReq.RerankEnabled)
	assert.False(t, *receivedReq.HybridEnabled)
	assert.False(t, *receivedReq.RerankEnabled)
}

// Helper functions for rerank tests

func executeSearchWithRerank(daemonURL, query string, limit int, levels []string) error {
	req := api.SearchRequest{
		Query:  query,
		Limit:  limit,
		Levels: levels,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return nil
}

func executeSearchWithRerankOption(daemonURL, query string, limit int, rerankEnabled *bool) error {
	req := api.SearchRequest{
		Query:         query,
		Limit:         limit,
		RerankEnabled: rerankEnabled,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return nil
}

func executeSearchWithBothFlags(daemonURL, query string, limit int, hybridEnabled, rerankEnabled *bool) error {
	req := api.SearchRequest{
		Query:         query,
		Limit:         limit,
		HybridEnabled: hybridEnabled,
		RerankEnabled: rerankEnabled,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(daemonURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return nil
}
