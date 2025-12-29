package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// OllamaClient provides embedding generation via Ollama's local API.
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// OllamaConfig holds configuration options for the Ollama client.
type OllamaConfig struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// ollamaEmbedRequest represents the request to Ollama's /api/embed endpoint.
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// ollamaEmbedResponse represents the response from Ollama's /api/embed endpoint.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// DefaultOllamaConfig returns the default configuration for Ollama.
func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   "unclemusclez/jina-embeddings-v2-base-code",
		Timeout: 30 * time.Second,
	}
}

// NewOllamaClient creates a new Ollama client with the given configuration.
func NewOllamaClient(cfg OllamaConfig) *OllamaClient {
	// Apply defaults for empty values
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultOllamaConfig().BaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultOllamaConfig().Model
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultOllamaConfig().Timeout
	}

	return &OllamaClient{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Health checks if Ollama is running and accessible.
func (c *OllamaClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// EmbedSingle generates an embedding for a single text input.
func (c *OllamaClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.embed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// Embed generates embeddings for multiple texts in a single request.
func (c *OllamaClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	return c.embed(ctx, texts)
}

// embed is the internal method that handles the actual embedding request.
func (c *OllamaClient) embed(ctx context.Context, input any) ([][]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model: c.model,
		Input: input,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding request failed with status %d: model not found: %s", resp.StatusCode, string(body))
	}

	var embedResp ollamaEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embedResp.Embeddings, nil
}

// EmbedBatch generates embeddings for multiple texts with concurrency control.
func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string, concurrency int) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if concurrency <= 0 {
		concurrency = 1
	}

	// Results slice to preserve order
	results := make([][]float32, len(texts))
	var firstErr error
	var errOnce sync.Once
	var wg sync.WaitGroup

	// Semaphore for concurrency control
	sem := make(chan struct{}, concurrency)

	for i, text := range texts {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errOnce.Do(func() {
					firstErr = ctx.Err()
				})
				return
			}

			// Check if we should proceed (no error yet)
			if firstErr != nil {
				return
			}

			embedding, err := c.EmbedSingle(ctx, t)
			if err != nil {
				errOnce.Do(func() {
					firstErr = err
				})
				return
			}

			results[idx] = embedding
		}(i, text)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// ModelName returns the configured model name.
func (c *OllamaClient) ModelName() string {
	return c.model
}

// Dimensions returns the embedding dimension size (768 for Jina Code).
func (c *OllamaClient) Dimensions() int {
	return 768
}
