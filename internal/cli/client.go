package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/pommel-dev/pommel/internal/config"
)

// daemonSearchResult matches the daemon's actual response format
type daemonSearchResult struct {
	ChunkID   string  `json:"chunk_id"`
	FilePath  string  `json:"file_path"`
	Content   string  `json:"content"`
	Level     string  `json:"level"`
	Score     float64 `json:"score"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
}

// daemonSearchResponse matches the daemon's actual response format
type daemonSearchResponse struct {
	Results []daemonSearchResult `json:"results"`
	Query   string               `json:"query"`
	Limit   int                  `json:"limit"`
}

// Client provides methods to communicate with the pommeld daemon
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new daemon client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s", cfg.Daemon.Address()),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientFromProjectRoot creates a client by loading config from project root
func NewClientFromProjectRoot(projectRoot string) (*Client, error) {
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return NewClient(cfg), nil
}

// Health checks if the daemon is healthy
func (c *Client) Health() (*api.HealthResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	var health api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &health, nil
}

// Status retrieves the daemon and index status
func (c *Client) Status() (*api.StatusResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/status")
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status request failed: %s", string(body))
	}

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &status, nil
}

// Search performs a semantic search
func (c *Client) Search(req api.SearchRequest) (*api.SearchResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search request failed: %s", string(bodyBytes))
	}

	// Parse daemon's response format
	var daemonResp daemonSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&daemonResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to API response format
	results := make([]api.SearchResult, len(daemonResp.Results))
	for i, r := range daemonResp.Results {
		results[i] = api.SearchResult{
			ID:        r.ChunkID,
			File:      r.FilePath,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Level:     r.Level,
			Score:     float32(r.Score),
			Content:   r.Content,
		}
	}

	return &api.SearchResponse{
		Query:        daemonResp.Query,
		Results:      results,
		TotalResults: len(results),
		SearchTimeMs: 0, // Daemon doesn't provide this currently
	}, nil
}

// Reindex triggers a full reindex
func (c *Client) Reindex() (*api.ReindexResponse, error) {
	resp, err := c.httpClient.Post(c.baseURL+"/reindex", "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reindex request failed: %s", string(body))
	}

	var reindexResp api.ReindexResponse
	if err := json.NewDecoder(resp.Body).Decode(&reindexResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &reindexResp, nil
}

// Config retrieves the daemon configuration
func (c *Client) Config() (*config.Config, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/config")
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config request failed: %s", string(body))
	}

	var configResp struct {
		Config *config.Config `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return configResp.Config, nil
}
