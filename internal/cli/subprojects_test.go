package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSubprojectsCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "subprojects" {
			found = true
			break
		}
	}
	assert.True(t, found, "subprojects command should be registered with root")
}

func TestSubprojectsCmd_Aliases(t *testing.T) {
	flag := subprojectsCmd.Aliases
	assert.Contains(t, flag, "sp", "should have 'sp' alias")
}

// =============================================================================
// Output Tests
// =============================================================================

func TestSubprojectsCmd_ListsSubprojects(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		response := api.SubprojectsResponse{
			Subprojects: []api.SubprojectInfo{
				{ID: "backend", Path: "backend", MarkerFile: "go.mod", Language: "go"},
				{ID: "frontend", Path: "frontend", MarkerFile: "package.json", Language: "javascript"},
			},
			Total: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeSubprojectsWithOutput(server.URL, false)
	require.NoError(t, err)

	assert.Equal(t, "/subprojects", receivedPath)
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "frontend")
}

func TestSubprojectsCmd_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.SubprojectsResponse{
			Subprojects: []api.SubprojectInfo{
				{ID: "backend", Path: "backend", MarkerFile: "go.mod", Language: "go"},
			},
			Total: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeSubprojectsWithOutput(server.URL, true)
	require.NoError(t, err)

	// Verify valid JSON
	var resp api.SubprojectsResponse
	err = json.Unmarshal([]byte(output), &resp)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Subprojects, 1)
	assert.Equal(t, "backend", resp.Subprojects[0].ID)
}

func TestSubprojectsCmd_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := api.SubprojectsResponse{
			Subprojects: []api.SubprojectInfo{},
			Total:       0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	output, err := executeSubprojectsWithOutput(server.URL, false)
	require.NoError(t, err)

	assert.Contains(t, output, "No sub-projects", "Should show message for empty list")
}

// =============================================================================
// Helper Functions
// =============================================================================

func executeSubprojectsWithOutput(daemonURL string, jsonOutput bool) (string, error) {
	resp, err := http.Get(daemonURL + "/subprojects")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if jsonOutput {
		return string(body), nil
	}

	// Parse and format human-readable output
	var subResp api.SubprojectsResponse
	if err := json.Unmarshal(body, &subResp); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if len(subResp.Subprojects) == 0 {
		buf.WriteString("No sub-projects found\n")
	} else {
		for _, sp := range subResp.Subprojects {
			buf.WriteString(sp.ID + " (" + sp.Path + ")\n")
		}
	}

	return buf.String(), nil
}
