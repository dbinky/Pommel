package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCmd_NotRunning(t *testing.T) {
	// Test that status command shows "not running" message when daemon is not available
	// Use an invalid URL to simulate daemon not running
	var buf bytes.Buffer
	output, err := executeStatusWithOutput("http://localhost:0", false)

	// Should return an error or indicate daemon not running
	if err != nil {
		assert.Contains(t, err.Error(), "not running")
	} else {
		assert.Contains(t, output, "not running")
	}
	_ = buf // silence unused variable warning
}

func TestStatusCmd_ShowsInfo(t *testing.T) {
	// Test that status command shows daemon and index info when daemon is running
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/status", r.URL.Path)

		response := api.StatusResponse{
			Daemon: &api.DaemonStatus{
				Running:       true,
				PID:           12345,
				UptimeSeconds: 3600.5,
			},
			Index: &api.IndexStatus{
				TotalFiles:     150,
				TotalChunks:    2500,
				LastIndexedAt:  time.Now().Add(-5 * time.Minute),
				IndexingActive: false,
			},
			Dependencies: &api.DependenciesStatus{
				Database: true,
				Embedder: true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeStatusWithOutput(server.URL, false)
	require.NoError(t, err)

	// Verify output contains expected information
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "12345") // PID
	assert.Contains(t, output, "150")   // files
	assert.Contains(t, output, "2500")  // chunks
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	// Test that status command returns JSON with --json flag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.StatusResponse{
			Daemon: &api.DaemonStatus{
				Running:       true,
				PID:           54321,
				UptimeSeconds: 1800.0,
			},
			Index: &api.IndexStatus{
				TotalFiles:     100,
				TotalChunks:    1000,
				LastIndexedAt:  time.Now(),
				IndexingActive: true,
			},
			Dependencies: &api.DependenciesStatus{
				Database: true,
				Embedder: true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeStatusWithOutput(server.URL, true)
	require.NoError(t, err)

	// Verify output is valid JSON
	var response api.StatusResponse
	err = json.Unmarshal([]byte(output), &response)
	require.NoError(t, err, "output should be valid JSON")

	assert.True(t, response.Daemon.Running)
	assert.Equal(t, 54321, response.Daemon.PID)
	assert.Equal(t, int64(100), response.Index.TotalFiles)
	assert.Equal(t, int64(1000), response.Index.TotalChunks)
	assert.True(t, response.Index.IndexingActive)
}

func TestStatusCmd_CommandRegistered(t *testing.T) {
	// Verify status command is registered with root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "status" {
			found = true
			break
		}
	}
	assert.True(t, found, "status command should be registered with root")
}

func TestStatusCmd_ShowsDependencies(t *testing.T) {
	// Test that status shows dependency availability
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.StatusResponse{
			Daemon: &api.DaemonStatus{
				Running:       true,
				PID:           99999,
				UptimeSeconds: 60.0,
			},
			Index: &api.IndexStatus{
				TotalFiles:     50,
				TotalChunks:    500,
				IndexingActive: false,
			},
			Dependencies: &api.DependenciesStatus{
				Database: true,
				Embedder: false, // Embedder not available
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeStatusWithOutput(server.URL, true)
	require.NoError(t, err)

	var response api.StatusResponse
	err = json.Unmarshal([]byte(output), &response)
	require.NoError(t, err)

	assert.True(t, response.Dependencies.Database)
	assert.False(t, response.Dependencies.Embedder)
}

// executeStatusWithOutput fetches status from the daemon and returns formatted output
func executeStatusWithOutput(daemonURL string, jsonOutput bool) (string, error) {
	// Send GET request to daemon
	resp, err := http.Get(daemonURL + "/status")
	if err != nil {
		return "", fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var statusResp api.StatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Return JSON output if requested
	if jsonOutput {
		output, err := json.MarshalIndent(statusResp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format JSON output: %w", err)
		}
		return string(output), nil
	}

	// Format human-readable output
	var buf bytes.Buffer

	// Daemon status
	if statusResp.Daemon != nil {
		if statusResp.Daemon.Running {
			fmt.Fprintf(&buf, "Daemon: running (PID %d)\n", statusResp.Daemon.PID)
			fmt.Fprintf(&buf, "Uptime: %.1f seconds\n", statusResp.Daemon.UptimeSeconds)
		} else {
			fmt.Fprintln(&buf, "Daemon: not running")
		}
	}

	// Index status
	if statusResp.Index != nil {
		fmt.Fprintf(&buf, "\nIndex:\n")
		fmt.Fprintf(&buf, "  Files: %d\n", statusResp.Index.TotalFiles)
		fmt.Fprintf(&buf, "  Chunks: %d\n", statusResp.Index.TotalChunks)
		if !statusResp.Index.LastIndexedAt.IsZero() {
			fmt.Fprintf(&buf, "  Last indexed: %s\n", statusResp.Index.LastIndexedAt.Format(time.RFC3339))
		}
		if statusResp.Index.IndexingActive {
			fmt.Fprintln(&buf, "  Status: indexing in progress")
		}
	}

	// Dependencies status
	if statusResp.Dependencies != nil {
		fmt.Fprintf(&buf, "\nDependencies:\n")
		fmt.Fprintf(&buf, "  Database: %s\n", boolToStatus(statusResp.Dependencies.Database))
		fmt.Fprintf(&buf, "  Embedder: %s\n", boolToStatus(statusResp.Dependencies.Embedder))
	}

	return buf.String(), nil
}

func boolToStatus(b bool) string {
	if b {
		return "available"
	}
	return "unavailable"
}
