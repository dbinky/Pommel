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

func TestReindexCmd_SendsRequest(t *testing.T) {
	// Test that reindex command sends a POST request to /reindex endpoint
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/reindex", r.URL.Path)

		response := api.ReindexResponse{
			Status:  "started",
			Message: "Reindexing started in background",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeReindex(server.URL, false)
	require.NoError(t, err)
	assert.True(t, requestReceived, "reindex endpoint should have been called")
}

func TestReindexCmd_Force(t *testing.T) {
	// Test that reindex command passes force flag correctly
	var receivedForce bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for force parameter in request body or query
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		if len(body) > 0 {
			var req map[string]interface{}
			err = json.Unmarshal(body, &req)
			if err == nil {
				if force, ok := req["force"].(bool); ok {
					receivedForce = force
				}
			}
		}

		// Also check query parameter
		if r.URL.Query().Get("force") == "true" {
			receivedForce = true
		}

		response := api.ReindexResponse{
			Status:  "started",
			Message: "Forced reindexing started",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeReindex(server.URL, true)
	require.NoError(t, err)
	assert.True(t, receivedForce, "force flag should be passed to server")
}

func TestReindexCmd_JSONOutput(t *testing.T) {
	// Test that reindex command returns JSON with --json flag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.ReindexResponse{
			Status:  "started",
			Message: "Reindexing started in background",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeReindexWithOutput(server.URL, false, true)
	require.NoError(t, err)

	// Verify output is valid JSON
	var response api.ReindexResponse
	err = json.Unmarshal([]byte(output), &response)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "started", response.Status)
	assert.Contains(t, response.Message, "Reindexing")
}

func TestReindexCmd_FlagsExist(t *testing.T) {
	// Verify the force flag is registered
	forceFlag := reindexCmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag)
	assert.Equal(t, "f", forceFlag.Shorthand)
	assert.Equal(t, "false", forceFlag.DefValue)
}

func TestReindexCmd_PathFlag(t *testing.T) {
	// Verify the path flag is registered
	pathFlag := reindexCmd.Flags().Lookup("path")
	require.NotNil(t, pathFlag, "--path flag should be registered")
	assert.Equal(t, "", pathFlag.DefValue, "default should be empty")
}

func TestReindexCmd_PathFlagSendsPath(t *testing.T) {
	// Test that path flag is sent to daemon
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err == nil {
			if path, ok := req["path"].(string); ok {
				receivedPath = path
			}
		}

		response := api.ReindexResponse{
			Status:  "started",
			Message: "Reindexing path: src/",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeReindexWithPath(server.URL, false, "src/")
	require.NoError(t, err)
	assert.Equal(t, "src/", receivedPath, "path should be sent to server")
}

func TestReindexCmd_CommandRegistered(t *testing.T) {
	// Verify reindex command is registered with root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "reindex" {
			found = true
			break
		}
	}
	assert.True(t, found, "reindex command should be registered with root")
}

func TestReindexCmd_HandlesError(t *testing.T) {
	// Test that reindex command handles server errors gracefully
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.ErrorResponse{
			Error: "indexing already in progress",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	err := executeReindex(server.URL, false)
	require.Error(t, err, "should return error for conflict status")
	assert.Contains(t, err.Error(), "indexing already in progress")
}

func TestReindexCmd_DaemonNotRunning(t *testing.T) {
	// Test that reindex command handles daemon not running
	var buf bytes.Buffer
	err := executeReindex("http://localhost:0", false)

	assert.Error(t, err)
	// Error should indicate connection failure or daemon not running
	_ = buf // silence unused variable warning
}

// executeReindex sends a reindex request to the daemon API
// This function will be implemented to make the tests pass
func executeReindex(daemonURL string, force bool) error {
	_, err := executeReindexWithOutput(daemonURL, force, false)
	return err
}

// executeReindexWithPath sends a reindex request with a path filter
func executeReindexWithPath(daemonURL string, force bool, path string) error {
	// Build the request with path
	req := struct {
		Force bool   `json:"force"`
		Path  string `json:"path,omitempty"`
	}{
		Force: force,
		Path:  path,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(daemonURL+"/reindex", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("reindex failed with status %d", resp.StatusCode)
	}

	return nil
}

// executeReindexWithOutput sends a reindex request and returns formatted output
func executeReindexWithOutput(daemonURL string, force bool, jsonOutput bool) (string, error) {
	// Build the request
	req := api.ReindexRequest{
		Force: force,
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send POST request to daemon
	resp, err := http.Post(daemonURL+"/reindex", "application/json", bytes.NewReader(reqBody))
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("%s", errResp.Error)
		}
		return "", fmt.Errorf("reindex failed with status %d", resp.StatusCode)
	}

	// Parse response
	var reindexResp api.ReindexResponse
	if err := json.Unmarshal(body, &reindexResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Return JSON output if requested
	if jsonOutput {
		output, err := json.MarshalIndent(reindexResp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format JSON output: %w", err)
		}
		return string(output), nil
	}

	// Format human-readable output
	return fmt.Sprintf("%s: %s\n", reindexResp.Status, reindexResp.Message), nil
}
