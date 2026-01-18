package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OllamaError represents an error from Ollama operations with helpful context.
type OllamaError struct {
	Code       string
	Message    string
	Suggestion string
	Cause      error
}

// Error implements the error interface.
func (e *OllamaError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)
	if e.Suggestion != "" {
		sb.WriteString(". ")
		sb.WriteString(e.Suggestion)
	}
	return sb.String()
}

// Unwrap returns the underlying cause for errors.Is/As compatibility.
func (e *OllamaError) Unwrap() error {
	return e.Cause
}

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
	Model   string                 `json:"model"`
	Input   any                    `json:"input"`
	Options map[string]interface{} `json:"options,omitempty"`
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
		return &OllamaError{
			Code:       "OLLAMA_UNAVAILABLE",
			Message:    fmt.Sprintf("cannot connect to Ollama at %s", c.baseURL),
			Suggestion: "Is Ollama running? Start it with 'ollama serve' or check if it's listening on the configured port",
			Cause:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &OllamaError{
			Code:       "OLLAMA_HEALTH_FAILED",
			Message:    fmt.Sprintf("Ollama health check failed with status %d", resp.StatusCode),
			Suggestion: "Ollama may be starting up or experiencing issues. Check 'ollama logs' for details",
		}
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
		return nil, &OllamaError{
			Code:       "EMBEDDING_EMPTY",
			Message:    "Ollama returned no embeddings for the input",
			Suggestion: "This may indicate an issue with the model. Try restarting Ollama with 'ollama serve'",
		}
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
		Options: map[string]interface{}{
			"num_ctx": c.ContextSize(),
		},
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
		return nil, &OllamaError{
			Code:       "OLLAMA_CONNECTION_FAILED",
			Message:    fmt.Sprintf("cannot connect to Ollama at %s", c.baseURL),
			Suggestion: "Is Ollama running? Start it with 'ollama serve' or check if it's listening on the configured port (default: 11434)",
			Cause:      err,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error message from Ollama
		errMsg := strings.TrimSpace(string(body))

		// Check for common error cases
		if resp.StatusCode == http.StatusNotFound || strings.Contains(errMsg, "not found") {
			return nil, &OllamaError{
				Code:       "OLLAMA_MODEL_NOT_FOUND",
				Message:    fmt.Sprintf("embedding model '%s' not found in Ollama", c.model),
				Suggestion: fmt.Sprintf("Pull the model with 'ollama pull %s' or check your embedding.model config setting", c.model),
			}
		}

		return nil, &OllamaError{
			Code:       "OLLAMA_REQUEST_FAILED",
			Message:    fmt.Sprintf("Ollama embedding request failed with status %d", resp.StatusCode),
			Suggestion: "Check Ollama logs for details. The model may be loading or Ollama may be experiencing issues",
		}
	}

	var embedResp ollamaEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, &OllamaError{
			Code:       "OLLAMA_INVALID_RESPONSE",
			Message:    "received invalid response from Ollama",
			Suggestion: "This may indicate a version mismatch. Ensure Ollama is up to date",
			Cause:      err,
		}
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
	var errMu sync.Mutex
	var errOnce sync.Once
	var wg sync.WaitGroup

	// Helper to safely read firstErr
	getErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr
	}

	// Helper to safely set firstErr (only once)
	setErr := func(err error) {
		errOnce.Do(func() {
			errMu.Lock()
			firstErr = err
			errMu.Unlock()
		})
	}

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
				setErr(ctx.Err())
				return
			}

			// Check if we should proceed (no error yet)
			if getErr() != nil {
				return
			}

			embedding, err := c.EmbedSingle(ctx, t)
			if err != nil {
				setErr(err)
				return
			}

			results[idx] = embedding
		}(i, text)
	}

	wg.Wait()

	if err := getErr(); err != nil {
		return nil, err
	}

	return results, nil
}

// ModelName returns the configured model name.
func (c *OllamaClient) ModelName() string {
	return c.model
}

// Dimensions returns the embedding dimension size based on the configured model.
func (c *OllamaClient) Dimensions() int {
	return GetDimensionsForModel(c.model)
}

// ContextSize returns the context window size based on the configured model.
func (c *OllamaClient) ContextSize() int {
	return GetContextSizeForModel(c.model)
}
