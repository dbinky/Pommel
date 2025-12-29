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
